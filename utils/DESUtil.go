/**
* @Author: Ramoncjs
* @Date: 2021/9/1 14:46
 */
package utils

import (
	"bytes"
	"crypto/des"
	"encoding/base64"
	"errors"
)

func PKCS5Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

func PKCS5UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}

func DesEncrypt(originText, Key string) (string, error) {
	src := []byte(originText)
	key := []byte(Key)
	block, err := des.NewCipher(key)
	if err != nil {
		return "", err
	}
	bs := block.BlockSize()
	src = PKCS5Padding(src, bs)
	if len(src)%bs != 0 {
		return "", errors.New("Need a multiple of the blocksize")
	}
	out := make([]byte, len(src))
	dst := out
	for len(src) > 0 {
		block.Encrypt(dst, src[:bs])
		src = src[bs:]
		dst = dst[bs:]
	}

	return base64.StdEncoding.EncodeToString(out), nil
}

func DesDecrypt(originText, Key string) (string, error) {
	src, _ := base64.StdEncoding.DecodeString(originText)
	key := []byte(Key)
	block, err := des.NewCipher(key)
	if err != nil {
		return "", err
	}
	out := make([]byte, len(src))
	dst := out
	bs := block.BlockSize()
	if len(src)%bs != 0 {
		return "", errors.New("crypto/cipher: input not full blocks")
	}
	for len(src) > 0 {
		block.Decrypt(dst, src[:bs])
		src = src[bs:]
		dst = dst[bs:]
	}
	out = PKCS5UnPadding(out)
	return string(out), nil
}
