package httpclient

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/itportal/it-portal-agent/internal/security"
)

type Client struct { HTTP *http.Client; Token string; Retries int; RetryDelay time.Duration }

func New(timeout, retryDelay time.Duration, token string) *Client {
	return &Client{Token: token, Retries: 3, RetryDelay: retryDelay, HTTP: &http.Client{Timeout: timeout, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, DisableCompression: false}}}
}

func (c *Client) Do(request *http.Request) (*http.Response, error) {
	if err := security.SignRequest(request, c.Token, time.Now()); err != nil { return nil, err }
	var lastErr error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		response, err := c.HTTP.Do(request)
		if err == nil { return response, nil }
		lastErr = err
		if attempt < c.Retries { time.Sleep(c.RetryDelay) }
	}
	return nil, fmt.Errorf("request failed after %d retries: %w", c.Retries, lastErr)
}
