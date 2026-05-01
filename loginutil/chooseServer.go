package loginutil

import (
	"errors"
	"fmt"
	"strings"
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
	rt = make(map[string]urlStruct)
	for _, raw := range srv {
		entry, ok := raw.([]interface{})
		if !ok || len(entry) < 6 {
			continue
		}
		name, _ := entry[5].(string)
		code := ""
		if strings.HasPrefix(name, "官方") {
			if n := zoneNumber(name); n > 0 {
				code = fmt.Sprintf("g%d", n)
			}
		} else if strings.HasPrefix(name, "混服") {
			if n := zoneNumber(name); n > 0 {
				code = fmt.Sprintf("h%d", n)
			}
		}
		if code != "" {
			rt[code] = urlStruct{
				QuickLoginUrl: serverURL(entry, 2),
				GameUrl:       serverURL(entry, 3),
			}
		}
	}

	selected, ok := rt[servercode]
	if !ok || selected.QuickLoginUrl == "" || selected.GameUrl == "" {
		return "", "", errors.New("server code not found: " + servercode)
	}
	return selected.QuickLoginUrl, selected.GameUrl, nil
}

func serverURL(entry []interface{}, pathIndex int) string {
	if len(entry) <= pathIndex {
		return ""
	}
	base, _ := entry[1].(string)
	path, _ := entry[pathIndex].(string)
	if base == "" || path == "" {
		return ""
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

func zoneNumber(name string) int {
	n := 0
	for _, r := range name {
		if r >= '0' && r <= '9' {
			n = n*10 + int(r-'0')
		}
	}
	return n
}
