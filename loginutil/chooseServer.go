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

		case "官方1区":
			rt["g1"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}
		case "官方2区":
			rt["g2"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}
		case "官方3区":
			rt["g3"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}
		case "混服1区":
			rt["h1"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}
		case "混服2区":
			rt["h2"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}
		case "混服3区":
			rt["h3"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}
		case "混服4区":
			rt["h4"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}
		case "混服5区":
			rt["h5"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}
		case "混服6区":
			rt["h6"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}
		case "混服7区":
			rt["h7"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}
		case "混服8区":
			rt["h8"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}
		case "混服9区":
			rt["h9"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}
		case "混服10区":
			rt["h10"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}
		case "混服11区":
			rt["h11"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}
		case "混服12区":
			rt["h12"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}
		case "混服13区":
			rt["h13"] = urlStruct{QuickLoginUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[2].(string)), GameUrl: fmt.Sprintf("%s:%s", bsvrlst[k][0], v.([]interface{})[3].(string))}

		default:
			return "", "", errors.New("[-] 区服代码错误.")
		}
	}
	return rt[fmt.Sprintf("%s", servercode)].QuickLoginUrl, rt[fmt.Sprintf("%s", servercode)].GameUrl, nil

}
