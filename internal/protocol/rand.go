package protocol

import (
	"crypto/rand"
	"math/big"
)

const letters = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandomString(n int) string {
	if n <= 0 {
		return ""
	}
	out := make([]byte, n)
	max := big.NewInt(int64(len(letters)))
	for i := range out {
		value, err := rand.Int(rand.Reader, max)
		if err != nil {
			out[i] = letters[i%len(letters)]
			continue
		}
		out[i] = letters[value.Int64()]
	}
	return string(out)
}
