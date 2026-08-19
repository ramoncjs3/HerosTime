package config

import "testing"

// TestCaptchaDefaultDisabled 钉住评审结论：旧配置没有 captcha 段时识别保持关闭，
// 升级不改变生产行为；显式 enabled: true 才开启。
func TestCaptchaDefaultDisabled(t *testing.T) {
	if (CaptchaConfig{}).IsEnabled() {
		t.Fatal("captcha must default to disabled")
	}

	enabled, disabled := true, false
	if !(CaptchaConfig{Enabled: &enabled}).IsEnabled() {
		t.Fatal("captcha with enabled=true must be enabled")
	}
	if (CaptchaConfig{Enabled: &disabled}).IsEnabled() {
		t.Fatal("captcha with enabled=false must be disabled")
	}
}

func TestMiniVariantUsesH5DefaultsAndPassesValidation(t *testing.T) {
	if got, want := defaultServerIndexURL("mini"), defaultServerIndexURL("h5"); got != want {
		t.Fatalf("mini server index URL = %q, want %q", got, want)
	}
	if got, want := defaultServerListPayload("mini"), defaultServerListPayload("h5"); got != want {
		t.Fatalf("mini server payload = %q, want h5 payload", got)
	}

	cfg := &Config{
		Storage: StorageConfig{Type: "json"},
		Catalog: CatalogConfig{ItemFile: "items.json", ItemNameFile: "item-names.json"},
		Variants: []Variant{{
			Name:        "mini",
			Kind:        "mini",
			ServerRules: []ServerRule{{MatchPrefix: "mini", CodePrefix: "m"}},
			Accounts:    []Account{{Server: "m1"}},
		}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("mini variant rejected: %v", err)
	}
}
