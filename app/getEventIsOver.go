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

func ItoB(i int) bool { return i != 0 }

func SelectEventIsOver(l *loginutil.Login) (bool, error) {
	nonce := strconv.FormatInt(time.Now().Unix(), 13) + utils.RandStr(8)
	originBodys := fmt.Sprintf(`{"mod":"User","do":"GetBigMapSpecialEvent","p":{"roleID":%s,"web":false,"clientVersion":{"android":"2.3.4.1622199442306"},"NeedUpdateVersion":"2.1.7","userAccount":"%s","loginFlag":"%s","nonce":"%s"}}`, strconv.FormatFloat(l.RoleID, 'f', -1, 64), l.Loginid, strconv.FormatFloat(l.LoginFlag, 'f', -1, 64), nonce)
	reqBodys := utils.EncryptAES(utils.CompressToBase64(originBodys))
	resp, err := utils.ReqPostData(l.GameUrl+"/getSpecialevts", reqBodys)
	if err != nil {
		return false, err
	}
	respj, err := simplejson.NewJson([]byte(resp.String()))
	if err != nil {
		return false, err
	}
	if _, ok := respj.CheckGet("code"); !ok {
		return false, errors.New("获取老乞丐来没来过状态失败.")
	} else if code, _ := respj.Get("code").Int(); code != 200 {
		log.Println("[-] ", code)
		return false, errors.New("获取老乞丐来没来过状态失败.")
	}
	if data, ok := respj.CheckGet("MsgData"); ok {
		MsgData, _ := data.String()
		MsgDataDE, _ := utils.DecompressFromBase64(utils.DecryptAES(MsgData))
		log.Println("[+] ", MsgDataDE)
		tmp, _ := simplejson.NewJson([]byte(MsgDataDE))
		if data, ok := tmp.Get("User.getSpecialevts").CheckGet("state"); ok {
			state, _ := data.Int()
			return ItoB(state), nil
		}
	}
	return false, errors.New("[-] 老乞丐来没来过MsgData值解密获取失败.")
}
