package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// SignRequest applies the Sprint 6.6-compatible HMAC SHA-256 request headers.
// Canonical form: timestamp + newline + nonce + newline + method + newline + path + newline + SHA256(body).
func SignRequest(request *http.Request, token string, now time.Time) error {
	if request.Body == nil {
		request.Body = http.NoBody
	}
	body, err := io.ReadAll(request.Body)
	if err != nil { return fmt.Errorf("read request body: %w", err) }
	if err := request.Body.Close(); err != nil { return fmt.Errorf("close request body: %w", err) }
	request.Body = io.NopCloser(bytesReader(body))
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil { return fmt.Errorf("generate nonce: %w", err) }
	timestamp := strconv.FormatInt(now.UTC().Unix(), 10)
	nonce := hex.EncodeToString(nonceBytes)
	bodyHash := sha256.Sum256(body)
	canonical := timestamp + "\n" + nonce + "\n" + request.Method + "\n" + request.URL.EscapedPath() + "\n" + hex.EncodeToString(bodyHash[:])
	mac := hmac.New(sha256.New, []byte(token)); mac.Write([]byte(canonical))
	request.Header.Set("X-ITPortal-Timestamp", timestamp)
	request.Header.Set("X-ITPortal-Nonce", nonce)
	request.Header.Set("X-ITPortal-Signature", hex.EncodeToString(mac.Sum(nil)))
	return nil
}

type byteReader struct { data []byte; offset int }
func bytesReader(data []byte) *byteReader { return &byteReader{data: data} }
func (r *byteReader) Read(p []byte) (int, error) { if r.offset >= len(r.data) { return 0, io.EOF }; n := copy(p, r.data[r.offset:]); r.offset += n; return n, nil }
