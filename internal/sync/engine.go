// Package sync provides the shared authenticated transport for all Portal payloads.
// Heartbeat is the first consumer; inventory, software, commands, policy, and uploads
// must use this package when those later sprints are implemented.
package sync

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/itportal/it-portal-agent/internal/httpclient"
	"github.com/itportal/it-portal-agent/internal/storage"
	"go.uber.org/zap"
)

const (
	HeartbeatType = "heartbeat"
	InventoryType = "inventory"
	SoftwareType  = "software"
)

var (
	ErrQueued          = errors.New("payload queued for offline delivery")
	ErrNotRegistered   = errors.New("agent is not registered")
	ErrInvalidResponse = errors.New("invalid Portal response")
)

type Options struct {
	PortalURL   string
	Proxy       string
	TLSValidate bool
	Compression string
	Timeout     time.Duration
	RetryDelay  time.Duration
}

type Result struct {
	StatusCode int
	Body       []byte
	Latency    time.Duration
}

type Engine struct {
	DB          *sql.DB
	Client      *httpclient.Client
	PortalURL   string
	Log         *zap.Logger
	Compression string
	mu          sync.Mutex
}

func New(db *sql.DB, options Options, log *zap.Logger) *Engine {
	return &Engine{
		DB:          db,
		Client:      httpclient.NewWithOptions(httpclient.Options{Timeout: options.Timeout, RetryDelay: options.RetryDelay, TLSValidation: options.TLSValidate, Proxy: options.Proxy}),
		PortalURL:   strings.TrimRight(options.PortalURL, "/"),
		Log:         log,
		Compression: strings.ToLower(options.Compression),
	}
}

func (e *Engine) SendJSON(ctx context.Context, endpoint, payloadType string, payload any) (Result, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return Result{}, fmt.Errorf("marshal %s payload: %w", payloadType, err)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	result, err := e.send(ctx, endpoint, payloadType, data, true)
	return result, err
}

// Flush retries persisted payloads in FIFO order. A failed item remains in SQLite
// and stops the pass, preserving order and avoiding a retry storm while offline.
func (e *Engine) Flush(ctx context.Context) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	items, err := storage.Pending(e.DB, 100)
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, item := range items {
		endpoint := routeForType(item.Type)
		if endpoint == "" {
			continue
		}
		if _, err := e.send(ctx, endpoint, item.Type, item.Payload, false); err != nil {
			return sent, err
		}
		if err := storage.DeleteQueueItem(e.DB, item.ID); err != nil {
			return sent, err
		}
		sent++
	}
	return sent, nil
}

func (e *Engine) send(ctx context.Context, endpoint, payloadType string, payload []byte, queueOnFailure bool) (Result, error) {
	token, found, err := storage.GetSecret(e.DB, "agent_token")
	if err != nil {
		return Result{}, err
	}
	if !found || token == "" {
		return Result{}, ErrNotRegistered
	}
	// The shared HTTP client signs each request with the DPAPI-protected token.
	e.Client.Token = token
	body, encoding, err := encodePayload(payload, e.Compression)
	if err != nil {
		return Result{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, e.PortalURL+endpoint, bytes.NewReader(body))
	if err != nil {
		return Result{}, fmt.Errorf("create sync request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	if encoding != "" {
		request.Header.Set("Content-Encoding", encoding)
	}
	started := time.Now()
	response, err := e.Client.Do(request)
	latency := time.Since(started)
	if err != nil {
		return e.queueOnFailure(ctx, payloadType, payload, queueOnFailure, fmt.Errorf("sync request: %w", err))
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(response.Body)
	if readErr != nil {
		return e.queueOnFailure(ctx, payloadType, payload, queueOnFailure, fmt.Errorf("read sync response: %w", readErr))
	}
	result := Result{StatusCode: response.StatusCode, Body: responseBody, Latency: latency}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return e.queueOnFailure(ctx, payloadType, payload, queueOnFailure, fmt.Errorf("Portal returned %s: %s", response.Status, truncate(responseBody)))
	}
	// A successful Portal endpoint may return either an envelope or an empty
	// acknowledgement. Non-empty responses must still be valid JSON.
	if len(bytes.TrimSpace(responseBody)) > 0 && !json.Valid(responseBody) {
		return e.queueOnFailure(ctx, payloadType, payload, queueOnFailure, ErrInvalidResponse)
	}
	return result, nil
}

func (e *Engine) queueOnFailure(ctx context.Context, payloadType string, payload []byte, queueOnFailure bool, cause error) (Result, error) {
	if !queueOnFailure || ctx.Err() != nil {
		return Result{}, cause
	}
	if err := storage.Enqueue(e.DB, payloadType, payload); err != nil {
		return Result{}, fmt.Errorf("%w; queue payload: %v", cause, err)
	}
	return Result{}, fmt.Errorf("%w: %v", ErrQueued, cause)
}

func encodePayload(payload []byte, compression string) ([]byte, string, error) {
	if compression != "gzip" {
		return payload, "", nil
	}
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, err := writer.Write(payload); err != nil {
		return nil, "", fmt.Errorf("compress payload: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("finish compression: %w", err)
	}
	return buffer.Bytes(), "gzip", nil
}

func routeForType(payloadType string) string {
	switch payloadType {
	case HeartbeatType:
		return "/api/agent/heartbeat"
	case InventoryType:
		return "/api/agent/inventory"
	case SoftwareType:
		return "/api/agent/software"
	default:
		return ""
	}
}

func truncate(value []byte) string {
	const maximum = 512
	if len(value) <= maximum {
		return strings.TrimSpace(string(value))
	}
	return strings.TrimSpace(string(value[:maximum])) + "..."
}
