package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Luzifer/go-openssl/v4"

	"oldbeggar-refactor/internal/captcha"
	"oldbeggar-refactor/internal/config"
	"oldbeggar-refactor/internal/httputil"
	"oldbeggar-refactor/internal/protocol"
)

type BaozouProvider struct {
	http       *httputil.Client
	cfg        config.AuthConfig
	captchaCfg config.CaptchaConfig
	captchaRec captcha.Recognizer

	cooldownInit sync.Once
	cooldowns    *BaozouCooldowns
}

// BaozouCooldowns stores account cooldowns independently of a provider
// instance. Runner keeps one store for the lifetime of the app so a new
// provider created by a later refresh still observes an earlier rate limit.
type BaozouCooldowns struct {
	mu    sync.Mutex
	until map[string]time.Time
	now   func() time.Time
}

func NewBaozouCooldowns() *BaozouCooldowns {
	return &BaozouCooldowns{
		until: make(map[string]time.Time),
		now:   time.Now,
	}
}

const (
	baozouOAuthURL        = "https://m.4399api.com/openapiv2/oauth.html"
	baozouSDKVersion      = "3.18.0.682"
	baozouGameVersion     = "4.0.2"
	baozouGameVersionCode = 402
	baozouDeviceID        = "20210827124041a3379d1da96eaf928ca0c54799eb58f701b8208b2b0f29c1"
)

var (
	formPattern           = regexp.MustCompile(`(?is)<form\b[^>]*>.*?</form>`)
	inputPattern          = regexp.MustCompile(`(?is)<input\b[^>]*>`)
	attributePattern      = regexp.MustCompile(`(?i)([a-zA-Z_:][a-zA-Z0-9_:.-]*)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'=<>]+))`)
	titlePattern          = regexp.MustCompile(`(?is)<title\b[^>]*>(.*?)</title>`)
	loginErrorPattern     = regexp.MustCompile(`(?is)<[^>]*\bid\s*=\s*(?:"login_err_msg"|'login_err_msg')[^>]*>(.*?)</[^>]+>`)
	tagPattern            = regexp.MustCompile(`(?is)<[^>]+>`)
	spacePattern          = regexp.MustCompile(`\s+`)
	imgTagPattern         = regexp.MustCompile(`(?is)<img\b[^>]*>`)
	captchaInputPattern   = regexp.MustCompile(`(?is)<input\b[^>]*\b(?:id|name)\s*=\s*["']?captcha["']?[^>]*>`)
	captchaRefreshPattern = regexp.MustCompile(`UniLoginChangPIC\(\s*['"]([^'"]+)['"]`)
	captchaIDPattern      = regexp.MustCompile(`captchaId=([A-Za-z0-9_-]+)`)
)

type loginForm struct {
	action string
	values url.Values
}

func (p *BaozouProvider) Login(ctx context.Context, account Account) (Credentials, error) {
	if account.Username == "" || account.Password == "" {
		return Credentials{}, fmt.Errorf("baozou username and password are required")
	}
	if until, ok := p.cooldownFor(account.Username); ok {
		return Credentials{}, fmt.Errorf("baozou account %s is in rate-limit cooldown until %s, skipping login", account.Username, until.Format("15:04:05"))
	}

	session, err := p.http.NewSession()
	if err != nil {
		return Credentials{}, fmt.Errorf("baozou create login session: %w", err)
	}
	loginURL, err := p.fetchLoginURL(ctx, session, account.Username)
	if err != nil {
		return Credentials{}, err
	}
	token, uid, err := p.authorize(ctx, session, loginURL, account)
	if err != nil {
		return Credentials{}, err
	}
	userToken, err := p.checkQuickAPI(ctx, account.Username, uid, token)
	if err != nil {
		return Credentials{}, err
	}
	return Credentials{LoginID: uid, Token: userToken}, nil
}

