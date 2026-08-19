package auth

// 探测用测试：实际走一遍 baozou 自动登录流程（到账号密码表单为止），
// 把各阶段页面落盘，用于确认验证码在页面中的字段名与图片地址。
// 运行方式：OLDBEGGAR_PROBE_CAPTCHA=1 go test ./internal/auth -run TestBaozouCaptchaProbe -v

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	openssl "github.com/Luzifer/go-openssl/v4"

	"oldbeggar-refactor/internal/config"
	"oldbeggar-refactor/internal/httputil"
)

var imgPattern = regexp.MustCompile(`(?is)<img\b[^>]*>`)

func TestBaozouCaptchaProbe(t *testing.T) {
	if os.Getenv("OLDBEGGAR_PROBE_CAPTCHA") != "1" {
		t.Skip("set OLDBEGGAR_PROBE_CAPTCHA=1 to run")
	}
	probeUser := "captcha_probe_user"
	if v := os.Getenv("OLDBEGGAR_PROBE_USERNAME"); v != "" {
		probeUser = v
	}
	provider := &BaozouProvider{
		http: httputil.New(config.HTTPConfig{Timeout: config.Duration{Duration: 30 * time.Second}, Retries: 1, Backoff: config.Duration{Duration: time.Second}, InsecureTLS: true}),
		cfg: config.AuthConfig{
			GameKey:     "116798",
			PackageName: "com.maple.madherogo.m4399",
		},
	}
	ctx := context.Background()

	dump := func(name string, body []byte) {
		path := fmt.Sprintf("../../state/probe-%s.html", name)
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		t.Logf("dumped %s (%d bytes)", path, len(body))
	}

	session, err := provider.http.NewSession()
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	loginURL, err := provider.fetchLoginURL(ctx, session, probeUser)
	if err != nil {
		t.Fatalf("fetch login url: %v", err)
	}
	t.Logf("login url: %s", loginURL.String())

	phonePage, err := session.Get(ctx, loginURL.String())
	if err != nil {
		t.Fatalf("open login page: %v", err)
	}
	dump("phone", phonePage.Body)

	phoneForm, err := parseLoginForm(phonePage.Body, "authorizeForm")
	if err != nil {
		t.Fatalf("parse phone form: %v", err)
	}
	phoneForm.values.Set("auth_action", "ORILOGIN")
	accountURL, err := resolveBaozouLoginAction(loginURL, phoneForm.action)
	if err != nil {
		t.Fatalf("resolve account url: %v", err)
	}
	accountPage, err := session.PostForm(ctx, accountURL.String(), phoneForm.values, nil)
	if err != nil {
		t.Fatalf("switch to account login: %v", err)
	}
	dump("account", accountPage.Body)
	t.Logf("account page url: %s", accountURL.String())
	t.Logf("img tags:\n%s", findImgTags(accountPage.Body))
	t.Logf("form values: %s", accountPageFormValues(accountPage.Body))

	password := os.Getenv("OLDBEGGAR_PROBE_PASSWORD")
	if password == "" {
		return
	}
	form, err := parseLoginForm(accountPage.Body, "authorizeForm")
	if err != nil {
		t.Fatalf("parse account form: %v", err)
	}
	encrypted, err := openssl.New().EncryptBytes("lzYW5qaXVqa", []byte(password), openssl.BytesToKeyMD5)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	form.values.Set("response_type", "TOKEN")
	form.values.Set("sec", "1")
	form.values.Set("username", probeUser)
	form.values.Set("password", string(encrypted))
	if _, ok := form.values["captcha"]; ok {
		form.values.Set("captcha", "aaaa")
	}
	actionURL, err := resolveBaozouLoginAction(accountURL, form.action)
	if err != nil {
		t.Fatalf("resolve action: %v", err)
	}
	resp, err := session.PostForm(ctx, actionURL.String(), form.values, nil)
	if err != nil {
		t.Fatalf("submit login: %v", err)
	}
	dump("submit", resp.Body)
	t.Logf("submit status=%d bytes=%d summary=%s", resp.StatusCode, len(resp.Body), summarizeLoginHTML(resp.Body))
}

