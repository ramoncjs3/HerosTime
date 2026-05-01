package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"oldbeggar-refactor/internal/httputil"
	"oldbeggar-refactor/internal/protocol"
)

type OfficialProvider struct {
	http *httputil.Client
}

func (p *OfficialProvider) Login(ctx context.Context, account Account) (Credentials, error) {
	if account.Username == "" || account.Password == "" {
		return Credentials{}, fmt.Errorf("official username and password are required")
	}

	values := map[string]string{
		"username":   account.Username,
		"password":   account.Password,
		"tfyuuid":    protocol.RandomString(16),
		"agent":      "app-10-02",
		"logintime":  fmt.Sprintf("%d", time.Now().Unix()),
		"gameid":     "10",
		"deviceType": "android",
		"appid":      "1001",
	}
	values["sign"] = protocol.SignCheck(values)

	form := url.Values{}
	for key, value := range values {
		form.Set(key, value)
	}

	resp, err := p.http.PostForm(ctx, "https://xfsdk.tfy-inc.com/app/login.php", form, nil)
	if err != nil {
		return Credentials{}, err
	}

	var result struct {
		LoginID string `json:"loginid"`
		Token   string `json:"token"`
		Msg     string `json:"msg"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return Credentials{}, err
	}
	if result.LoginID == "" || result.Token == "" {
		if result.Msg != "" {
			return Credentials{}, fmt.Errorf("official login failed: %s", result.Msg)
		}
		return Credentials{}, fmt.Errorf("official login returned empty loginid/token")
	}
	return Credentials{LoginID: result.LoginID, Token: result.Token}, nil
}