func (p *BaozouProvider) fetchLoginURL(ctx context.Context, session *httputil.Client, username string) (*url.URL, error) {
	device, err := json.Marshal(map[string]interface{}{
		"DEVICE_IDENTIFIER":    baozouDeviceID,
		"SCREEN_RESOLUTION":    "1170*1872",
		"DEVICE_MODEL":         "Note10",
		"DEVICE_MODEL_VERSION": "6.0.1",
		"SYSTEM_VERSION":       "6.0.1",
		"PLATFORM_TYPE":        "Android",
		"SDK_VERSION":          baozouSDKVersion,
		"GAME_KEY":             p.cfg.GameKey,
		"GAME_VERSION":         baozouGameVersion,
		"GAME_VERSION_CODE":    baozouGameVersionCode,
		"BID":                  p.cfg.PackageName,
		"RUNTIME":              "Origin",
		"CANAL_IDENTIFIER":     "",
		"UDID":                 "",
		"DEBUG":                "false",
		"VIP_INFO":             "",
		"TEAM":                 "",
		"NETWORK_TYPE":         "WIFI",
		"DEVICE_IDENTIFIER_SM": baozouDeviceID,
		"SERVER_SERIAL":        "0",
		"UID":                  "",
	})
	if err != nil {
		return nil, fmt.Errorf("baozou encode device: %w", err)
	}
	form := url.Values{}
	form.Set("device", string(device))
	form.Set("usernames", username)
	form.Set("top_bar", "1")
	resp, err := session.PostForm(ctx, baozouOAuthURL, form, nil)
	if err != nil {
		return nil, fmt.Errorf("baozou request oauth URL: %w", err)
	}
	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Result  struct {
			LoginURL string `json:"login_url"`
		} `json:"result"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("baozou parse oauth response: %w", err)
	}
	if result.Result.LoginURL == "" {
		return nil, fmt.Errorf("baozou oauth returned empty login_url (code=%d message=%s)", result.Code, result.Message)
	}
	loginURL, err := url.Parse(result.Result.LoginURL)
	if err != nil {
		return nil, fmt.Errorf("baozou parse login URL: %w", err)
	}
	if err := validateBaozouLoginURL(loginURL); err != nil {
		return nil, err
	}
	return loginURL, nil
}

func (p *BaozouProvider) authorize(ctx context.Context, session *httputil.Client, loginURL *url.URL, account Account) (string, string, error) {
	page, pageURL, err := p.openAccountPage(ctx, session, loginURL)
	if err != nil {
		return "", "", err
	}

	maxAttempts := p.captchaCfg.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	attempts := 0
	for {
		form, err := parseLoginForm(page, "authorizeForm")
		if err != nil {
			return "", "", fmt.Errorf("baozou parse account login page: %w", err)
		}

		encrypted, err := openssl.New().EncryptBytes(p.cfg.OpenSSLKey, []byte(account.Password), openssl.BytesToKeyMD5)
		if err != nil {
			return "", "", fmt.Errorf("baozou encrypt password: %w", err)
		}
		form.values.Set("response_type", "TOKEN")
		form.values.Set("sec", "1")
		form.values.Set("username", account.Username)
		form.values.Set("password", string(encrypted))
		if pageRequiresCaptcha(page) {
			code, err := p.solveCaptcha(ctx, session, page, pageURL)
			if err != nil {
				return "", "", err
			}
			form.values.Set("captcha", code)
			log.Printf("baozou captcha detected, submitting ocr result (%d chars)", len(code))
		}

		loginActionURL, err := resolveBaozouLoginAction(pageURL, form.action)
		if err != nil {
			return "", "", err
		}
		resp, err := session.PostForm(ctx, loginActionURL.String(), form.values, nil)
		if err != nil {
			return "", "", fmt.Errorf("baozou submit account login: %w", err)
		}
		if state, uid, ok := parseAuthorizeResult(resp.Body); ok {
			return state, uid, nil
		}
		if isRateLimited(resp) {
			until := p.markRateLimited(account.Username)
			return "", "", fmt.Errorf("baozou login rate limited, account cooled down until %s (%s)", until.Format("15:04:05"), strings.TrimSpace(string(resp.Body)))
		}
		attempts++
		if formPattern.Match(resp.Body) {
			if !pageRequiresCaptcha(resp.Body) {
				// 非验证码导致的登录失败（如密码错误），重试没有意义，直接报错。
				return "", "", fmt.Errorf("baozou account login failed: %s", summarizeLoginHTML(resp.Body))
			}
			// 验证码识别错误等服务端返回新验证码页的情况，换一张验证码重试。
			if attempts >= maxAttempts {
				return "", "", fmt.Errorf("baozou login failed after %d captcha attempts: %s", maxAttempts, summarizeLoginHTML(resp.Body))
			}
			log.Printf("baozou captcha attempt %d/%d failed, refreshing captcha", attempts, maxAttempts)
			page = resp.Body
			pageURL = loginActionURL
			continue
		}
		contentType := resp.Header.Get("Content-Type")
		return "", "", fmt.Errorf("baozou account login response parse failed (content-type=%s %s): %w", contentType, summarizeLoginHTML(resp.Body), errJSONLoginResponse)
	}
}

var errJSONLoginResponse = fmt.Errorf("login response is not a json authorize result")

// openAccountPage 走手机号页 -> 账号密码页的切换流程，返回账号页 HTML 与其 URL。
func (p *BaozouProvider) openAccountPage(ctx context.Context, session *httputil.Client, loginURL *url.URL) ([]byte, *url.URL, error) {
	phonePage, err := session.Get(ctx, loginURL.String())
	if err != nil {
		return nil, nil, fmt.Errorf("baozou open mobile login page: %w", err)
	}
	phoneForm, err := parseLoginForm(phonePage.Body, "authorizeForm")
	if err != nil {
		return nil, nil, fmt.Errorf("baozou parse mobile login page: %w", err)
	}
	phoneForm.values.Set("auth_action", "ORILOGIN")
	accountURL, err := resolveBaozouLoginAction(loginURL, phoneForm.action)
	if err != nil {
		return nil, nil, err
	}
	accountPage, err := session.PostForm(ctx, accountURL.String(), phoneForm.values, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("baozou switch to account login: %w", err)
	}
	return accountPage.Body, accountURL, nil
}

func parseAuthorizeResult(body []byte) (string, string, bool) {
	var result struct {
		Code    interface{} `json:"code"`
		Message string      `json:"message"`
		Result  struct {
			State string      `json:"state"`
			UID   interface{} `json:"uid"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", false
	}
	uid := normalizeUID(result.Result.UID)
	if result.Result.State == "" || uid == "" {
		return "", "", false
	}
	return result.Result.State, uid, true
}

