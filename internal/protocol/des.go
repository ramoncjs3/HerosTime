package protocol

import (
	"bytes"
	"crypto/des"
	"encoding/base64"
	"fmt"
)

func DESEncrypt(plainText, key string) (string, error) {
	block, err := des.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}
	blockSize := block.BlockSize()
	src := pkcs5Pad([]byte(plainText), blockSize)
	out := make([]byte, len(src))
	for start := 0; start < len(src); start += blockSize {
		block.Encrypt(out[start:start+blockSize], src[start:start+blockSize])
	}
	return base64.StdEncoding.EncodeToString(out), nil
}

func DESDecrypt(cipherText, key string) (string, error) {
	src, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", err
	}
	block, err := des.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}
	blockSize := block.BlockSize()
	if len(src)%blockSize != 0 {
		return "", fmt.Errorf("DES input is not a multiple of block size")
	}
	out := make([]byte, len(src))
	for start := 0; start < len(src); start += blockSize {
		block.Decrypt(out[start:start+blockSize], src[start:start+blockSize])
	}
	unpadded, err := pkcs5Unpad(out)
	if err != nil {
		return "", err
	}
	return string(unpadded), nil
}

func pkcs5Pad(src []byte, blockSize int) []byte {
	padding := blockSize - len(src)%blockSize
	return append(src, bytes.Repeat([]byte{byte(padding)}, padding)...)
}

func pkcs5Unpad(src []byte) ([]byte, error) {
	if len(src) == 0 {
		return nil, fmt.Errorf("empty PKCS5 input")
	}
	padding := int(src[len(src)-1])
	if padding == 0 || padding > len(src) {
		return nil, fmt.Errorf("invalid PKCS5 padding")
	}
	return src[:len(src)-padding], nil
}
