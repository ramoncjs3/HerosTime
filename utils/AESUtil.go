package utils

import (
	"encoding/hex"
	"github.com/zc2638/go-standard/src/crypto/aes/extra"
	"log"
)

//key,iv
var key = []byte("120734aa834dea925ca8d2e1bb6811c3")
var iv = []byte("33daee8409834c00")

//AES加密
func EncryptAES(origin1 string) string {
	origin := []byte(origin1)
	// 加密
	cipherText, err := extra.CBCEncrypt(origin, key, iv)
	if err != nil {
		log.Println(err)
	}
	// byte转hex字符串
	cipherTextStr := hex.EncodeToString(cipherText)
	return cipherTextStr
}

//AES解密
func DecryptAES(origin1 string) string {
	//hex字符串转byte
	cipherText, err := hex.DecodeString(origin1)
	if err != nil {
		log.Println(err)
	}
	// 解密
	originText, err := extra.CBCDecrypt(cipherText, key, iv)
	if err != nil {
		log.Println(err)
	}
	return string(originText)
}
