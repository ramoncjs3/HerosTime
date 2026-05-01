package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/Luzifer/go-openssl/v4"

	"oldbeggar-refactor/internal/config"
	"oldbeggar-refactor/internal/httputil"
	"oldbeggar-refactor/internal/protocol"
)

type BaozouProvider struct {
	http *httputil.Client
	cfg  config.AuthConfig
}

func (p *BaozouProvider) Login(ctx context.Context, account Account) (Credentials, error) {
	if account.Username == "" || account.Password == "" {
		return Credentials{}, fmt.Errorf("baozou username and password are required")
	}

	loginURL, err := p.fetchLoginURL(ctx)
	if err != nil {
		return Credentials{}, err
	}
	token, uid, err := p.authorize(ctx, loginURL, account)
	if err != nil {
		return Credentials{}, err
	}
	userToken, err := p.checkQuickAPI(ctx, account.Username, uid, token)
	if err != nil {
		return Credentials{}, err
	}
	return Credentials{LoginID: uid, Token: userToken}, nil
}

func (p *BaozouProvider) fetchLoginURL(ctx context.Context) (*url.URL, error) {
	device := fmt.Sprintf(`{"DEVICE_IDENTIFIER":"","SCREEN_RESOLUTION":"1170*1872","DEVICE_MODEL":"Note10","DEVICE_MODEL_VERSION":"6.0.1","SYSTEM_VERSION":"6.0.1","PLATFORM_TYPE":"Android","SDK_VERSION":"2.37.0.211","GAME_KEY":"%s","GAME_VERSION":"2.3.3","BID":"%s","IMSI":"","PHONE":"","RUNTIME":"Origin","CANAL_IDENTIFIER":"","UDID":"","DEBUG":"false","NETWORK_TYPE":"WIFI","DEVICE_IDENTIFIER_SM":"20210827124041a3379d1da96eaf928ca0c54799eb58f701b8208b2b0f29c1","SERVER_SERIAL":"0","UID":"0000000000"}`, p.cfg.GameKey, p.cfg.PackageName)
	form := url.Values{"device": {device}}
	resp, err := p.http.PostForm(ctx, "https://m.4399api.com/openapiv2/oauth.html", form, nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		Result struct {
			LoginURL string `json:"login_url"`
		} `json:"result"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, err
	}
	if result.Result.LoginURL == "" {
		return nil, fmt.Errorf("baozou oauth returned empty login_url")
	}
	return url.Parse(result.Result.LoginURL)
}

func (p *BaozouProvider) authorize(ctx context.Context, loginURL *url.URL, account Account) (string, string, error) {
	encrypted, err := openssl.New().EncryptBytes(p.cfg.OpenSSLKey, []byte(account.Password), openssl.BytesToKeyMD5)
	if err != nil {
		return "", "", err
	}
	query := loginURL.Query()
	form := url.Values{}
	form.Set("response_type", "TOKEN")
	form.Set("sec", "1")
	form.Set("username", account.Username)
	form.Set("password", string(encrypted))
	form.Set("client_id", query.Get("client_id"))
	form.Set("ref", query.Get("ref"))
	form.Set("state", query.Get("state"))
	form.Set("redirect_uri", query.Get("redirect_uri"))

	resp, err := p.http.PostForm(ctx, "https://ptlogin.4399.com/oauth2/loginAndAuthorize.do", form, nil)
	if err != nil {
		return "", "", err
	}
	var result struct {
		Result struct {
			State string      `json:"state"`
			UID   interface{} `json:"uid"`
		} `json:"result"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return "", "", err
	}
	uid := normalizeUID(result.Result.UID)
	if result.Result.State == "" || uid == "" {
		return "", "", fmt.Errorf("baozou authorize missing state or uid")
	}
	return result.Result.State, uid, nil
}

func (p *BaozouProvider) checkQuickAPI(ctx context.Context, username, uid, token string) (string, error) {
	payload := fmt.Sprintf(`{
  "channel_code" : %s,
  "uid" : "%s",
  "token" : "%s",
  "user_name" : "%s"}`, p.cfg.ChannelCode, uid, token, username)
	encrypted, err := protocol.DESEncrypt(payload, p.cfg.DESKey)
	if err != nil {
		return "", err
	}
	form := url.Values{}
	form.Set("json_data", encrypted)
	form.Set("product_code", p.cfg.ProductCode)
	resp, err := p.http.PostForm(ctx, "http://sdkapi03.quickapi.net/v2/users/checkLogin", form, nil)
	if err != nil {
		return "", err
	}
	var result struct {
		Data struct {
			UserToken string `json:"user_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return "", err
	}
	if result.Data.UserToken == "" {
		return "", fmt.Errorf("baozou checkLogin returned empty user_token")
	}
	return result.Data.UserToken, nil
}

func normalizeUID(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case json.Number:
		return v.String()
	default:
		return ""
	}
}
