package httputil

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"oldbeggar-refactor/internal/config"
)

type Client struct {
	httpClient *http.Client
	retries    int
	backoff    time.Duration
}

type Response struct {
	StatusCode int
	Body       []byte
	Header     http.Header
	Cookies    []*http.Cookie
}

func New(cfg config.HTTPConfig) *Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.InsecureTLS {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &Client{
		httpClient: &http.Client{
			Timeout:   cfg.Timeout.Duration,
			Transport: transport,
		},
		retries: cfg.Retries,
		backoff: cfg.Backoff.Duration,
	}
}

func (c *Client) Get(ctx context.Context, rawURL string) (*Response, error) {
	return c.Do(ctx, http.MethodGet, rawURL, "", nil, nil)
}

func (c *Client) Post(ctx context.Context, rawURL, contentType string, body []byte, headers map[string]string) (*Response, error) {
	return c.Do(ctx, http.MethodPost, rawURL, contentType, body, headers)
}

func (c *Client) PostForm(ctx context.Context, rawURL string, values url.Values, headers map[string]string) (*Response, error) {
	return c.Post(ctx, rawURL, "application/x-www-form-urlencoded", []byte(values.Encode()), headers)
}

func (c *Client) Do(ctx context.Context, method, rawURL, contentType string, body []byte, headers map[string]string) (*Response, error) {
	rawURL = normalizeURL(rawURL)
	attempts := c.retries + 1
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, rawURL, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		req.Header.Set("User-Agent", "Dalvik/2.1.0 (Linux; U; Android 10.0.1; Galaxy S20+ Build/V417IR)")
		for key, value := range headers {
			req.Header.Set(key, value)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if !c.shouldRetry(ctx, attempt, attempts, 0) {
				break
			}
			continue
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		result := &Response{
			StatusCode: resp.StatusCode,
			Body:       respBody,
			Header:     resp.Header.Clone(),
			Cookies:    resp.Cookies(),
		}

		if readErr != nil {
			lastErr = readErr
			if !c.shouldRetry(ctx, attempt, attempts, resp.StatusCode) {
				break
			}
			continue
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return result, nil
		}
		lastErr = fmt.Errorf("%s %s returned status %d: %s", method, rawURL, resp.StatusCode, trimBody(respBody))
		if !c.shouldRetry(ctx, attempt, attempts, resp.StatusCode) {
			return result, lastErr
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("%s %s failed", method, rawURL)
	}
	return nil, lastErr
}

func (c *Client) shouldRetry(ctx context.Context, attempt, attempts, status int) bool {
	if attempt >= attempts {
		return false
	}
	if status != 0 && status != http.StatusTooManyRequests && status < 500 {
		return false
	}
	timer := time.NewTimer(c.backoff * time.Duration(attempt))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func normalizeURL(rawURL string) string {
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		return rawURL
	}
	return "http://" + rawURL
}

func trimBody(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 300 {
		return text[:300]
	}
	return text
}
