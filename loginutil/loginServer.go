/**
* @Author: Ramoncjs
* @Date: 2021/8/20 21:03
 */
package loginutil

import (
	Util "HerosTime/utils"
	"errors"
	"fmt"
	"github.com/bitly/go-simplejson"
	"github.com/imroc/req"
	"log"
	"strconv"
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
}

//登陆认证第一步，返回loginid与token
func ReqLogin(l *Login) (string, string, error) {
	var inter map[string]interface{}
	reqUrl := "https://xfsdk.tfy-inc.com/app/login.php"
	m := make(map[string]string)
	m["username"] = l.Username
	m["password"] = l.Password
	m["tfyuuid"] = Util.RandStr(16)
	m["agent"] = "app-10-02"
	m["logintime"] = fmt.Sprintf("%d", time.Now().Unix())
	m["gameid"] = "10"
	m["deviceType"] = "android"
	m["appid"] = "1001"
	m["sign"] = Util.SignCheck(m)
	reqBodys := ""
	for i, j := range m {
		reqBodys = reqBodys + i
		reqBodys = reqBodys + "=" + j + "&"
	}
	var resp *req.Resp
	var err error
	for true {
		resp, err = Util.ReqPostData(reqUrl, reqBodys)
		if err != nil {
			time.Sleep(time.Second * 2)
			continue
		} else if err == nil {
			break
		}
	}
	err = resp.ToJSON(&inter)
	if err != nil {
		return "", "", err
	}
	log.Println("[*] 登陆状态:", inter["msg"], "当前区服:", l.ServerCode)
	if (inter["loginid"] == nil || inter["token"] == nil) && inter["msg"] != nil {
		return "", "", errors.New(inter["msg"].(string))
	}
	return inter["loginid"].(string), inter["token"].(string), nil
}

//登陆认证第二步，返回loginFlag，roleID，account
func QuickLogin(l *Login) (float64, float64, error) {
	nonce := strconv.FormatInt(time.Now().Unix(), 13) + Util.RandStr(8)
	originBodys := fmt.Sprintf(`{"mod":"User","do":"quicklogin","p":{"account":"%s","pwd":"123456","checkObj":{"app":"1","sdk":"meple100221","uin":"%s","sess":"%s","newPack":1},"channel":"meple100221","flag":1,"macAdress":"02:00:00:00:00:00","platform":"0","web":false,"clientVersion":{"android":"2.3.4.1628007977548"},"NeedUpdateVersion":"2.1.7","inGameTime":0,"roleID":0,"loginFlag":"1","nonce":"%s"}}`, l.Loginid, l.Loginid, l.Token, nonce)
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
