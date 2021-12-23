/**
* @Author: Ramoncjs
* @Date: 2021/8/20 21:03
 */
package loginutil

import (
	Util "HerosTime/utils"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/Luzifer/go-openssl/v4"
	"github.com/bitly/go-simplejson"
	"github.com/imroc/req"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

//登陆身份认证信息
type Login struct {
	RoleID        float64
	LoginFlag     float64
	Loginid       string
	Token         string
	QuickLoginUrl string
	GameUrl       string
	ServerCode    string
	IsOver        bool
}

// ReqLogin 登陆认证，返回带user_token,uid去与晓枫认证
func ReqLogin() (string, string, error) {
	var user = "4399madhero"
	var pass = "1qaz@WSX"
	var key = "lzYW5qaXVqa"
	var deskey = "57493415"
	var ul *url.URL
	var token string
	var uid string
	var resultSess string

	//req.Debug=true
	rq := req.New()
	rq.SetTimeout(30 * time.Second)
	rq.SetFlags(req.LstdFlags | req.Lcost)
	rq.Client().Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	encryptPass, err := openssl.New().EncryptBytes(key, []byte(pass), openssl.BytesToKeyMD5)
	if err != nil {
		return "", "", err
	}
	reqUrl := `https://m.4399api.com/openapiv2/oauth.html`
	param := req.Param{"device": "{\"DEVICE_IDENTIFIER\":\"\",\"SCREEN_RESOLUTION\":\"1170*1872\",\"DEVICE_MODEL\":\"Note10\",\"DEVICE_MODEL_VERSION\":\"6.0.1\",\"SYSTEM_VERSION\":\"6.0.1\",\"PLATFORM_TYPE\":\"Android\",\"SDK_VERSION\":\"2.37.0.211\",\"GAME_KEY\":\"116798\",\"GAME_VERSION\":\"2.3.3\",\"BID\":\"com.maple.madherogo.m4399\",\"IMSI\":\"\",\"PHONE\":\"\",\"RUNTIME\":\"Origin\",\"CANAL_IDENTIFIER\":\"\",\"UDID\":\"\",\"DEBUG\":\"false\",\"NETWORK_TYPE\":\"WIFI\",\"DEVICE_IDENTIFIER_SM\":\"20210827124041a3379d1da96eaf928ca0c54799eb58f701b8208b2b0f29c1\",\"SERVER_SERIAL\":\"0\",\"UID\":\"0000000000\"}"}
	resp1, err := rq.Post(reqUrl, param)
	if err != nil {
		return "", "", err
	}
	resp1j, _ := simplejson.NewJson([]byte(resp1.String()))
	if data, ok := resp1j.Get("result").CheckGet("login_url"); ok {
		aa, _ := data.String()
		ul, _ = url.Parse(aa)
	} else {
		return "", "", errors.New("[-] login_url获取失败.")
	}
	param2 := req.Param{
		"response_type": "TOKEN",
		"sec":           "1",
		"username":      user,
		"password":      string(encryptPass),
		"client_id":     ul.Query().Get("client_id"),
		"ref":           ul.Query().Get("ref"),
		"state":         ul.Query().Get("state"),
		"redirect_uri":  ul.Query().Get("redirect_uri"),
	}
	resp2, _ := rq.Post("https://ptlogin.4399.com/oauth2/loginAndAuthorize.do", param2)
	resp2j, _ := simplejson.NewJson([]byte(resp2.String()))
	if data, ok := resp2j.Get("result").CheckGet("state"); ok {
		token, _ = data.String()
	} else {
		return "", "", errors.New("[-] state/token获取失败.")
	}
	if data, ok := resp2j.Get("result").CheckGet("uid"); ok {
		m, _ := data.Int()

		uid = strconv.Itoa(m)
	} else {
		return "", "", errors.New("[-] uid获取失败.")
	}
	oriTextDes := fmt.Sprintf(`{
  "channel_code" : 27,
  "uid" : "%s",
  "token" : "%s",
  "user_name" : "%s"}`, uid, token, user)
	encryptJsonData, err := Util.DesEncrypt(oriTextDes, deskey)
	if err != nil {
		return "", "", err
	}
	param3 := req.Param{
		"json_data":    encryptJsonData,
		"product_code": "83313602112675691534121381984132",
	}
	resp3, _ := rq.Post("http://sdkapi03.quickapi.net/v2/users/checkLogin", param3)
	resp3j, _ := simplejson.NewJson([]byte(resp3.String()))
	if data, ok := resp3j.Get("data").CheckGet("user_token"); ok {
		resultSess, _ = data.String()
		return resultSess, uid, nil
	}
	return "", "", errors.New("[-] user_token 获取失败")
}

// QuickLogin 登陆认证第三步，返回roleID, loginFlag, nil
func QuickLogin(l *Login) (float64, float64, error) {
	nonce := fmt.Sprintf("%d"+"%s", time.Now().UnixNano()/1e6, Util.RandStr(8))
	originBodys := fmt.Sprintf(`{"mod":"User","do":"quicklogin","p":{"account":"%s","pwd":"123456","checkObj":{"app":"83313602112675691534121381984132","sdk":"27","uin":"%s","sess":"%s","newPack":1},"channel":"27","flag":1,"macAdress":"02:00:00:00:00:00","platform":"2","web":false,"clientVersion":{"android":"2.3.5.1629810643786"},"NeedUpdateVersion":"2.1.9","inGameTime":0,"roleID":0,"userAccount":"%s","loginFlag":"0","nonce":"%s"}}`, l.Loginid, l.Loginid, l.Token, l.Loginid, nonce)
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
