package auth

import (
	"net/url"
	"testing"
	"time"

	"oldbeggar-refactor/internal/config"
)

func TestParseLoginForm(t *testing.T) {
	body := []byte(`
<html><body>
  <form name="authorizeForm" action="/oauth2/authorize.do?from=mobile&amp;v=1" method="POST">
    <input type="hidden" name="client_id" value="client-1">
    <input type='hidden' name='state' value='state-1'>
    <input type="hidden" name="auth_action" value="LOGIN">
    <input type="text" name="username" value="">
  </form>
</body></html>`)

	form, err := parseLoginForm(body, "authorizeForm")
	if err != nil {
		t.Fatalf("parseLoginForm() error = %v", err)
	}
	if form.action != "/oauth2/authorize.do?from=mobile&v=1" {
		t.Fatalf("action = %q", form.action)
	}
	if got := form.values.Get("client_id"); got != "client-1" {
		t.Fatalf("client_id = %q", got)
	}
	if got := form.values.Get("state"); got != "state-1" {
		t.Fatalf("state = %q", got)
	}
	if got := form.values.Get("auth_action"); got != "LOGIN" {
		t.Fatalf("auth_action = %q", got)
	}
}

func TestResolveBaozouLoginAction(t *testing.T) {
	base, err := url.Parse("https://ptlogin.4399.com/oauth2/authorize.do?state=one&sdk=1&sdk_version=3.18.0.682")
	if err != nil {
		t.Fatal(err)
	}

	resolved, err := resolveBaozouLoginAction(base, "/oauth2/loginAndAuthorize.do")
	if err != nil {
		t.Fatalf("resolveBaozouLoginAction() error = %v", err)
	}
	if got, want := resolved.String(), "https://ptlogin.4399.com/oauth2/loginAndAuthorize.do?sdk=1&sdk_version=3.18.0.682"; got != want {
		t.Fatalf("resolved URL = %q, want %q", got, want)
	}
}

func TestResolveBaozouLoginActionRejectsUnexpectedHost(t *testing.T) {
	base, err := url.Parse("https://ptlogin.4399.com/oauth2/authorize.do")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := resolveBaozouLoginAction(base, "https://example.com/login"); err == nil {
		t.Fatal("resolveBaozouLoginAction() accepted unexpected host")
	}
}

func TestBaozouCooldownSharedAcrossProviders(t *testing.T) {
	now := time.Date(2026, 8, 19, 21, 0, 0, 0, time.UTC)
	cooldowns := NewBaozouCooldowns()
	cooldowns.now = func() time.Time { return now }

	first := &BaozouProvider{
		captchaCfg: config.CaptchaConfig{
			RateLimitCooldown: config.Duration{Duration: 5 * time.Minute},
		},
		cooldowns: cooldowns,
	}
	second := &BaozouProvider{cooldowns: cooldowns}

	wantUntil := now.Add(5 * time.Minute)
	if got := first.markRateLimited("shared-account"); !got.Equal(wantUntil) {
		t.Fatalf("cooldown until = %s, want %s", got, wantUntil)
	}
	if got, ok := second.cooldownFor("shared-account"); !ok || !got.Equal(wantUntil) {
		t.Fatalf("second provider cooldown = %s, %v; want %s, true", got, ok, wantUntil)
	}

	now = wantUntil
	if got, ok := second.cooldownFor("shared-account"); ok {
		t.Fatalf("expired cooldown still active at %s", got)
	}
}
