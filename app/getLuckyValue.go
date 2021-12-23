/**
* @Author: Ramoncjs
* @Date: 2021/8/20 21:09
 */
package app

import (
	"HerosTime/loginutil"
	"HerosTime/utils"
	"errors"
	"fmt"
	"github.com/bitly/go-simplejson"
	"log"
	"strconv"
	"time"
)

func GetLuckyValue(l *loginutil.Login) (float64, error) {
	nonce := strconv.FormatInt(time.Now().Unix(), 13) + utils.RandStr(8)
	originBodys := fmt.Sprintf(`{"mod":"User","do":"GetChatAddress","p":{"roleID":%s,"web":false,"clientVersion":{"android":"2.3.4.1622199442306"},"NeedUpdateVersion":"2.3.4","inGameTime":3530982,"userAccount":"%s","loginFlag":"%s","nonce":"%s"}}`, strconv.FormatFloat(l.RoleID, 'f', -1, 64), l.Loginid, strconv.FormatFloat(l.LoginFlag, 'f', -1, 64), nonce)
	reqBodys := utils.EncryptAES(utils.CompressToBase64(originBodys))
	resp, err := utils.ReqPostData(l.GameUrl+"/GetChatAddress", reqBodys)
	if err != nil {
		return 0, err
	}
	respj, err := simplejson.NewJson([]byte(resp.String()))
	if err != nil {
		return 0, err
	}
	if code, err := respj.Get("code").Int(); code != 200 || err != nil {
		log.Println("[-] ", code)
		return 0, errors.New("获取福缘值失败.")
	}
	if data, ok := respj.CheckGet("MsgData"); ok {
		MsgData, _ := data.String()
		MsgDataDE, _ := utils.DecompressFromBase64(utils.DecryptAES(MsgData))
		Luck, _ := simplejson.NewJson([]byte(MsgDataDE))
		if data, ok := Luck.CheckGet("Luck"); ok {
			luck, _ := data.Float64()
			return luck, nil
		}
	}
	return 0, errors.New("福缘MsgData值解密获取失败.")
}
