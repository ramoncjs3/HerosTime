package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func SendQQMessage(serverCode, summary, content string) error {
	apiURL := strings.TrimRight(strings.TrimSpace(os.Getenv("QQ_BOT_API_URL")), "/")
	if apiURL == "" {
		return errors.New("QQ_BOT_API_URL is required")
	}

	endpoint, idKey, id, err := qqTarget(serverCode)
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

	req, err := http.NewRequest(http.MethodPost, apiURL+endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token := strings.TrimSpace(os.Getenv("QQ_BOT_ACCESS_TOKEN")); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("qq push http status %d: %s", resp.StatusCode, trimBody(respBody))
	}

	var result struct {
		Status  string `json:"status"`
		Retcode int    `json:"retcode"`
		Msg     string `json:"msg"`
		Wording string `json:"wording"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("qq push response decode failed: %w body=%s", err, trimBody(respBody))
	}
	if result.Retcode != 0 || (result.Status != "" && strings.ToLower(result.Status) != "ok") {
		return fmt.Errorf("qq push failed: status=%s retcode=%d msg=%s wording=%s", result.Status, result.Retcode, result.Msg, result.Wording)
	}
	return nil
}

func qqTarget(serverCode string) (string, string, int64, error) {
	targetType := strings.ToLower(strings.TrimSpace(os.Getenv("QQ_TARGET_TYPE")))
	if targetType == "" {
		targetType = "group"
	}

	targetID := strings.TrimSpace(os.Getenv("QQ_TARGET_ID"))
	endpoint := "/send_group_msg"
	idKey := "group_id"

	switch targetType {
	case "group":
		targetID = qqGroupID(serverCode)
	case "private", "user", "friend":
		endpoint = "/send_private_msg"
		idKey = "user_id"
		if targetID == "" {
			targetID = strings.TrimSpace(os.Getenv("QQ_USER_ID"))
		}
	default:
		return "", "", 0, fmt.Errorf("unsupported QQ_TARGET_TYPE: %s", targetType)
	}

	if targetID == "" {
		if targetType == "group" {
			return "", "", 0, fmt.Errorf("QQ group id is required for server %s; set QQ_GROUP_ID_%s, QQ_GROUP_MAP, QQ_GROUP_MAP_FILE, or QQ_GROUP_ID", serverCode, envServerCode(serverCode))
		}
		return "", "", 0, errors.New("QQ target id is required")
	}
	id, err := strconv.ParseInt(targetID, 10, 64)
	if err != nil {
		return "", "", 0, fmt.Errorf("invalid QQ target id %q: %w", targetID, err)
	}
	return endpoint, idKey, id, nil
}

func qqGroupID(serverCode string) string {
	if id := strings.TrimSpace(os.Getenv("QQ_GROUP_ID_" + envServerCode(serverCode))); id != "" {
		return id
	}
	if id, err := qqGroupIDFromJSON(strings.TrimSpace(os.Getenv("QQ_GROUP_MAP")), serverCode); err == nil && id != "" {
		return id
	}
	if mapFile := strings.TrimSpace(os.Getenv("QQ_GROUP_MAP_FILE")); mapFile != "" {
		if raw, err := os.ReadFile(mapFile); err == nil {
			if id, err := qqGroupIDFromJSON(string(raw), serverCode); err == nil && id != "" {
				return id
			}
		}
	}
	if id := strings.TrimSpace(os.Getenv("QQ_TARGET_ID")); id != "" {
		return id
	}
	return strings.TrimSpace(os.Getenv("QQ_GROUP_ID"))
}

func qqGroupIDFromJSON(raw, serverCode string) (string, error) {
	if raw == "" {
		return "", nil
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()

	var groups map[string]interface{}
	if err := decoder.Decode(&groups); err != nil {
		return "", err
	}
	for key, value := range groups {
		if key != serverCode && !strings.EqualFold(key, serverCode) {
			continue
		}
		switch v := value.(type) {
		case string:
			return strings.TrimSpace(v), nil
		case json.Number:
			return v.String(), nil
		case float64:
			return strconv.FormatInt(int64(v), 10), nil
		default:
			return "", fmt.Errorf("unsupported QQ group id value for %s", serverCode)
		}
	}
	return "", nil
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
