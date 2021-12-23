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

var srv map[string]interface{}
var rt = make(map[string]urlStruct)

type BZSRVLIST map[string][]string

var bsvrlst BZSRVLIST

func ChooseServer(servercode string) (string, string, error) {
	for k, v := range srv {
		switch v.([]interface{})[5].(string) {

		case "暴走1区":
			rt["b1"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}
		case "暴走2区":
			rt["b2"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}
		case "暴走3区":
			rt["b3"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}
		case "暴走4区":
			rt["b4"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}
		case "暴走5区":
			rt["b5"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}
		case "暴走6区":
			rt["b6"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}
		case "暴走7区":
			rt["b7"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}
		case "暴走8区":
			rt["b8"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}
		case "暴走9区":
			rt["b9"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}
		case "暴走10区":
			rt["b10"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}
		case "暴走11区":
			rt["b11"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}
		case "暴走12区":
			rt["b12"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}
		case "暴走13区":
			rt["b13"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}

		default:
			return "", "", errors.New("[-] 区服代码错误.")
		}
	}
	return rt[fmt.Sprintf("%s", servercode)].QuickLoginUrl, rt[fmt.Sprintf("%s", servercode)].GameUrl, nil

}
