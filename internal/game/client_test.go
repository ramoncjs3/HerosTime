package game

import (
	"encoding/json"
	"testing"

	"oldbeggar-refactor/internal/auth"
)

func TestQuickLoginPayloadMiniUsesH5Contract(t *testing.T) {
	payload, err := quickLoginPayload(&Session{
		Kind: "mini",
		Credentials: auth.Credentials{
			LoginID:     "user-id",
			DisplayName: "user-name",
			Sign:        "sign",
		},
	})
	if err != nil {
		t.Fatalf("quickLoginPayload(mini): %v", err)
	}

	var envelope struct {
		Params struct {
			Channel  string `json:"channel"`
			Platform int    `json:"platform"`
			Web      bool   `json:"web"`
			CheckObj struct {
				GameID string `json:"gameId"`
			} `json:"checkObj"`
		} `json:"p"`
	}
	if err := json.Unmarshal([]byte(payload), &envelope); err != nil {
		t.Fatalf("decode mini quicklogin payload: %v", err)
	}
	if envelope.Params.Channel != "4399" || envelope.Params.Platform != 5 || !envelope.Params.Web || envelope.Params.CheckObj.GameID != "100053785" {
		t.Fatalf("mini quicklogin did not use h5 contract: %+v", envelope.Params)
	}
}

func TestQuickLoginPayloadBaozouUsesU8Identity(t *testing.T) {
	payload, err := quickLoginPayload(&Session{
		Kind: "baozou",
		Credentials: auth.Credentials{
			LoginID:     "4399-user-id",
			Token:       "u8-token",
			U8AppID:     "8029",
			U8UserID:    "u8-user-id",
			ChannelCode: "27",
			SDKUserID:   "4399-user-id",
		},
	})
	if err != nil {
		t.Fatalf("quickLoginPayload(baozou): %v", err)
	}
	var envelope struct {
		Params struct {
			Account  string `json:"account"`
			Channel  string `json:"channel"`
			CheckObj struct {
				App  string `json:"app"`
				SDK  string `json:"sdk"`
				UIN  string `json:"uin"`
				Sess string `json:"sess"`
			} `json:"checkObj"`
			Mac      string `json:"macAdress"`
			Platform int    `json:"platform"`
			UserAcct string `json:"userAccount"`
		} `json:"p"`
	}
	if err := json.Unmarshal([]byte(payload), &envelope); err != nil {
		t.Fatalf("decode baozou quicklogin payload: %v", err)
	}
	if envelope.Params.Account != "4399-user-id" || envelope.Params.Channel != "27" ||
		envelope.Params.CheckObj.App != "u8sdk" || envelope.Params.CheckObj.SDK != "u8-user-id" ||
		envelope.Params.CheckObj.UIN != "4399-user-id" || envelope.Params.CheckObj.Sess != "u8-token" ||
		envelope.Params.Mac != "8029" || envelope.Params.Platform != 8 || envelope.Params.UserAcct != "4399-user-id" {
		t.Fatalf("baozou quicklogin did not use U8 identity: %+v", envelope.Params)
	}
}