func findImgTags(body []byte) string {
	matches := imgPattern.FindAll(body, -1)
	out := ""
	for _, m := range matches {
		out += string(m) + "\n"
	}
	if out == "" {
		return "(none)"
	}
	return out
}

func accountPageFormValues(body []byte) string {
	form, err := parseLoginForm(body, "authorizeForm")
	if err != nil {
		return fmt.Sprintf("parse error: %v", err)
	}
	return url.Values(form.values).Encode()
}

// TestBaozouCaptchaTriggerProbe 并发提交登录，观察图形验证码何时出现。
// 账号密码通过环境变量 OLDBEGGAR_PROBE_USERNAME / OLDBEGGAR_PROBE_PASSWORD 传入，不落盘不入仓。
func TestBaozouCaptchaTriggerProbe(t *testing.T) {
	if os.Getenv("OLDBEGGAR_PROBE_CAPTCHA") != "1" {
		t.Skip("set OLDBEGGAR_PROBE_CAPTCHA=1 to run")
	}
	username := os.Getenv("OLDBEGGAR_PROBE_USERNAME")
	password := os.Getenv("OLDBEGGAR_PROBE_PASSWORD")
	if username == "" || password == "" {
		t.Skip("set OLDBEGGAR_PROBE_USERNAME and OLDBEGGAR_PROBE_PASSWORD to run")
	}
	provider := &BaozouProvider{
		http: httputil.New(config.HTTPConfig{Timeout: config.Duration{Duration: 30 * time.Second}, Retries: 1, Backoff: config.Duration{Duration: time.Second}, InsecureTLS: true}),
		cfg: config.AuthConfig{
			GameKey:     "116798",
			PackageName: "com.maple.madherogo.m4399",
			OpenSSLKey:  "lzYW5qaXVqa",
		},
	}
	ctx := context.Background()

	// 并发 10 个会话同时走登录流程，制造风控压力触发验证码。
	const workers = 10
	type workerResult struct {
		worker int
		page   []byte
	}
	results := make(chan workerResult, workers)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			session, err := provider.http.NewSession()
			if err != nil {
				t.Logf("worker %d new session: %v", worker, err)
				return
			}
			loginURL, err := provider.fetchLoginURL(ctx, session, username)
			if err != nil {
				t.Logf("worker %d fetch login url: %v", worker, err)
				return
			}
			phonePage, err := session.Get(ctx, loginURL.String())
			if err != nil {
				t.Logf("worker %d open page: %v", worker, err)
				return
			}
			phoneForm, err := parseLoginForm(phonePage.Body, "authorizeForm")
			if err != nil {
				t.Logf("worker %d parse phone form: %v", worker, err)
				return
			}
			phoneForm.values.Set("auth_action", "ORILOGIN")
			accountURL, err := resolveBaozouLoginAction(loginURL, phoneForm.action)
			if err != nil {
				t.Logf("worker %d resolve url: %v", worker, err)
				return
			}
			accountPage, err := session.PostForm(ctx, accountURL.String(), phoneForm.values, nil)
			if err != nil {
				t.Logf("worker %d switch login: %v", worker, err)
				return
			}
			page := accountPage.Body
			pageURL := accountURL
			for attempt := 1; attempt <= 4; attempt++ {
				form, err := parseLoginForm(page, "authorizeForm")
				if err != nil {
					t.Logf("worker %d attempt %d parse form: %v (body=%s)", worker, attempt, err, summarizeLoginHTML(page))
					return
				}
				encrypted, err := openssl.New().EncryptBytes(provider.cfg.OpenSSLKey, []byte(password), openssl.BytesToKeyMD5)
				if err != nil {
					t.Logf("worker %d encrypt: %v", worker, err)
					return
				}
				form.values.Set("response_type", "TOKEN")
				form.values.Set("sec", "1")
				form.values.Set("username", username)
				form.values.Set("password", string(encrypted))
				if _, ok := form.values["captcha"]; ok {
					form.values.Set("captcha", "aaaa")
				}
				actionURL, err := resolveBaozouLoginAction(pageURL, form.action)
				if err != nil {
					t.Logf("worker %d attempt %d resolve action: %v", worker, attempt, err)
					return
				}
				resp, err := session.PostForm(ctx, actionURL.String(), form.values, nil)
				if err != nil {
					t.Logf("worker %d attempt %d submit: %v", worker, attempt, err)
					return
				}
				hasCaptchaInput := strings.Contains(string(resp.Body), `id="captcha"`) || strings.Contains(string(resp.Body), `name="captcha"`)
				hasCaptchaImg := strings.Contains(string(resp.Body), "captcha.do")
				t.Logf("worker %d attempt %d: status=%d bytes=%d captchaInput=%v captchaImg=%v summary=%s",
					worker, attempt, resp.StatusCode, len(resp.Body), hasCaptchaInput, hasCaptchaImg, summarizeLoginHTML(resp.Body))
				if hasCaptchaInput || hasCaptchaImg {
					results <- workerResult{worker: worker, page: resp.Body}
					return
				}
				page = resp.Body
				pageURL = actionURL
			}
		}(w)
	}
	wg.Wait()
	close(results)
	dumped := 0
	for r := range results {
		path := fmt.Sprintf("../../state/probe-captcha-%d.html", r.worker)
		if err := os.WriteFile(path, r.page, 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		t.Logf("captcha page dumped: %s (%d bytes)", path, len(r.page))
		dumped++
	}
	if dumped == 0 {
		t.Fatal("captcha was not triggered in any worker")
	}
}

