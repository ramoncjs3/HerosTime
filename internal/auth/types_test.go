package auth

import (
	"testing"

	"oldbeggar-refactor/internal/config"
	"oldbeggar-refactor/internal/httputil"
)

func TestNewProviderSupportsMini(t *testing.T) {
	provider, err := NewProvider("mini", config.AuthConfig{}, config.CaptchaConfig{}, httputil.New(config.HTTPConfig{}), NewBaozouCooldowns())
	if err != nil {
		t.Fatalf("NewProvider(mini): %v", err)
	}
	if _, ok := provider.(*H5Provider); !ok {
		t.Fatalf("NewProvider(mini) returned %T, want *H5Provider", provider)
	}
}
