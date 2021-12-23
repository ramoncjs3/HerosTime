/**
* @Author: Ramoncjs
* @Date: 2021/8/20 22:46
 */
package test1

import (
	"HerosTime/global"
	"HerosTime/loginutil"
	"HerosTime/utils"
	"crypto/tls"
	"fmt"
	"github.com/Luzifer/go-openssl/v4"
	"github.com/bitly/go-simplejson"
	"github.com/imroc/req"
	"github.com/spf13/viper"
	"log"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func Test_ce2(t *testing.T) {
	//req.Debug = true
	rq := req.New()
	rq.SetTimeout(30 * time.Second)
	rq.SetFlags(req.LstdFlags | req.Lcost)
	rq.Client().Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	var url1 = `https://m.4399api.com/openapiv2/oauth.html`
	var url2 string
	var ul *url.URL
	var user = "4399madhero"
	var pass = "1qaz@WSX"
	var key = "lzYW5qaXVqa"
	//var final_state string
	var deskey = "57493415"
	var uid string
	var token string
	var result string
	encryptPass, _ := openssl.New().EncryptBytes(key, []byte(pass), openssl.BytesToKeyMD5)

	param := req.Param{"device": "{\"DEVICE_IDENTIFIER\":\"\",\"SCREEN_RESOLUTION\":\"1170*1872\",\"DEVICE_MODEL\":\"Note10\",\"DEVICE_MODEL_VERSION\":\"6.0.1\",\"SYSTEM_VERSION\":\"6.0.1\",\"PLATFORM_TYPE\":\"Android\",\"SDK_VERSION\":\"2.37.0.211\",\"GAME_KEY\":\"116798\",\"GAME_VERSION\":\"2.3.3\",\"BID\":\"com.maple.madherogo.m4399\",\"IMSI\":\"\",\"PHONE\":\"\",\"RUNTIME\":\"Origin\",\"CANAL_IDENTIFIER\":\"\",\"UDID\":\"\",\"DEBUG\":\"false\",\"NETWORK_TYPE\":\"WIFI\",\"DEVICE_IDENTIFIER_SM\":\"20210827124041a3379d1da96eaf928ca0c54799eb58f701b8208b2b0f29c1\",\"SERVER_SERIAL\":\"0\",\"UID\":\"0000000000\"}"}
	resp1, _ := rq.Post(url1, param)
	//fmt.Println(resp1.String())
	resp1j, _ := simplejson.NewJson([]byte(resp1.String()))
	if data, ok := resp1j.Get("result").CheckGet("login_url"); ok {
		url2, _ = data.String()
		ul, _ = url.Parse(url2)
	}
	if url2 != "" {
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
		}
		if data, ok := resp2j.Get("result").CheckGet("uid"); ok {
			m, _ := data.Float64()
			uid = fmt.Sprintf("%f", m)
		}
	}
	ori_text_des := fmt.Sprintf(`{
  "channel_code" : 27,
  "uid" : "%s",
  "token" : "%s",
  "user_name" : "%s"
}`, uid, token, user)
	encrypt_json_data, err := utils.DesEncrypt(ori_text_des, deskey)
	if err != nil {
		return
	}
	param3 := req.Param{
		"json_data":    encrypt_json_data,
		"product_code": "83313602112675691534121381984132",
	}
	resp3, _ := rq.Post("http://sdkapi03.quickapi.net/v2/users/checkLogin", param3)

	resp3j, _ := simplejson.NewJson([]byte(resp3.String()))
	if data, ok := resp3j.Get("data").CheckGet("user_token"); ok {
		result, _ = data.String()
		log.Println(result)
	}

}

func Test_aa(t *testing.T) {
	viper.SetConfigFile("../config/config.yaml") // 指定配置文件
	viper.AddConfigPath("./")                    // 指定查找配置文件的路径
	err := viper.ReadInConfig()                  // 读取配置信息
	if err != nil {                              // 读取配置信息失败
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
	global.LoginStructList = nil //重置登陆信息
	for i, _ := range viper.GetStringMap("Account") {
		a := &loginutil.Login{
			ServerCode: i,
		}
		global.LoginStructList = append(global.LoginStructList, a)
	}
	Token, Loginid, err := loginutil.ReqLogin()
	if err != nil {
		log.Println(err)
	}
	for _, v := range global.LoginStructList {
		v.QuickLoginUrl, v.GameUrl, err = loginutil.ChooseServer(v.ServerCode)
		if err != nil {
			log.Println(err)
			return
		}
		v.Token, v.Loginid = Token, Loginid
		if err != nil {
			log.Println(err)
			return
		}
		v.LoginFlag, v.RoleID, err = loginutil.QuickLogin(v)
		if err != nil {
			log.Println(err)
			return
		}
		log.Println(v.LoginFlag, v.RoleID)
	}

}
func Test_a(t *testing.T) {
	loginutil.GetServerList()

}
