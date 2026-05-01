package protocol

import (
	"crypto/md5"
	"encoding/hex"
	"sort"
)

func SignCheck(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	plain := ""
	for _, key := range keys {
		plain += values[key]
	}
	sum := md5.Sum([]byte(plain + "d393805c9fea36662befb6282aa7331c"))
	return hex.EncodeToString(sum[:])
}
