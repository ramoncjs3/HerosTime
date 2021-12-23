package loginutil

/**
* @Author: Ramoncjs
* @Date: 2021/11/26 15:56
 */

import (
	"Milk/utils"
	"encoding/json"
	"fmt"
	"github.com/bitly/go-simplejson"
)

func GetServerList() error {
	err := Get_bzJSON()
	if err != nil {
		return err
	}

	reqBodys := "a515314766c66a0146918898435cb2c08938a1cf3899c350cd905566983202334bea7b42c11ddb6b32cf21a1e61ec92ce74011509d3e126e12091d5f8590ce8c98987160177e7d91ea8b118e2429484a6b6ca73e366b3bb7acbfb8cc2db804ca"
	resp, err := utils.ReqPostData(fmt.Sprintf("http://%s:9898/GetServerList", bsvrlst["0"][0]), reqBodys)
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
	resp, err := utils.ReqGetData("http://bz.maple-game.com/bz.json")
	if err != nil {
		return err
	}

	err = json.Unmarshal([]byte(resp.String()), &bsvrlst)
	if err != nil {
		return err
	}

	return nil
}
