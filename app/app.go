/**
* @Author: Ramoncjs
* @Date: 2021/8/20 21:00
 */
package app

import (
	"HerosTime/global"
	"HerosTime/loginutil"
	"errors"
	"fmt"
	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
	"github.com/wxpusher/wxpusher-sdk-go"
	"github.com/wxpusher/wxpusher-sdk-go/model"
	"log"
	"strings"
	"time"
)

func init() {
	err := D1()
	if err != nil {
		log.Println(err)
		return
	}
	err = _No()
	if err != nil {
		log.Println(err)
		return
	}
	err = D30()
	if err != nil {
		log.Println(err)
		return
	}
}

func D1() error {
	viper.SetConfigFile("./config/config.yaml") // 指定配置文件
	viper.AddConfigPath("./")                   // 指定查找配置文件的路径
	err := viper.ReadInConfig()                 // 读取配置信息
	if err != nil {                             // 读取配置信息失败
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}

	global.WX_TOPIC_Initial()
	for i, v := range viper.GetStringMap("WX_Topic") {
		global.WX_TOPIC[i] = v.(int)
	}

	global.LoginStructList = nil //重置登陆信息
	for i, v := range viper.GetStringMap("Account") {
		a := &loginutil.Login{
			Username:   v.(map[string]interface{})["username"].(string),
			Password:   v.(map[string]interface{})["password"].(string),
			ServerCode: i,
		}
		global.LoginStructList = append(global.LoginStructList, a)
	}
	err = loginutil.GetServerList()
	if err != nil {
		return err
	}
	for _, v := range global.LoginStructList {
		v.QuickLoginUrl, v.GameUrl, err = loginutil.ChooseServer(v.ServerCode)
		if err != nil {
			return err
		}
		v.Loginid, v.Token, err = loginutil.ReqLogin(v)
		if err != nil {
			return err
		}
		v.RoleID, v.LoginFlag, err = loginutil.QuickLogin(v)
		//增加对8-22时间保护处理
		if err != nil {
			if err.Error() == "登陆MsgData值解密获取失败." {
				for true {
					log.Println("[-] 登陆MsgData值解密获取失败,2秒后重新尝试.")
					time.Sleep(2 * time.Second)
					v.RoleID, v.LoginFlag, err = loginutil.QuickLogin(v)
					if err.Error() != "登陆MsgData值解密获取失败." {
						break
					}
				}
			} else {
				return err
			}
		}
	}
	//登陆MsgData错误检查,应对8点init
	for _, v := range global.LoginStructList {
		aa := "[-] 登陆MsgData自检失败,v.RoleID, v.LoginFlag存在空值."
		if v.RoleID == float64(0) || v.LoginFlag == float64(0) {
			log.Println(aa, v.ServerCode)
			return errors.New(aa)
		} else {
			log.Println("[+] 8点MsgData自检完成.", v.ServerCode)
		}
	}
	return nil
}

func D30() error {
	var appToken = "AT_vYeuEDdRy41tuBxjfoN22MS1eudEMAF8"
	for _, v := range global.LoginStructList {
		if v.IsOver {
			continue
		}
		ItemData, err := GetShopData(v)
		if err != nil {
			return err
		}
		if ItemData != nil {
			log.Println("[+] 老乞丐售卖物品:", ItemData, "当前区服:", v.ServerCode)
			//微信提醒通道
			//appToken := "AT_vYeuEDdRy41tuBxjfoN22MS1eudEMAF8"
			msg := model.NewMessage(appToken)
			msg.Summary = fmt.Sprintf("老乞丐提醒-%s", v.ServerCode)
			msg.SetContent((strings.Join(ItemData[:], ","))).AddTopicId(WxTopicid(v.ServerCode))
			msgArr, err := wxpusher.SendMessage(msg)
			log.Println("[+] 微信通道状态:", msgArr, err)
			if err != nil {
				for true {
					log.Println("[-] 微信通道失败:", err, " 2秒后重新尝试发送.")
					time.Sleep(2 * time.Second)
					msgArr2, err2 := wxpusher.SendMessage(msg)
					log.Println("[+] 微信通道状态:", msgArr2)
					if err2 == nil {
						break
					}
				}
			}
			v.IsOver = true
		}
	}
	return nil
}

func WxTopicid(srvid string) int {
	return global.WX_TOPIC[srvid]
}

func _No() error {
	for _, v := range global.LoginStructList {
		IsOver, err := SelectEventIsOver(v)
		if err != nil {
			return err
		}
		if IsOver {
			log.Println("[+] 老乞丐今日已经来过.", "当前区服:", v.ServerCode)
			v.IsOver = true
		}
	}
	return nil
}

func Run() {
	c := cron.New(cron.WithSeconds())
	//早上查询福缘值
	_, _ = c.AddFunc("55 0 7 * * *", func() { //01 * * * *
		err := D1()
		if err != nil {
			log.Println(err)
		}
	})
	//半小时查询
	_, _ = c.AddFunc("30 0,30 * * * *", func() { //30 0,30 * * * *
		err := D30()
		if err != nil {
			log.Println(err)
		}
	})
	c.Start()
	log.Println("[+] Already Start! ")
	select {}
}
