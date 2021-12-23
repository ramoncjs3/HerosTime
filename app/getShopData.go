/**
* @Author: Ramoncjs
* @Date: 2021/8/20 21:11
 */
package app

import (
	"HerosTime/global"
	"HerosTime/loginutil"
	Util "HerosTime/utils"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bitly/go-simplejson"
	"log"
	"strconv"
	"time"
)

// GetShopData 获取老乞丐售卖物品
func GetShopData(l *loginutil.Login) ([]string, error) {
	var OldManShopData = []string{}
	ItemJ, _ := simplejson.NewFromReader(bytes.NewReader(global.Item))
	ItemNmJ, _ := simplejson.NewFromReader(bytes.NewReader(global.ItemToName))
	// 构造请求体
	nonce := strconv.FormatInt(time.Now().Unix(), 13) + Util.RandStr(8)
	originBodys := fmt.Sprintf(`{"mod":"User","do":"GetSpecialShopData","p":{"npcID":10163,"roleID":%s,"web":false,"clientVersion":{"android":"2.3.4.1622199442306"},"NeedUpdateVersion":"2.3.4","userAccount":"%s","loginFlag":"%s","nonce":"%s"}}`, strconv.FormatFloat(l.RoleID, 'f', -1, 64), l.Loginid, strconv.FormatFloat(l.LoginFlag, 'f', -1, 64), nonce)
	reqBodys := Util.EncryptAES(Util.CompressToBase64(originBodys))
	resp, err := Util.ReqPostData(l.GameUrl+"/GetSpecialShopData", reqBodys)
	if err != nil {
		return nil, err
	}
	respj, err := simplejson.NewJson([]byte(resp.String()))
	if err != nil {
		return nil, err
	}
	if _, ok := respj.CheckGet("code"); !ok {
		return nil, errors.New("获取老乞丐售卖物品失败.")
	} else if code, _ := respj.Get("code").Int(); code != 200 {
		log.Println("[-] ", code)
		return nil, errors.New("获取老乞丐售卖物品失败.")
	}
	if data, ok := respj.CheckGet("MsgData"); ok {
		MsgData, _ := data.String()
		MsgDataDE, _ := Util.DecompressFromBase64(Util.DecryptAES(MsgData))
		log.Println("[+] ", MsgDataDE)
		shopData, _ := simplejson.NewJson([]byte(MsgDataDE))
		//if data, ok := shopData.Get("User.GetSpecialShopData").CheckGet("shopData"); ok {
		//	xx, _ := data.Map()
		//	for i := range xx {
		//		for n, v := range ShopDataItem {
		//			if i == n {
		//				OldManShopData = append(OldManShopData, v)
		//			}
		//		}
		//	}
		//}
		if data, ok := shopData.Get("User.GetSpecialShopData").CheckGet("shopData"); ok {
			xx, _ := data.Map()
			for i := range xx {
				if data, ok := ItemJ.CheckGet(i); ok {
					tt, err := data.Array()
					if err != nil {
						return nil, err
					}
					code := tt[7].(json.Number)
					if data, ok := ItemNmJ.CheckGet(string(code)); ok {
						mm, err := data.String()
						if err != nil {
							return nil, err
						}
						OldManShopData = append(OldManShopData, mm)
					}
				}
			}
		} else {
			return nil, nil
		}
	}
	return OldManShopData, nil
}
