package game

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"oldbeggar-refactor/internal/catalog"
	"oldbeggar-refactor/internal/httputil"
	"oldbeggar-refactor/internal/protocol"
)

type Client struct {
	http    *httputil.Client
	catalog *catalog.Catalog
}

const DefaultOldBeggarNPCID = 10163

var ErrEmptyQuickLogin = errors.New("quicklogin returned empty roleID/loginFlag")

func NewClient(httpClient *httputil.Client, itemCatalog *catalog.Catalog) *Client {
	return &Client{http: httpClient, catalog: itemCatalog}
}

func (c *Client) QuickLogin(ctx context.Context, session *Session) error {
	payload, err := quickLoginPayload(session)
	if err != nil {
		return err
	}
	decoded, err := c.postGame(ctx, session.QuickLoginURL+"/quicklogin", payload)
	if err != nil {
		return err
	}
	var result struct {
		RoleID    float64 `json:"roleID"`
		LoginFlag float64 `json:"loginFlag"`
	}
	if err := json.Unmarshal([]byte(decoded), &result); err != nil {
		return err
	}
	if result.RoleID == 0 || result.LoginFlag == 0 {
		return ErrEmptyQuickLogin
	}
	session.RoleID = result.RoleID
	session.LoginFlag = result.LoginFlag
	return nil
}

func (c *Client) EventIsOver(ctx context.Context, session *Session) (bool, error) {
	nonce := nonceForKind(session.Kind)
	payload := fmt.Sprintf(`{"mod":"User","do":"GetBigMapSpecialEvent","p":{"roleID":%s,"web":false,"clientVersion":{"android":"2.3.4.1622199442306"},"NeedUpdateVersion":"2.1.7","userAccount":"%s","loginFlag":"%s","nonce":"%s"}}`,
		floatString(session.RoleID),
		session.Credentials.LoginID,
		floatString(session.LoginFlag),
		nonce,
	)
	decoded, err := c.postGame(ctx, session.GameURL+"/getSpecialevts", payload)
	if err != nil {
		return false, err
	}
	var result struct {
		User struct {
			State int `json:"state"`
		} `json:"User.getSpecialevts"`
	}
	if err := json.Unmarshal([]byte(decoded), &result); err != nil {
		return false, err
	}
	return result.User.State != 0, nil
}

func (c *Client) ShopItems(ctx context.Context, session *Session) ([]string, error) {
	return c.SpecialShopItems(ctx, session, DefaultOldBeggarNPCID)
}

func (c *Client) SpecialShopItems(ctx context.Context, session *Session, npcID int) ([]string, error) {
	if npcID <= 0 {
		return nil, fmt.Errorf("invalid special shop npcID %d", npcID)
	}
	nonce := nonceForKind(session.Kind)
	payload := fmt.Sprintf(`{"mod":"User","do":"GetSpecialShopData","p":{"npcID":%d,"roleID":%s,"web":false,"clientVersion":{"android":"2.3.4.1622199442306"},"NeedUpdateVersion":"2.3.4","userAccount":"%s","loginFlag":"%s","nonce":"%s"}}`,
		npcID,
		floatString(session.RoleID),
		session.Credentials.LoginID,
		floatString(session.LoginFlag),
		nonce,
	)
	decoded, err := c.postGame(ctx, session.GameURL+"/GetSpecialShopData", payload)
	if err != nil {
		return nil, err
	}
	var result struct {
		User struct {
			ShopData map[string]interface{} `json:"shopData"`
		} `json:"User.GetSpecialShopData"`
	}
	if err := json.Unmarshal([]byte(decoded), &result); err != nil {
		return nil, err
	}
	if len(result.User.ShopData) == 0 {
		return nil, nil
	}
	items := make([]string, 0, len(result.User.ShopData))
	seen := make(map[string]struct{})
	for shopKey := range result.User.ShopData {
		name, ok := c.catalog.NameForShopKey(shopKey)
		if !ok || name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		items = append(items, name)
	}
	if len(items) == 0 {
		return nil, nil
	}
	return items, nil
}

