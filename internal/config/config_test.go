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
