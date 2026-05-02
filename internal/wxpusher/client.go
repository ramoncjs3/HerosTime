package wxpusher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"oldbeggar-refactor/internal/config"
	"oldbeggar-refactor/internal/httputil"
)

type Client struct {
	http   *httputil.Client
	config config.WxPusherConfig
}

func NewClient(httpClient *httputil.Client, cfg config.WxPusherConfig) *Client {
	if cfg.TopicMap == nil {
		cfg.TopicMap = map[string]int64{}
	}
	return &Client{http: httpClient, config: cfg}
}

func (c *Client) Enabled() bool {
	return c.config.Enabled
}

func (c *Client) Send(ctx context.Context, serverCode, summary, content string) error {
	if !c.Enabled() {
		return nil
	}
	appToken := strings.TrimSpace(c.config.AppToken)
	if appToken == "" {
		return errors.New("wxpusher.app_token or WXPUSHER_APP_TOKEN is required")
	}
	topicID := c.topicID(serverCode)
	if topicID <= 0 {
		return fmt.Errorf("wxpusher topic id is required for server %s", serverCode)
	}

	message := strings.TrimSpace(content)
	if message == "" {
		message = strings.TrimSpace(summary)
	}
	payload := sendRequest{
		AppToken:    appToken,
		Summary:     limitRunes(strings.TrimSpace(summary), 20),
		Content:     message,
		ContentType: c.contentType(),
		TopicIDs:    []int64{topicID},
		URL:         strings.TrimSpace(c.config.URL),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	endpoint := strings.TrimSpace(c.config.Endpoint)
	if endpoint == "" {
		endpoint = "https://wxpusher.zjiecode.com/api/send/message"
	}
	resp, err := c.http.Post(ctx, endpoint, "application/json", body, nil)
	if err != nil {
		return err
	}
	var result sendResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return fmt.Errorf("wxpusher response decode failed: %w body=%s", err, trimBody(resp.Body))
	}
	if result.Success != nil && !*result.Success {
		return fmt.Errorf("wxpusher push failed: code=%d msg=%s", result.Code, result.Text())
	}
	if result.Code != 0 && result.Code != 1000 {
		return fmt.Errorf("wxpusher push failed: code=%d msg=%s", result.Code, result.Text())
	}
	for _, item := range result.Data {
		if item.Code != 0 && item.Code != 1000 {
			return fmt.Errorf("wxpusher push failed for topic %d: code=%d status=%s msg=%s", topicID, item.Code, item.Status, result.Text())
		}
	}
	return nil
}

type sendRequest struct {
	AppToken    string  `json:"appToken"`
	Content     string  `json:"content"`
	Summary     string  `json:"summary,omitempty"`
	ContentType int     `json:"contentType"`
	TopicIDs    []int64 `json:"topicIds"`
	URL         string  `json:"url,omitempty"`
}

type sendResponse struct {
	Code            int              `json:"code"`
	Msg             string           `json:"msg"`
	ResponseMessage string           `json:"message"`
	Success         *bool            `json:"success"`
	Data            []sendResultItem `json:"data"`
}

func (r sendResponse) Text() string {
	if strings.TrimSpace(r.Msg) != "" {
		return strings.TrimSpace(r.Msg)
	}
	return strings.TrimSpace(r.ResponseMessage)
}

type sendResultItem struct {
	Code   int    `json:"code"`
	Status string `json:"status"`
}

func (c *Client) contentType() int {
	if c.config.ContentType == 0 {
		return 1
	}
	return c.config.ContentType
}

func (c *Client) topicID(serverCode string) int64 {
	if id := strings.TrimSpace(os.Getenv("WXPUSHER_TOPIC_ID_" + envServerCode(serverCode))); id != "" {
		return parseTopicID(id)
	}
	if id := c.config.TopicMap[serverCode]; id > 0 {
		return id
	}
	if id := c.topicIDFromFile(serverCode); id > 0 {
		return id
	}
	return 0
}

func (c *Client) topicIDFromFile(serverCode string) int64 {
	if strings.TrimSpace(c.config.TopicMapFile) == "" {
		return 0
	}
	raw, err := os.ReadFile(c.config.TopicMapFile)
	if err != nil {
		return 0
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	var topics map[string]interface{}
	if err := decoder.Decode(&topics); err != nil {
		return 0
	}
	for key, value := range topics {
		if !strings.EqualFold(key, serverCode) {
			continue
		}
		switch v := value.(type) {
		case string:
			return parseTopicID(v)
		case json.Number:
			return parseTopicID(v.String())
		case float64:
			return int64(v)
		}
	}
	return 0
}

func parseTopicID(value string) int64 {
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}
	return id
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

func trimBody(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 300 {
		return text[:300]
	}
	return text
}

func limitRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
