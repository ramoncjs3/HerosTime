package auth

import (
	"context"
	"fmt"

	"oldbeggar-refactor/internal/captcha"
	"oldbeggar-refactor/internal/config"
	"oldbeggar-refactor/internal/httputil"
)

type Account struct {
	Username string
	Password string
}

type Credentials struct {
	LoginID     string
	Token       string
	DisplayName string
	Sign        string
}

type Provider interface {
	Login(ctx context.Context, account Account) (Credentials, error)
}

func NewProvider(kind string, cfg config.AuthConfig, captchaCfg config.CaptchaConfig, httpClient *httputil.Client) (Provider, error) {
	switch kind {
	case "official", "apple":
		return &OfficialProvider{http: httpClient}, nil
	case "h5":
		return &H5Provider{http: httpClient, cfg: cfg}, nil
	case "baozou":
		recognizer, err := captcha.New(captchaCfg, httpClient)
		if err != nil {
			return nil, err
		}
		return &BaozouProvider{http: httpClient, cfg: cfg, captchaCfg: captchaCfg, captchaRec: recognizer}, nil
	default:
		return nil, fmt.Errorf("unsupported auth kind %q", kind)
	}
}