// markRateLimited 记录账号限流冷却，避免跨刷新周期继续撞风控。
func (p *BaozouProvider) markRateLimited(username string) time.Time {
	cooldown := p.captchaCfg.RateLimitCooldown.Duration
	if cooldown <= 0 {
		cooldown = 5 * time.Minute
	}
	return p.cooldownStore().mark(username, cooldown)
}

func (p *BaozouProvider) cooldownFor(username string) (time.Time, bool) {
	return p.cooldownStore().active(username)
}

func (p *BaozouProvider) cooldownStore() *BaozouCooldowns {
	p.cooldownInit.Do(func() {
		if p.cooldowns == nil {
			p.cooldowns = NewBaozouCooldowns()
		}
	})
	return p.cooldowns
}

func (c *BaozouCooldowns) mark(username string, duration time.Duration) time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.currentTime()
	until := now.Add(duration)
	if c.until == nil {
		c.until = make(map[string]time.Time)
	}
	c.until[username] = until
	return until
}

func (c *BaozouCooldowns) active(username string) (time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	until, ok := c.until[username]
	if !ok {
		return time.Time{}, false
	}
	if !c.currentTime().Before(until) {
		delete(c.until, username)
		return time.Time{}, false
	}
	return until, true
}

func (c *BaozouCooldowns) currentTime() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

// isRateLimited 识别 4399 风控限流响应（HTTP 202 + “请稍后再试~”纯文本）。
func isRateLimited(resp *httputil.Response) bool {
	return resp.StatusCode == http.StatusAccepted || bytes.Contains(resp.Body, []byte("请稍后再试"))
}

// pageRequiresCaptcha 判断登录页是否渲染了图形验证码（输入框或验证码图片）。
func pageRequiresCaptcha(page []byte) bool {
	return captchaInputPattern.Match(page) || captchaImageURLInPage(page)
}

func captchaImageURLInPage(page []byte) bool {
	for _, tag := range imgTagPattern.FindAll(page, -1) {
		attributes := parseHTMLAttributes(string(tag))
		if attributes["id"] == "captcha_img" || strings.Contains(attributes["src"], "captcha.do") {
			return true
		}
	}
	return captchaRefreshPattern.Match(page)
}

// captchaImageURL 从登录页中提取验证码图片地址：
// 优先用 <img id="captcha_img"> 的 src，其次从 UniLoginChangPIC('id') / captchaId= 推导。
func captchaImageURL(page []byte, base *url.URL) (string, bool) {
	for _, tag := range imgTagPattern.FindAll(page, -1) {
		attributes := parseHTMLAttributes(string(tag))
		src := strings.TrimSpace(attributes["src"])
		if attributes["id"] != "captcha_img" && !strings.Contains(src, "captcha.do") {
			continue
		}
		if src == "" {
			break
		}
		parsed, err := url.Parse(html.UnescapeString(src))
		if err != nil {
			return "", false
		}
		return base.ResolveReference(parsed).String(), true
	}
	var id string
	if match := captchaRefreshPattern.FindSubmatch(page); len(match) > 1 {
		id = string(match[1])
	} else if match := captchaIDPattern.FindSubmatch(page); len(match) > 1 {
		id = string(match[1])
	}
	if id == "" {
		return "", false
	}
	return "https://ptlogin.4399.com/ptlogin/captcha.do?captchaId=" + url.QueryEscape(id), true
}

