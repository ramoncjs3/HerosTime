package auth

import (
	"encoding/json"
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

func TestParseBaozouUserTokenAcceptsObjectAndArray(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "object", raw: `{"user_token":"token-object"}`, want: "token-object"},
		{name: "array", raw: `[{"user_token":"token-array"}]`, want: "token-array"},
		{name: "camel case", raw: `[{"userToken":"token-camel"}]`, want: "token-camel"},
		{name: "nested array", raw: `{"result":[{"user_token":"token-nested"}]}`, want: "token-nested"},
		{name: "string array", raw: `["token-string"]`, want: "token-string"},
		{name: "empty", raw: `[]`, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBaozouUserToken(json.RawMessage(tt.raw))
			if err != nil {
				t.Fatalf("parseBaozouUserToken() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("parseBaozouUserToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBaozouCheckLoginPayloadUsesStringChannelCode(t *testing.T) {
	payload, err := json.Marshal(struct {
		ChannelCode string `json:"channel_code"`
		UID         string `json:"uid"`
		Token       string `json:"token"`
		UserName    string `json:"user_name"`
	}{ChannelCode: "27", UID: "100", Token: "state", UserName: "user"})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if got, ok := decoded["channel_code"].(string); !ok || got != "27" {
		t.Fatalf("channel_code = %#v, want string 27", decoded["channel_code"])
	}
}

func TestParseU8Fields(t *testing.T) {
	state, err := parseJSONInt(json.RawMessage(`"1"`))
	if err != nil || state != 1 {
		t.Fatalf("parseJSONInt() = %d, %v; want 1", state, err)
	}
	userID, err := parseJSONString(json.RawMessage(`12345`))
	if err != nil || userID != "12345" {
		t.Fatalf("parseJSONString() = %q, %v; want 12345", userID, err)
	}
}