// TestBaozouSequentialLoginProbe 顺序跑一遍程序的真实登录链路（Login 全流程，
// 失败时带退避重试），观察是否出现图形验证码。不并发。
func TestBaozouSequentialLoginProbe(t *testing.T) {
	if os.Getenv("OLDBEGGAR_PROBE_CAPTCHA") != "1" {
		t.Skip("set OLDBEGGAR_PROBE_CAPTCHA=1 to run")
	}
	username := os.Getenv("OLDBEGGAR_PROBE_USERNAME")
	password := os.Getenv("OLDBEGGAR_PROBE_PASSWORD")
	if username == "" || password == "" {
		t.Skip("set OLDBEGGAR_PROBE_USERNAME and OLDBEGGAR_PROBE_PASSWORD to run")
	}
	provider := &BaozouProvider{
		http: httputil.New(config.HTTPConfig{Timeout: config.Duration{Duration: 30 * time.Second}, Retries: 0, Backoff: config.Duration{Duration: 2 * time.Second}, InsecureTLS: true}),
		cfg: config.AuthConfig{
			GameKey:     "116798",
			PackageName: "com.maple.madherogo.m4399",
			OpenSSLKey:  "lzYW5qaXVqa",
			DESKey:      "57493415",
			ProductCode: "83313602112675691534121381984132",
			ChannelCode: "27",
		},
	}
	ctx := context.Background()

	for round := 1; round <= 5; round++ {
		creds, err := provider.Login(ctx, Account{Username: username, Password: password})
		if err == nil {
			t.Logf("round %d: login OK login_id=%s token_prefix=%.12s...", round, creds.LoginID, creds.Token)
			return
		}
		t.Logf("round %d: login failed: %v", round, err)
		if round < 5 {
			wait := time.Duration(round*20) * time.Second
			t.Logf("waiting %s before next round", wait)
			time.Sleep(wait)
		}
	}
	t.Fatal("sequential login never succeeded")
}