// solveCaptcha 拉取验证码图片并调用 OCR 识别。
func (p *BaozouProvider) solveCaptcha(ctx context.Context, session *httputil.Client, page []byte, pageURL *url.URL) (string, error) {
	if p.captchaRec == nil {
		return "", fmt.Errorf("baozou login requires captcha but captcha OCR is disabled (set captcha.enabled=true)")
	}
	imageURL, ok := captchaImageURL(page, pageURL)
	if !ok {
		return "", fmt.Errorf("baozou captcha required but captcha image not found in login page")
	}
	imageResp, err := session.Get(ctx, imageURL)
	if err != nil {
		return "", fmt.Errorf("baozou fetch captcha image: %w", err)
	}
	code, err := p.captchaRec.Recognize(ctx, imageResp.Body)
	if err != nil {
		return "", fmt.Errorf("baozou recognize captcha: %w", err)
	}
	return code, nil
}

func parseLoginForm(body []byte, name string) (loginForm, error) {
	for _, match := range formPattern.FindAll(body, -1) {
		openingEnd := strings.IndexByte(string(match), '>')
		if openingEnd < 0 {
			continue
		}
		attributes := parseHTMLAttributes(string(match[:openingEnd+1]))
		if !strings.EqualFold(attributes["name"], name) {
			continue
		}
		action := strings.TrimSpace(attributes["action"])
		if action == "" {
			return loginForm{}, fmt.Errorf("form %q has no action", name)
		}
		values := url.Values{}
		for _, input := range inputPattern.FindAll(match, -1) {
			inputAttributes := parseHTMLAttributes(string(input))
			inputName := strings.TrimSpace(inputAttributes["name"])
			if inputName != "" {
				values.Set(inputName, inputAttributes["value"])
			}
		}
		return loginForm{action: action, values: values}, nil
	}
	return loginForm{}, fmt.Errorf("form %q not found", name)
}

func parseHTMLAttributes(tag string) map[string]string {
	attributes := make(map[string]string)
	for _, match := range attributePattern.FindAllStringSubmatch(tag, -1) {
		value := match[2]
		if value == "" {
			value = match[3]
		}
		if value == "" {
			value = match[4]
		}
		attributes[strings.ToLower(match[1])] = html.UnescapeString(value)
	}
	return attributes
}

func summarizeLoginHTML(body []byte) string {
	details := make([]string, 0, 3)
	if match := titlePattern.FindSubmatch(body); len(match) > 1 {
		if title := cleanHTMLText(string(match[1])); title != "" {
			details = append(details, "title="+title)
		}
	}
	if match := loginErrorPattern.FindSubmatch(body); len(match) > 1 {
		if message := cleanHTMLText(string(match[1])); message != "" {
			details = append(details, "message="+message)
		}
	}
	for _, match := range formPattern.FindAll(body, -1) {
		openingEnd := strings.IndexByte(string(match), '>')
		if openingEnd < 0 {
			continue
		}
		attributes := parseHTMLAttributes(string(match[:openingEnd+1]))
		if action := strings.TrimSpace(attributes["action"]); action != "" {
			details = append(details, "form="+action)
			break
		}
	}
	if len(details) == 0 {
		return fmt.Sprintf("html_bytes=%d", len(body))
	}
	return strings.Join(details, " ")
}

func cleanHTMLText(value string) string {
	value = tagPattern.ReplaceAllString(value, " ")
	value = html.UnescapeString(value)
	return strings.TrimSpace(spacePattern.ReplaceAllString(value, " "))
}

func resolveBaozouLoginAction(base *url.URL, action string) (*url.URL, error) {
	actionURL, err := url.Parse(strings.TrimSpace(html.UnescapeString(action)))
	if err != nil {
		return nil, fmt.Errorf("baozou parse login form action: %w", err)
	}
	resolved := base.ResolveReference(actionURL)
	resolvedQuery := resolved.Query()
	baseQuery := base.Query()
	for _, key := range []string{"channel", "sdk", "sdk_version"} {
		if resolvedQuery.Get(key) == "" && baseQuery.Get(key) != "" {
			resolvedQuery.Set(key, baseQuery.Get(key))
		}
	}
	resolved.RawQuery = resolvedQuery.Encode()
	if err := validateBaozouLoginURL(resolved); err != nil {
		return nil, err
	}
	return resolved, nil
}

func validateBaozouLoginURL(loginURL *url.URL) error {
	host := strings.ToLower(loginURL.Hostname())
	if loginURL.Scheme != "https" || (host != "ptlogin.4399.com" && host != "extlogin.4399.com") {
		return fmt.Errorf("baozou rejected unexpected login URL host %q", loginURL.Host)
	}
	return nil
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
