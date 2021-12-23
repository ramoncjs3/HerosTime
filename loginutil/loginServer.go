/**
* @Author: Ramoncjs
* @Date: 2021/8/20 21:03
 */
package loginutil

import (
	Util "HerosTime/utils"
	"errors"
	"fmt"
	"github.com/Luzifer/go-openssl/v4"
	"github.com/bitly/go-simplejson"
	"log"
	"net/http"
	"net/url"
	"time"
)

//登陆身份认证信息
type Login struct {
	Username      string
	Password      string
	Loginid       string
	Token         string
	RoleID        float64
	LoginFlag     float64
	QuickLoginUrl string
	GameUrl       string
	ServerCode    string
	IsOver        bool
	Sign          string
	DisName       string
}

// ReqLogin 登陆认证第一步，返回Pauth
func ReqLogin() (string, error) {
	var user = "4399madhero"
	var pass = "1qaz@WSX"
	var key = "lzYW5qaXVqa"
	encryptPass, err := openssl.New().EncryptBytes(key, []byte(pass), openssl.BytesToKeyMD5)
	if err != nil {
		return "", err
	}
	reqUrl := `http://ptlogin.4399.com/ptlogin/login.do`
	reqBody := fmt.Sprintf("username=%s&password=%s&sec=1", user, url.QueryEscape(string(encryptPass)))
	resp, err := Util.ReqPostData(reqUrl, reqBody)
	if err != nil {
		return "", err
	}
	Pauth := getCookieByName(resp.Response().Cookies(), "Pauth")
	return Pauth, nil
}

//登陆认证第二步，返回loginID,Name,SIGN
func Grant2(Pauth string) (string, string, string, error) {
	reqUrl := `http://h.api.4399.com/intermodal/user/grant2`
	reqBody := fmt.Sprintf(`gameId=100053785&authType=cookie&cookieValue=%s`, url.QueryEscape(Pauth))
	resp, err := Util.ReqPostData(reqUrl, reqBody)
	if err != nil {
		return "", "", "", err
	}
	respj, err := simplejson.NewJson([]byte(resp.String()))
	if err != nil {
		return "", "", "", err
	}
	if usr, ok := respj.Get("data").CheckGet("game"); ok {
		gameUrl, _ := usr.Get("gameUrl").String()
		aa, _ := url.Parse(gameUrl)
		return aa.Query()["userId"][0], aa.Query()["account"][0], aa.Query()["sign"][0], nil
	}
	return "", "", "", errors.New("Grant2获取loginID，Name，sign失败")

}

//登陆认证第三步，返回loginFlag，roleID
func QuickLogin(l *Login) (float64, float64, error) {
	nonce := fmt.Sprintf("%d"+"%s", time.Now().UnixNano()/1e6, Util.RandStr(8))
	originBodys := fmt.Sprintf(`{"mod":"User","do":"quicklogin","p":{"account":"%s","pwd":"123456","checkObj":{"userId":"%s","userName":"%s","time":"%s","sign":"%s","gameId":"100053785"},"channel":"4399","flag":1,"macAdress":"","platform":5,"web":true,"clientVersion":{"android":"2.3.5.1629810643786"},"NeedUpdateVersion":"","inGameTime":0,"roleID":0,"userAccount":"%s","loginFlag":"0","nonce":"%s"}}`, l.Loginid, l.Loginid, l.DisName, fmt.Sprintf("%d", time.Now().Unix()), l.Sign, l.Loginid, nonce)
	reqBodys := Util.EncryptAES(Util.CompressToBase64(originBodys))
	resp, err := Util.ReqPostData(l.QuickLoginUrl+"/quicklogin", reqBodys)
	if err != nil {
		return 0, 0, err
	}
	respj, err := simplejson.NewJson([]byte(resp.String()))
	if err != nil {
		return 0, 0, err
	}
	if code, err := respj.Get("code").Int(); code != 200 || err != nil {
		log.Println("[-] ", code)
		return 0, 0, errors.New("快速登陆失败.")
	}
	if data, ok := respj.CheckGet("MsgData"); ok {
		MsgData, _ := data.String()
		MsgDataDE, _ := Util.DecompressFromBase64(Util.DecryptAES(MsgData))
		ridfg, _ := simplejson.NewJson([]byte(MsgDataDE))
		if rid, ok := ridfg.CheckGet("roleID"); ok {
			if lfg, ok := ridfg.CheckGet("loginFlag"); ok {
				roleID, _ := rid.Float64()
				loginFlag, _ := lfg.Float64()
				return roleID, loginFlag, nil
			}
		}
	}
	return 0, 0, errors.New("登陆MsgData值解密获取失败.")
}

func getCookieByName(cookie []*http.Cookie, name string) string {
	cookieLen := len(cookie)
	result := ""
	for i := 0; i < cookieLen; i++ {
		if cookie[i].Name == name {
			result = cookie[i].Value
		}
	}
	return result
}
