package game

import "oldbeggar-refactor/internal/auth"

type ServerEndpoint struct {
	Code          string
	QuickLoginURL string
	GameURL       string
	Name          string
}

type Session struct {
	VariantName   string
	Kind          string
	ServerCode    string
	Account       auth.Account
	Credentials   auth.Credentials
	QuickLoginURL string
	GameURL       string
	RoleID        float64
	LoginFlag     float64
}
