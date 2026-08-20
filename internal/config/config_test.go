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

func TestMiniVariantUsesDedicatedPayloadAndPassesValidation(t *testing.T) {
	if got, want := defaultServerIndexURL("mini"), defaultServerIndexURL("h5"); got != want {
		t.Fatalf("mini server index URL = %q, want %q", got, want)
	}
	const wantMiniPayload = "a515314766c66a0146918898435cb2c08938a1cf3899c350cd905566983202334bea7b42c11ddb6b32cf21a1e61ec92ce74011509d3e126e12091d5f8590ce8cb75d44f0f78bcbc433d53fdb588112e91eb5a91dec5a1c3ca504d729520b2ad6"
	if got := defaultServerListPayload("mini"); got != wantMiniPayload {
		t.Fatalf("mini server payload = %q, want dedicated channel payload", got)
	}
	if got, h5 := defaultServerListPayload("mini"), defaultServerListPayload("h5"); got == h5 {
		t.Fatal("mini server payload must not reuse h5 payload")
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
