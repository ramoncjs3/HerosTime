/**
* @Author: Ramoncjs
* @Date: 2021/8/20 21:03
 */
package loginutil

import (
	"errors"
	"fmt"
)

type urlStruct struct {
	QuickLoginUrl string
	GameUrl       string
}

type BZSRVLIST map[string][]string

var bsvrlst BZSRVLIST

var srv map[string]interface{}
var rt = make(map[string]urlStruct)

func ChooseServer(servercode string) (string, string, error) {
	for k, v := range srv {
		switch v.([]interface{})[5].(string) {

		case "h5暴走1区":
			rt["h5_1"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}

		case "h5暴走2区":
			rt["h5_2"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}

		case "h5暴走3区":
			rt["h5_3"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}

		case "h5暴走4区":
			rt["h5_4"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}

		case "h5暴走5区":
			rt["h5_5"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}

		case "h5暴走6区":
			rt["h5_6"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}

		case "h5暴走7区":
			rt["h5_7"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}

		case "h5暴走8区":
			rt["h5_8"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}

		default:
			return "", "", errors.New("[-] 区服代码错误.")
		}
	}
	return rt[fmt.Sprintf("%s", servercode)].QuickLoginUrl, rt[fmt.Sprintf("%s", servercode)].GameUrl, nil

}
