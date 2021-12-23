package utils

import (
	"crypto/md5"
	"encoding/hex"
	"sort"
)

//生成Sign
func SignCheck(m map[string]string) string {
	var keys []string
	var result string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		result = result + m[k]
	}
	sign := md5.Sum([]byte(result + "d393805c9fea36662befb6282aa7331c"))
	return hex.EncodeToString(sign[:])
}
