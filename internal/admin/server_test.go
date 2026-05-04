package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"oldbeggar-refactor/internal/config"
)

func TestCSRFTokenValidatesSession(t *testing.T) {
	server := testServer()
	loginReq := httptest.NewRequest(http.MethodPost, "http://admin.example.test/api/login", nil)
	recorder := httptest.NewRecorder()
	if err := server.issueSession(recorder, loginReq); err != nil {
		t.Fatalf("issueSession() error = %v", err)
	}

	var sessionCookie *http.Cookie
	var csrfCookie *http.Cookie
	for _, cookie := range recorder.Result().Cookies() {
		switch cookie.Name {
		case sessionCookieName:
			sessionCookie = cookie
		case csrfCookieName:
			csrfCookie = cookie
		}
	}
	if sessionCookie == nil || csrfCookie == nil {
		t.Fatalf("expected session and csrf cookies, got %#v", recorder.Result().Cookies())
	}

	req := httptest.NewRequest(http.MethodPost, "http://admin.example.test/api/actions/check", nil)
	req.AddCookie(sessionCookie)
	req.Header.Set("X-CSRF-Token", csrfCookie.Value)
	if !server.validCSRFToken(req) {
		t.Fatal("validCSRFToken() = false, want true")
	}

	req.Header.Set("X-CSRF-Token", "wrong-token")
	if server.validCSRFToken(req) {
		t.Fatal("validCSRFToken() = true with wrong token")
	}
}

func TestLoginFailuresLockAndClear(t *testing.T) {
	server := testServer()
	key := "203.0.113.10"
	for i := 0; i < maxLoginFailures-1; i++ {
		if retryAfter := server.recordLoginFailure(key); retryAfter > 0 {
			t.Fatalf("recordLoginFailure() retryAfter = %v before limit", retryAfter)
		}
	}
	if retryAfter := server.recordLoginFailure(key); retryAfter <= 0 {
		t.Fatalf("recordLoginFailure() retryAfter = %v, want positive lock duration", retryAfter)
	}
	if retryAfter := server.loginRetryAfter(key); retryAfter <= 0 {
		t.Fatalf("loginRetryAfter() = %v, want positive lock duration", retryAfter)
	}
	server.clearLoginFailures(key)
	if retryAfter := server.loginRetryAfter(key); retryAfter != 0 {
		t.Fatalf("loginRetryAfter() after clear = %v, want 0", retryAfter)
	}
}

func TestActionQueueDedupesByName(t *testing.T) {
	server := testServer()
	action := queuedAction{Name: "refresh", Label: "刷新登录"}
	server.actionMu.Lock()
	server.enqueueActionLocked(action)
	server.enqueueActionLocked(action)
	server.actionMu.Unlock()
	if got := len(server.queue); got != 1 {
		t.Fatalf("queued actions = %d, want 1", got)
	}
}

func testServer() *Server {
	return &Server{
		adminCfg: config.AdminConfig{
			Username: "admin",
			Password: "secret",
		},
		sessions:      make(map[string]sessionInfo),
		loginFailures: make(map[string]loginFailure),
	}
}
