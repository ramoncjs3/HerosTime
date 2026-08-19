package auth

import (
	"context"
	"os"
	"testing"

	"gopkg.in/yaml.v3"

	"oldbeggar-refactor/internal/config"
	"oldbeggar-refactor/internal/httputil"
)

func TestBaozouLiveLogin(t *testing.T) {
	if os.Getenv("OLDBEGGAR_TEST_BAOZOU_LIVE") != "1" {
		t.Skip("set OLDBEGGAR_TEST_BAOZOU_LIVE=1 to run")
	}
	configPath := os.Getenv("OLDBEGGAR_TEST_CONFIG")
	if configPath == "" {
		configPath = "../../configs/config.local.yaml"
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var cfg struct {
		HTTP     config.HTTPConfig    `yaml:"http"`
		Captcha  config.CaptchaConfig `yaml:"captcha"`
		Variants []config.Variant     `yaml:"variants"`
	}
	if err := yaml.Unmarshal([]byte(os.ExpandEnv(string(raw))), &cfg); err != nil {
		t.Fatalf("parse config: %v", err)
	}

	for _, variant := range cfg.Variants {
		if variant.Kind != "baozou" || len(variant.Accounts) == 0 {
			continue
		}
		accountConfig := variant.AccountFor(variant.Accounts[0])
		provider, err := NewProvider(variant.Kind, variant.Auth, cfg.Captcha, httputil.New(cfg.HTTP), NewBaozouCooldowns())
		if err != nil {
			t.Fatalf("create provider: %v", err)
		}
		credentials, err := provider.Login(context.Background(), Account{
			Username: accountConfig.Username,
			Password: accountConfig.Password,
		})
		if err != nil {
			t.Fatalf("live login: %v", err)
		}
		if credentials.LoginID == "" || credentials.Token == "" {
			t.Fatal("live login returned empty credentials")
		}
		return
	}
	t.Fatal("enabled baozou account not found")
}
