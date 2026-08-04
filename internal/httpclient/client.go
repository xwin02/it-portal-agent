package httpclient

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/itportal/it-portal-agent/internal/security"
)

type Options struct {
	Timeout       time.Duration
	RetryDelay    time.Duration
	Token         string
	TLSValidation bool
	Proxy         string
}

type Client struct {
	HTTP       *http.Client
	Token      string
	Retries    int
	RetryDelay time.Duration
}

func New(timeout, retryDelay time.Duration, token string) *Client {
	return NewWithOptions(Options{Timeout: timeout, RetryDelay: retryDelay, Token: token, TLSValidation: true})
}

func NewWithOptions(options Options) *Client {
	if options.Timeout <= 0 {
		options.Timeout = 30 * time.Second
	}
	if options.RetryDelay <= 0 {
		options.RetryDelay = time.Second
	}
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: !options.TLSValidation}, DisableCompression: false}
	if options.Proxy != "" {
		if proxy, err := url.Parse(options.Proxy); err == nil {
			transport.Proxy = http.ProxyURL(proxy)
		}
	}
	return &Client{Token: options.Token, Retries: 3, RetryDelay: options.RetryDelay, HTTP: &http.Client{Timeout: options.Timeout, Transport: transport}}
}

func (c *Client) Do(request *http.Request) (*http.Response, error) {
	var body []byte
	if request.Body != nil {
		var err error
		body, err = io.ReadAll(request.Body)
		if err != nil {
			return nil, fmt.Errorf("read request body: %w", err)
		}
		if err := request.Body.Close(); err != nil {
			return nil, fmt.Errorf("close request body: %w", err)
		}
	}
	var lastErr error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		request.Body = io.NopCloser(bytes.NewReader(body))
		if c.Token != "" {
			if err := security.SignRequest(request, c.Token, time.Now()); err != nil {
				return nil, err
			}
		}
		response, err := c.HTTP.Do(request)
		if err == nil {
			return response, nil
		}
		lastErr = err
		if attempt < c.Retries {
			time.Sleep(c.RetryDelay)
		}
	}
	return nil, fmt.Errorf("request failed after %d retries: %w", c.Retries, lastErr)
}
