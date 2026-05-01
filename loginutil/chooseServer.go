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
		if !strings.HasPrefix(strings.ToLower(name), "h5") {
			continue
		}
		n := zoneNumber(name)
		if n <= 0 {
			continue
		}
		rt[fmt.Sprintf("h5_%d", n)] = urlStruct{
			QuickLoginUrl: serverURL(entry, 2),
			GameUrl:       serverURL(entry, 3),
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
	last := 0
	inDigits := false
	for _, r := range name {
		if r >= '0' && r <= '9' {
			if !inDigits {
				n = 0
				inDigits = true
			}
			n = n*10 + int(r-'0')
			continue
		}
		if inDigits {
			last = n
			inDigits = false
		}
	}
	if inDigits {
		last = n
	}
	return last
}
