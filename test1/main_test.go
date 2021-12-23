/**
* @Author: Ramoncjs
* @Date: 2021/8/20 22:46
 */
package test1

import (
	"Milk/loginutil"
	"bytes"
	"encoding/json"
	"github.com/bitly/go-simplejson"
	"io/ioutil"
	"log"
	"testing"
)

func Test_ce(t *testing.T) {
	aa := `{
  "error" : 0,
  "p" : "User.GetSpecialShopData",
  "User.GetSpecialShopData" : {
    "npcID" : 10163,
    "shopData" : {
      "3143" : {
        "npc" : 10163,
        "payment" : 2,
        "probability" : 10,
        "limit" : 10,
        "price" : 1000,
        "sale" : 0,
        "endtime" : 0,
        "isserver" : 1,
        "ID" : 3143,
        "page" : 1
      },
      "3714" : {
        "npc" : 10163,
        "payment" : 2,
        "probability" : 50,
        "limit" : 0,
        "price" : 20,
        "sale" : 0,
        "endtime" : 0,
        "isserver" : 1,
        "ID" : 3714,
        "page" : 1
      },
      "3726" : {
        "npc" : 10163,
        "payment" : 2,
        "probability" : 20,
        "limit" : 10,
        "price" : 20,
        "sale" : 0,
        "endtime" : 0,
        "isserver" : 0,
        "ID" : 3726,
        "page" : 1
      },
      "3729" : {
        "npc" : 10163,
        "payment" : 2,
        "probability" : 20,
        "limit" : 10,
        "price" : 20,
        "sale" : 0,
        "endtime" : 0,
        "isserver" : 0,
        "ID" : 3729,
        "page" : 1
      },
      "4496" : {
        "npc" : 10163,
        "payment" : 2,
        "probability" : 10,
        "limit" : 1,
        "price" : 360,
        "sale" : 0,
        "endtime" : 0,
        "isserver" : 0,
        "ID" : 4496,
        "page" : 1
      }
    }
  }
}`
	f1, _ := ioutil.ReadFile("./config/Item.json")
	f2, _ := ioutil.ReadFile("./config/ItemToName.json")
	ItemJ, _ := simplejson.NewFromReader(bytes.NewReader(f1))
	ItemNmJ, _ := simplejson.NewFromReader(bytes.NewReader(f2))
	shopData, _ := simplejson.NewJson([]byte(aa))

	var OldManShopData = []string{}
	if data, ok := shopData.Get("User.GetSpecialShopData").CheckGet("shopData"); ok {
		xx, _ := data.Map()
		for i := range xx {
			if data, ok := ItemJ.CheckGet(i); ok {
				tt, err := data.Array()
				if err != nil {
					return
				}
				code := tt[7].(json.Number)
				if data, ok := ItemNmJ.CheckGet(string(code)); ok {
					mm, err := data.String()
					if err != nil {
						return
					}
					OldManShopData = append(OldManShopData, mm)
				}
			}
		}

	}
	t.Log(OldManShopData)
}

func Test_a(t *testing.T) {
	a, b, err := loginutil.ChooseServer("g1")
	if err != nil {
		return
	}
	log.Println(a, b)

	c, d, err := loginutil.ChooseServer("h6")
	if err != nil {
		return
	}
	log.Println(c, d)

}
