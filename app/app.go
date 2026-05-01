/**
* @Author: Ramoncjs
* @Date: 2021/8/20 21:00
 */
package app

import (
	"HerosTime/global"
	"HerosTime/loginutil"
	"bytes"
	"embed"
	"fmt"
	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

//go:embed config
var f embed.FS
var jobMu sync.Mutex

func init() {
	global.ConfigFile, _ = f.ReadFile("config/config.yaml")
	global.Item, _ = f.ReadFile("config/Item.json")
	global.ItemToName, _ = f.ReadFile("config/ItemToName.json")
}

func bootstrap() error {
	if err := D1(); err != nil {
		return err
	}
	if err := _No(); err != nil {
		return err
	}
	if time.Now().Hour() < 8 {
		return nil
	}
	if err := D30(); err != nil {
		log.Println(err)
	}
	return nil
}

func runExclusive(fn func() error) error {
	jobMu.Lock()
	defer jobMu.Unlock()
	return fn()
}

func rememberFirstErr(firstErr *error, err error) {
	if *firstErr == nil {
		*firstErr = err
	}
}

func D1() error {
	viper.SetConfigType("yaml")
	err := viper.ReadConfig(bytes.NewBuffer(global.ConfigFile))
	if err != nil { // 读取配置信息失败
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}

	global.WX_TOPIC_Initial()
	for i, v := range viper.GetStringMap("WX_Topic") {
		global.WX_TOPIC[i] = v.(int)
	}

	global.WX_APPTOKEN = viper.GetString("WX_APPTOKEN")

	global.LoginStructList = nil //重置登陆信息
	for i, _ := range viper.GetStringMap("Account") {
		a := &loginutil.Login{
			//Username:   v.(map[string]interface{})["username"].(string),
			//Password:   v.(map[string]interface{})["password"].(string),
			ServerCode: i,
		}
		global.LoginStructList = append(global.LoginStructList, a)
	}
	global.Pauth, err = loginutil.ReqLogin()
	if err != nil {
		return err
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
		v.Loginid, v.DisName, v.Sign, err = loginutil.Grant2(global.Pauth)
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
		aa := "[-] 登陆MsgData自检失败,v.RoleID, v.LoginFlag存在空值,1秒后重试."
		for true {
			if v.RoleID == float64(0) || v.LoginFlag == float64(0) {
				log.Println(aa, v.ServerCode)
				time.Sleep(time.Second)
				v.Loginid, v.DisName, v.Sign, err = loginutil.Grant2(global.Pauth)
				if err != nil {
					return err
				}
				v.RoleID, v.LoginFlag, err = loginutil.QuickLogin(v)
				if err != nil {
					return err
				}
			} else {
				log.Println(v.RoleID, v.LoginFlag, v.ServerCode)
				log.Println("[+] 8点MsgData自检完成.", v.ServerCode)
				break
			}

		}

	}
	return nil
}

func D30() error {
	var firstErr error
	for _, v := range global.LoginStructList {
		if v.IsOver {
			continue
		}
		ItemData, err := GetShopData(v)
		if err != nil {
			err = fmt.Errorf("server %s get shop data: %w", v.ServerCode, err)
			log.Println("[-]", err)
			rememberFirstErr(&firstErr, err)
			continue
		}
		if ItemData != nil {
			log.Println("[+] 老乞丐售卖物品:", ItemData, "当前区服:", v.ServerCode)
			content := strings.Join(ItemData[:], ",")
			if os.Getenv("DRY_RUN") == "1" {
				log.Println("[DRY_RUN] skip qq push:", "server:", v.ServerCode, "content:", content)
				v.IsOver = true
				continue
			}
			summary := fmt.Sprintf("HerosTime old beggar reminder-%s", v.ServerCode)
			err = SendQQMessage(v.ServerCode, summary, content)
			for retry := 1; err != nil && retry <= 3; retry++ {
				log.Println("[-] QQ push failed:", err, "retry:", retry)
				time.Sleep(2 * time.Second)
				err = SendQQMessage(v.ServerCode, summary, content)
			}
			log.Println("[+] QQ push status:", err)
			if err != nil {
				err = fmt.Errorf("server %s qq push: %w", v.ServerCode, err)
				log.Println("[-]", err)
				rememberFirstErr(&firstErr, err)
				continue
			}
			v.IsOver = true
		}
	}
	return firstErr
}

func SendSelfCheckMonitor() error {
	content := fmt.Sprintf("服务: h5\n状态: 自检完成\n时间: %s\n正式检测: 08:00 开始", time.Now().Format("2006-01-02 15:04:05"))
	if os.Getenv("DRY_RUN") == "1" {
		log.Println("[DRY_RUN] skip self-check qq push:", content)
		return nil
	}
	err := SendQQMessage("monitor", "老乞丐推送程序自检完成-h5", content)
	for retry := 1; err != nil && retry <= 3; retry++ {
		log.Println("[-] self-check QQ push failed:", err, "retry:", retry)
		time.Sleep(2 * time.Second)
		err = SendQQMessage("monitor", "老乞丐推送程序自检完成-h5", content)
	}
	log.Println("[+] self-check QQ push status:", err)
	if err != nil {
		return fmt.Errorf("self-check qq push: %w", err)
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
	if err := runExclusive(bootstrap); err != nil {
		log.Fatal(err)
	}

	c := cron.New(cron.WithSeconds(), cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)))
	_, _ = c.AddFunc("0 30 7 * * *", func() {
		err := runExclusive(func() error {
			if err := D1(); err != nil {
				return err
			}
			return SendSelfCheckMonitor()
		})
		if err != nil {
			log.Println(err)
		}
	})
	_, _ = c.AddFunc("30 0,30 8-23 * * *", func() {
		if err := runExclusive(D30); err != nil {
			log.Println(err)
		}
	})
	c.Start()
	log.Println("[+] Already Start! ")
	select {}
}
