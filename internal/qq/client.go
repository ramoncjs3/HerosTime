package qq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"oldbeggar-refactor/internal/config"
	"oldbeggar-refactor/internal/httputil"
)

type Client struct {
	http     *httputil.Client
	config   config.QQConfig
	sendMu   sync.Mutex
	mapMu    sync.RWMutex
	lastSent time.Time
}

func NewClient(httpClient *httputil.Client, cfg config.QQConfig) *Client {
	if cfg.GroupMap == nil {
		cfg.GroupMap = map[string]string{}
	} else {
		cfg.GroupMap = cloneGroupMap(cfg.GroupMap)
	}
	return &Client{http: httpClient, config: cfg}
}

func (c *Client) SetGroupID(serverCode, groupID string) {
	serverCode = strings.TrimSpace(serverCode)
	groupID = strings.TrimSpace(groupID)
	if serverCode == "" || groupID == "" {
		return
	}
	c.mapMu.Lock()
	defer c.mapMu.Unlock()
	c.config.GroupMap[serverCode] = groupID
}

func (c *Client) RemoveGroupID(serverCode, groupID string) {
	serverCode = strings.TrimSpace(serverCode)
	groupID = strings.TrimSpace(groupID)
	if serverCode == "" || groupID == "" {
		return
	}
	c.mapMu.Lock()
	defer c.mapMu.Unlock()
	if c.config.GroupMap[serverCode] == groupID {
		delete(c.config.GroupMap, serverCode)
	}
}

func (c *Client) Send(ctx context.Context, serverCode, summary, content string) error {
	apiURL := strings.TrimRight(strings.TrimSpace(c.config.APIURL), "/")
	if apiURL == "" {
		return errors.New("qq.api_url or QQ_BOT_API_URL is required")
	}
	endpoint, idKey, id, err := c.target(serverCode)
	if err != nil {
		return err
	}

	message := strings.TrimSpace(summary)
	if content != "" {
		if message != "" {
			message += "\n"
		}
		message += content
	}
	payload := map[string]interface{}{
		idKey:     id,
		"message": message,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	release, err := c.reserveSend(ctx)
	if err != nil {
		return err
	}
	defer release()

	headers := map[string]string{}
	if token := strings.TrimSpace(c.config.AccessToken); token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	resp, err := c.http.Post(ctx, apiURL+endpoint, "application/json", body, headers)
	if err != nil {
		return err
	}
	var result struct {
		Status  string `json:"status"`
		Retcode int    `json:"retcode"`
		Msg     string `json:"msg"`
		Wording string `json:"wording"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return fmt.Errorf("qq response decode failed: %w body=%s", err, trimBody(resp.Body))
	}
	if result.Retcode != 0 || (result.Status != "" && strings.ToLower(result.Status) != "ok") {
		return fmt.Errorf("qq push failed: status=%s retcode=%d msg=%s wording=%s", result.Status, result.Retcode, result.Msg, result.Wording)
	}
	return nil
}

func (c *Client) reserveSend(ctx context.Context) (func(), error) {
	c.sendMu.Lock()
	interval := c.config.MinInterval.Duration
	if interval > 0 && !c.lastSent.IsZero() {
		wait := time.Until(c.lastSent.Add(interval))
		if wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				c.sendMu.Unlock()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
	}
	c.lastSent = time.Now()
	return c.sendMu.Unlock, nil
}

func (c *Client) target(serverCode string) (string, string, int64, error) {
	targetType := strings.ToLower(strings.TrimSpace(c.config.TargetType))
	if targetType == "" {
		targetType = "group"
	}
	endpoint := "/send_group_msg"
	idKey := "group_id"
	targetID := ""

	switch targetType {
	case "group":
		targetID = c.groupID(serverCode)
	case "private", "user", "friend":
		endpoint = "/send_private_msg"
		idKey = "user_id"
		targetID = firstNonEmpty(os.Getenv("QQ_USER_ID"), c.config.DefaultID)
	default:
		return "", "", 0, fmt.Errorf("unsupported qq target type %q", targetType)
	}
	if targetID == "" {
		return "", "", 0, fmt.Errorf("qq target id is required for server %s", serverCode)
	}
	id, err := strconv.ParseInt(targetID, 10, 64)
	if err != nil {
		return "", "", 0, fmt.Errorf("invalid qq target id %q: %w", targetID, err)
	}
	return endpoint, idKey, id, nil
}

func (c *Client) groupID(serverCode string) string {
	if id := strings.TrimSpace(os.Getenv("QQ_GROUP_ID_" + envServerCode(serverCode))); id != "" {
		return id
	}
	c.mapMu.RLock()
	id := strings.TrimSpace(c.config.GroupMap[serverCode])
	c.mapMu.RUnlock()
	if id != "" {
		return id
	}
	if id := c.groupIDFromFile(serverCode); id != "" {
		return id
	}
	return firstNonEmpty(os.Getenv("QQ_TARGET_ID"), os.Getenv("QQ_GROUP_ID"), c.config.DefaultID)
}

func (c *Client) groupIDFromFile(serverCode string) string {
	if strings.TrimSpace(c.config.GroupMapFile) == "" {
		return ""
	}
	raw, err := os.ReadFile(c.config.GroupMapFile)
	if err != nil {
		return ""
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	var groups map[string]interface{}
	if err := decoder.Decode(&groups); err != nil {
		return ""
	}
	for key, value := range groups {
		if !strings.EqualFold(key, serverCode) {
			continue
		}
		switch v := value.(type) {
		case string:
			return strings.TrimSpace(v)
		case json.Number:
			return v.String()
		case float64:
			return strconv.FormatInt(int64(v), 10)
		}
	}
	return ""
}

func envServerCode(serverCode string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(serverCode) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	return b.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func trimBody(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 300 {
		return text[:300]
	}
	return text
}

func cloneGroupMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