func (c *Client) postGame(ctx context.Context, endpoint, jsonPayload string) (string, error) {
	encoded, err := protocol.EncodeGamePayload(jsonPayload)
	if err != nil {
		return "", err
	}
	resp, err := c.http.Post(ctx, endpoint, "application/x-www-form-urlencoded", []byte(encoded), nil)
	if err != nil {
		return "", err
	}
	var envelope struct {
		Code    int    `json:"code"`
		MsgData string `json:"MsgData"`
	}
	if err := json.Unmarshal(resp.Body, &envelope); err != nil {
		return "", err
	}
	if envelope.Code != 200 {
		return "", fmt.Errorf("game endpoint %s returned code=%d", endpoint, envelope.Code)
	}
	if envelope.MsgData == "" {
		return "", fmt.Errorf("game endpoint %s returned empty MsgData", endpoint)
	}
	return protocol.DecodeMsgData(envelope.MsgData)
}

func quickLoginPayload(session *Session) (string, error) {
	nonce := nonceForKind(session.Kind)
	switch session.Kind {
	case "official", "apple":
		return fmt.Sprintf(`{"mod":"User","do":"quicklogin","p":{"account":"%s","pwd":"123456","checkObj":{"app":"1","sdk":"meple100221","uin":"%s","sess":"%s","newPack":1},"channel":"meple100221","flag":1,"macAdress":"02:00:00:00:00:00","platform":"0","web":false,"clientVersion":{"android":"2.3.4.1628007977548"},"NeedUpdateVersion":"2.1.7","inGameTime":0,"roleID":0,"loginFlag":"1","nonce":"%s"}}`,
			session.Credentials.LoginID,
			session.Credentials.LoginID,
			session.Credentials.Token,
			nonce,
		), nil
	case "h5":
		return fmt.Sprintf(`{"mod":"User","do":"quicklogin","p":{"account":"%s","pwd":"123456","checkObj":{"userId":"%s","userName":"%s","time":"%d","sign":"%s","gameId":"100053785"},"channel":"4399","flag":1,"macAdress":"","platform":5,"web":true,"clientVersion":{"android":"2.3.5.1629810643786"},"NeedUpdateVersion":"","inGameTime":0,"roleID":0,"userAccount":"%s","loginFlag":"0","nonce":"%s"}}`,
			session.Credentials.LoginID,
			session.Credentials.LoginID,
			session.Credentials.DisplayName,
			time.Now().Unix(),
			session.Credentials.Sign,
			session.Credentials.LoginID,
			nonce,
		), nil
	case "baozou":
		return fmt.Sprintf(`{"mod":"User","do":"quicklogin","p":{"account":"%s","pwd":"123456","checkObj":{"app":"83313602112675691534121381984132","sdk":"27","uin":"%s","sess":"%s","newPack":1},"channel":"27","flag":1,"macAdress":"02:00:00:00:00:00","platform":"2","web":false,"clientVersion":{"android":"2.3.5.1629810643786"},"NeedUpdateVersion":"2.1.9","inGameTime":0,"roleID":0,"userAccount":"%s","loginFlag":"0","nonce":"%s"}}`,
			session.Credentials.LoginID,
			session.Credentials.LoginID,
			session.Credentials.Token,
			session.Credentials.LoginID,
			nonce,
		), nil
	default:
		return "", fmt.Errorf("unsupported quicklogin kind %q", session.Kind)
	}
}

func nonceForKind(kind string) string {
	if kind == "official" || kind == "apple" {
		return strconv.FormatInt(time.Now().Unix(), 13) + protocol.RandomString(8)
	}
	return fmt.Sprintf("%d%s", time.Now().UnixNano()/1e6, protocol.RandomString(8))
}

func floatString(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func MessageSummary(serverCode string) string {
	return fmt.Sprintf("HerosTime old beggar reminder-%s", serverCode)
}

func WatchMessageSummary(serverCode string, items []string) string {
	return fmt.Sprintf("HerosTime key item reminder-%s-%s", serverCode, MessageContent(items))
}

func MessageContent(items []string) string {
	return strings.Join(items, ",")
}
