/**
* @Author: Ramoncjs
* @Date: 2021/8/20 22:46
 */
package test1

import (
	"HerosTime/global"
	"HerosTime/loginutil"
	"fmt"
	"github.com/spf13/viper"
	"log"
	"net/url"
	"testing"
)

func Test_ce2(t *testing.T) {
	aa := `https://bzhero.maple-game.com/BzHero/index-4399.html?account=4399madhero&gameId=100053785&nick=4399madhero&userId=3563646671&userName=4399madhero&time=1630144276&sign=4482fe1d1600c3fa793eb4cd895f852e&pc=0&device=pc&addiction=0`
	ab, _ := url.Parse(aa)
	log.Println(ab.Query()["userId"][0])
	log.Println(ab.Query()["account"][0])
	log.Println(ab.Query()["sign"][0])

}
func Test_q(t *testing.T) {
	err := loginutil.Get_bzJSON()
	if err != nil {
		return
	}
}

func Test_qa(t *testing.T) {
	viper.SetConfigFile("../config/config.yaml") // 指定配置文件
	viper.AddConfigPath("./")                    // 指定查找配置文件的路径
	err := viper.ReadInConfig()                  // 读取配置信息
	if err != nil {                              // 读取配置信息失败
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}

	//global.WX_TOPIC = nil //重置登陆信息
	global.WX_TOPIC_Initial()
	for i, v := range viper.GetStringMap("WX_Topic") {
		global.WX_TOPIC[i] = v.(int)
	}
	log.Println(global.WX_TOPIC)
	global.WX_APPTOKEN = viper.GetString("WX_APPTOKEN")
	log.Println(global.WX_APPTOKEN)

}
