package software

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/itportal/it-portal-agent/internal/storage"
	syncengine "github.com/itportal/it-portal-agent/internal/sync"
	"go.uber.org/zap"
)

const (
	Type            = "software"
	Endpoint        = "/api/agent/software"
	DefaultInterval = 24 * time.Hour
)

type Engine struct {
	DB      *sql.DB
	Sync    *syncengine.Engine
	Log     *zap.Logger
	Version string
}

func New(db *sql.DB, transport *syncengine.Engine, log *zap.Logger, version string) *Engine {
	return &Engine{DB: db, Sync: transport, Log: log, Version: version}
}

// Run deliberately waits for the first configured interval. Software is not a
// heartbeat task and must not add a heavy scan to bootstrap.
func (e *Engine) Run(ctx context.Context) {
	timer := time.NewTimer(e.interval())
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		_, _ = e.RunOnce(ctx)
		timer.Reset(e.interval())
	}
}

func (e *Engine) RunOnce(ctx context.Context) (Report, error) {
	if err := ctx.Err(); err != nil {
		return Report{Result: "FAILED"}, err
	}
	if _, err := e.Sync.Flush(ctx); err != nil {
		e.Log.Warn("software queue flush failed", zap.Error(err))
	}
	agentUUID, found, err := storage.Get(e.DB, "agent_uuid")
	if err != nil || !found || agentUUID == "" {
		if err == nil {
			err = syncengine.ErrNotRegistered
		}
		return Report{Result: "FAILED"}, err
	}
	collectStarted := time.Now()
	items, err := Collect(ctx)
	if err != nil {
		return Report{CollectDuration: time.Since(collectStarted), Result: "FAILED"}, err
	}
	items = NormalizeAndDeduplicate(items)
	collectDuration := time.Since(collectStarted)
	payload := Payload{AgentUUID: agentUUID, CollectedAt: time.Now().UTC(), Items: items}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return Report{CollectDuration: collectDuration, Result: "FAILED"}, err
	}
	fingerprint := payloadFingerprint(payload)
	previous, _, _ := storage.Get(e.DB, "software_last_fingerprint")
	if previous == fingerprint {
		return Report{CollectDuration: collectDuration, SoftwareCount: len(items), SkippedItems: len(items), Fingerprint: fingerprint, Result: "SKIPPED"}, nil
	}
	uploadStarted := time.Now()
	result, sendErr := e.Sync.SendJSON(ctx, Endpoint, Type, payload)
	report := Report{CollectDuration: collectDuration, UploadDuration: time.Since(uploadStarted), PayloadSize: len(encoded), SoftwareCount: len(items), ChangedItems: len(items), Fingerprint: fingerprint}
	if sendErr != nil {
		if errors.Is(sendErr, syncengine.ErrQueued) {
			report.Result = "QUEUED"
			_ = storage.Put(e.DB, "software_last_fingerprint", fingerprint)
			e.Log.Warn("software inventory queued offline", zap.Int("items", len(items)))
		} else {
			report.Result = "FAILED"
		}
		return report, sendErr
	}
	_ = storage.Put(e.DB, "software_last_fingerprint", fingerprint)
	_ = storage.Put(e.DB, "software_last_success", time.Now().UTC().Format(time.RFC3339Nano))
	e.Log.Info("software inventory sent", zap.Int("items", len(items)), zap.Int64("latency_ms", result.Latency.Milliseconds()))
	report.Result = "SENT"
	return report, nil
}

func payloadFingerprint(payload Payload) string {
	data, _ := json.Marshal(struct {
		AgentUUID string         `json:"agentUuid"`
		Items     []Installation `json:"items"`
	}{payload.AgentUUID, payload.Items})
	return fmt.Sprintf("%x", sha256Sum(data))
}

func sha256Sum(data []byte) [32]byte { return sha256.Sum256(data) }

func (e *Engine) interval() time.Duration {
	if value, found, err := storage.Get(e.DB, "portal_configuration"); err == nil && found {
		var raw map[string]any
		if json.Unmarshal([]byte(value), &raw) == nil {
			for _, key := range []string{"softwareInventoryInterval", "software_inventory_interval", "softwareInterval", "software_interval"} {
				if duration := parseInterval(raw[key]); duration > 0 {
					return duration
				}
			}
		}
	}
	return DefaultInterval
}

func parseInterval(value any) time.Duration {
	switch typed := value.(type) {
	case string:
		if duration, err := time.ParseDuration(typed); err == nil && duration > 0 {
			return duration
		}
	case float64:
		if typed > 0 {
			return time.Duration(typed) * time.Second
		}
	case int:
		if typed > 0 {
			return time.Duration(typed) * time.Second
		}
	}
	return 0
}
