package utils

import (
	"math/rand"
	"time"
)

//随机字符串生成
func RandStr(length int) string {
	str := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	bytes := []byte(str)
	result := []byte{}
	rand.Seed(time.Now().UnixNano() + int64(rand.Intn(100)))
	for i := 0; i < length; i++ {
		result = append(result, bytes[rand.Intn(len(bytes))])
	}
	return string(result)
}

//抖动等待时间
func RandTime() int {
	rand.Seed(time.Now().UnixNano() + int64(rand.Intn(100)))
	return rand.Intn(10) + 10
}