// TestBaozouRapidLoginProbe 模拟原程序多区服共用账号的场景：
// 同一账号几秒内连续走 27 次完整登录流程（每次新建会话），观察验证码页面。
func TestBaozouRapidLoginProbe(t *testing.T) {
	if os.Getenv("OLDBEGGAR_PROBE_CAPTCHA") != "1" {
		t.Skip("set OLDBEGGAR_PROBE_CAPTCHA=1 to run")
	}
	username := os.Getenv("OLDBEGGAR_PROBE_USERNAME")
	password := os.Getenv("OLDBEGGAR_PROBE_PASSWORD")
	if username == "" || password == "" {
		t.Skip("set OLDBEGGAR_PROBE_USERNAME and OLDBEGGAR_PROBE_PASSWORD to run")
	}
	provider := &BaozouProvider{
		http: httputil.New(config.HTTPConfig{Timeout: config.Duration{Duration: 30 * time.Second}, Retries: 0, Backoff: config.Duration{Duration: time.Second}, InsecureTLS: true}),
		cfg: config.AuthConfig{
			GameKey:     "116798",
			PackageName: "com.maple.madherogo.m4399",
			OpenSSLKey:  "lzYW5qaXVqa",
		},
	}
	ctx := context.Background()

	encrypted, err := openssl.New().EncryptBytes(provider.cfg.OpenSSLKey, []byte(password), openssl.BytesToKeyMD5)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	pwd := string(encrypted)

	for i := 1; i <= 27; i++ {
		session, err := provider.http.NewSession()
		if err != nil {
			t.Fatalf("round %d new session: %v", i, err)
		}
		loginURL, err := provider.fetchLoginURL(ctx, session, username)
		if err != nil {
			t.Logf("round %d fetch login url: %v", i, err)
			continue
		}
		phonePage, err := session.Get(ctx, loginURL.String())
		if err != nil {
			t.Logf("round %d open page: %v", i, err)
			continue
		}
		phoneForm, err := parseLoginForm(phonePage.Body, "authorizeForm")
		if err != nil {
			t.Logf("round %d parse phone form: %v", i, err)
			continue
		}
		phoneForm.values.Set("auth_action", "ORILOGIN")
		accountURL, err := resolveBaozouLoginAction(loginURL, phoneForm.action)
		if err != nil {
			t.Logf("round %d resolve: %v", i, err)
			continue
		}
		accountPage, err := session.PostForm(ctx, accountURL.String(), phoneForm.values, nil)
		if err != nil {
			t.Logf("round %d switch: %v", i, err)
			continue
		}
		form, err := parseLoginForm(accountPage.Body, "authorizeForm")
		if err != nil {
			t.Logf("round %d parse account form: %v", i, err)
			continue
		}
		form.values.Set("response_type", "TOKEN")
		form.values.Set("sec", "1")
		form.values.Set("username", username)
		form.values.Set("password", pwd)
		if _, ok := form.values["captcha"]; ok {
			form.values.Set("captcha", "aaaa")
		}
		actionURL, err := resolveBaozouLoginAction(accountURL, form.action)
		if err != nil {
			t.Logf("round %d resolve action: %v", i, err)
			continue
		}
		resp, err := session.PostForm(ctx, actionURL.String(), form.values, nil)
		if err != nil {
			t.Logf("round %d submit: %v", i, err)
			continue
		}
		hasCaptchaInput := strings.Contains(string(resp.Body), `id="captcha"`) || strings.Contains(string(resp.Body), `name="captcha"`)
		hasCaptchaImg := strings.Contains(string(resp.Body), "captcha.do")
		t.Logf("round %d: status=%d ct=%s bytes=%d captchaInput=%v captchaImg=%v summary=%s",
			i, resp.StatusCode, resp.Header.Get("Content-Type"), len(resp.Body), hasCaptchaInput, hasCaptchaImg, summarizeLoginHTML(resp.Body))
		if hasCaptchaInput || hasCaptchaImg {
			path := fmt.Sprintf("../../state/probe-captcha-rapid.html")
			if err := os.WriteFile(path, resp.Body, 0o644); err != nil {
				t.Fatalf("write: %v", err)
			}
			t.Logf("captcha page dumped: %s", path)
			return
		}
	}
	t.Log("captcha page not observed in 27 rapid logins")
}
