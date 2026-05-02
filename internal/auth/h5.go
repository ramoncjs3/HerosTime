package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Luzifer/go-openssl/v4"

	"oldbeggar-refactor/internal/config"
	"oldbeggar-refactor/internal/httputil"
)

// pauthTTL is how long a successfully obtained Pauth cookie is considered valid.
// 4399 session tokens typically expire after several hours; 6 h is conservative.
const pauthTTL = 6 * time.Hour

// pauthErrTTL is how long a login error is cached before retrying.
// Kept short so transient failures (network blip, 500) are retried promptly.
const pauthErrTTL = 2 * time.Minute

type H5Provider struct {
	http     *httputil.Client
	cfg      config.AuthConfig
	pauthMu  sync.Mutex
	pauthKey string
	pauth    string
	pauthAt  time.Time
	pauthErr error
}

func (p *H5Provider) Login(ctx context.Context, account Account) (Credentials, error) {
	if account.Username == "" || account.Password == "" {
		return Credentials{}, fmt.Errorf("h5 username and password are required")
	}
	pauth, err := p.cachedPauth(ctx, account)
	if err != nil {
		return Credentials{}, err
	}
	return p.grant(ctx, pauth)
}

func (p *H5Provider) cachedPauth(ctx context.Context, account Account) (string, error) {
	key := account.Username + "\x00" + account.Password
	p.pauthMu.Lock()
	defer p.pauthMu.Unlock()
	if p.pauth != "" && p.pauthKey == key && time.Since(p.pauthAt) < pauthTTL {
		return p.pauth, nil
	}
	if p.pauthErr != nil && p.pauthKey == key && time.Since(p.pauthAt) < pauthErrTTL {
		return "", p.pauthErr
	}
	pauth, err := p.login4399Cookie(ctx, account)
	if err != nil {
		p.pauthKey = key
		p.pauthAt = time.Now()
		p.pauthErr = err
		return "", err
	}
	p.pauthKey = key
	p.pauthAt = time.Now()
	p.pauth = pauth
	p.pauthErr = nil
	return pauth, nil
}

func (p *H5Provider) login4399Cookie(ctx context.Context, account Account) (string, error) {
	encrypted, err := openssl.New().EncryptBytes(p.cfg.OpenSSLKey, []byte(account.Password), openssl.BytesToKeyMD5)
	if err != nil {
		return "", err
	}
	body := fmt.Sprintf("username=%s&password=%s&sec=1", url.QueryEscape(account.Username), url.QueryEscape(string(encrypted)))
	headers := map[string]string{
		"Content-Type":    "application/x-www-form-urlencoded",
		"Origin":          "http://ptlogin.4399.com",
		"Referer":         "http://ptlogin.4399.com/ptlogin/loginFrame.do?postLoginHandler=default&displayMode=popup&appId=www_home",
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
	}
	resp, err := p.http.Post(ctx, "http://ptlogin.4399.com/ptlogin/login.do?v=1", "application/x-www-form-urlencoded", []byte(body), headers)
	if err != nil {
		return "", err
	}
	if cookie := cookieByName(resp.Cookies, "Pauth"); cookie != "" {
		return cookie, nil
	}
	return "", fmt.Errorf("h5 login returned empty Pauth, status=%d", resp.StatusCode)
}

func (p *H5Provider) grant(ctx context.Context, pauth string) (Credentials, error) {
	form := url.Values{}
	form.Set("gameId", p.cfg.GameID)
	form.Set("authType", "cookie")
	form.Set("cookieValue", pauth)

	resp, err := p.http.PostForm(ctx, "http://h.api.4399.com/intermodal/user/grant2", form, nil)
	if err != nil {
		return Credentials{}, err
	}

	var result struct {
		Data struct {
			Game struct {
				GameURL string `json:"gameUrl"`
			} `json:"game"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return Credentials{}, err
	}
	if result.Data.Game.GameURL == "" {
		return Credentials{}, fmt.Errorf("h5 grant2 returned empty gameUrl")
	}
	gameURL, err := url.Parse(result.Data.Game.GameURL)
	if err != nil {
		return Credentials{}, err
	}
	query := gameURL.Query()
	creds := Credentials{
		LoginID:     query.Get("userId"),
		DisplayName: query.Get("account"),
		Sign:        query.Get("sign"),
	}
	if creds.LoginID == "" || creds.DisplayName == "" || creds.Sign == "" {
		return Credentials{}, fmt.Errorf("h5 grant2 missing userId/account/sign")
	}
	return creds, nil
}

func cookieByName(cookies []*http.Cookie, name string) string {
	for _, cookie := range cookies {
		if strings.EqualFold(cookie.Name, name) {
			return cookie.Value
		}
	}
	return ""
}
