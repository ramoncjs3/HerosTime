package httputil

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"oldbeggar-refactor/internal/config"
)

func testClient() *Client {
	return New(config.HTTPConfig{
		Timeout: config.Duration{Duration: 5 * time.Second},
		Retries: 3,
		Backoff: config.Duration{Duration: time.Millisecond},
	})
}

// TestPostNotRetried 钉住评审结论：POST 等非幂等请求遇到 5xx/网络错误不自动重试。
func TestPostNotRetried(t *testing.T) {
	var hits int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	if _, err := testClient().Post(context.Background(), server.URL, "text/plain", []byte("x"), nil); err == nil {
		t.Fatal("expected error from 500 response")
	}
	if got := atomic.LoadInt64(&hits); got != 1 {
		t.Fatalf("POST was sent %d times, want 1 (POST must not be retried)", got)
	}
}

// TestGetRetried GET 是幂等请求，5xx 后仍按配置重试。
func TestGetRetried(t *testing.T) {
	var hits int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt64(&hits, 1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	resp, err := testClient().Get(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(resp.Body) != "ok" {
		t.Fatalf("body = %q, want ok", resp.Body)
	}
	if got := atomic.LoadInt64(&hits); got != 3 {
		t.Fatalf("GET was sent %d times, want 3", got)
	}
}
