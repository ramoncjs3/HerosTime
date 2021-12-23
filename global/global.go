/**
* @Author: Ramoncjs
* @Date: 2021/8/20 21:00
 */
package global

import "HerosTime/loginutil"

var Pauth string

var LoginStructList = []*loginutil.Login{} //配置文件读取的账号信息
var WX_TOPIC map[string]int
var WX_APPTOKEN string

func WX_TOPIC_Initial() {
	WX_TOPIC = nil
	WX_TOPIC = make(map[string]int)
}

var ConfigFile []byte
var Item []byte
var ItemToName []byte
