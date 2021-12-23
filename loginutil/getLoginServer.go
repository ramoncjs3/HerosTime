package loginutil

/**
* @Author: Ramoncjs
* @Date: 2021/11/26 15:56
 */

import (
	"HerosTime/utils"
	"encoding/json"
	"fmt"
	"github.com/bitly/go-simplejson"
)

func GetServerList() error {
	err := Get_bzJSON()
	if err != nil {
		return err
	}

	reqBodys := "a515314766c66a0146918898435cb2c08938a1cf3899c350cd905566983202334bea7b42c11ddb6b32cf21a1e61ec92ce74011509d3e126e12091d5f8590ce8c9d9e475c6ad6a014e3e04da25a2e82049f8c6378ecdac4950025843ee7a5dfd0"
	resp, err := utils.ReqPostData(fmt.Sprintf("%s:9898/GetServerList", bsvrlst["0"][0]), reqBodys)
	if err != nil {
		return err
	}

	respj, err := simplejson.NewJson([]byte(resp.String()))
	if err != nil {
		return err
	}

	if data, ok := respj.CheckGet("MsgData"); ok {
		MsgData, _ := data.String()
		MsgDataDE, _ := utils.DecompressFromBase64(utils.DecryptAES(MsgData))
		a, _ := simplejson.NewJson([]byte(MsgDataDE))
		aa, _ := a.CheckGet("serverObj")
		aaa, _ := aa.Map()
		srv = aaa
	}
	return nil
}

func Get_bzJSON() error {
	resp, err := utils.ReqGetData("https://bz.maple-game.com/h5.json")
	if err != nil {
		return err
	}

	err = json.Unmarshal([]byte(resp.String()), &bsvrlst)
	if err != nil {
		return err
	}

	return nil
}
