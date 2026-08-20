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
