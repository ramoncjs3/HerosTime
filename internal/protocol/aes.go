package protocol

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"fmt"
)

var (
	gameAESKey = []byte("120734aa834dea925ca8d2e1bb6811c3")
	gameAESIV  = []byte("33daee8409834c00")
)

func EncryptAES(plainText string) (string, error) {
	block, err := aes.NewCipher(gameAESKey)
	if err != nil {
		return "", err
	}
	padded := pkcs7Pad([]byte(plainText), block.BlockSize())
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, gameAESIV).CryptBlocks(out, padded)
	return hex.EncodeToString(out), nil
}

func DecryptAES(cipherHex string) (string, error) {
	cipherText, err := hex.DecodeString(cipherHex)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(gameAESKey)
	if err != nil {
		return "", err
	}
	if len(cipherText)%block.BlockSize() != 0 {
		return "", fmt.Errorf("ciphertext is not a multiple of block size")
	}
	out := make([]byte, len(cipherText))
	cipher.NewCBCDecrypter(block, gameAESIV).CryptBlocks(out, cipherText)
	unpadded, err := pkcs7Unpad(out, block.BlockSize())
	if err != nil {
		return "", err
	}
	return string(unpadded), nil
}

func pkcs7Pad(src []byte, blockSize int) []byte {
	padding := blockSize - len(src)%blockSize
	out := make([]byte, 0, len(src)+padding)
	out = append(out, src...)
	for i := 0; i < padding; i++ {
		out = append(out, byte(padding))
	}
	return out
}

func pkcs7Unpad(src []byte, blockSize int) ([]byte, error) {
	if len(src) == 0 || len(src)%blockSize != 0 {
		return nil, fmt.Errorf("invalid PKCS7 block")
	}
	padding := int(src[len(src)-1])
	if padding == 0 || padding > blockSize || padding > len(src) {
		return nil, fmt.Errorf("invalid PKCS7 padding")
	}
	for _, value := range src[len(src)-padding:] {
		if int(value) != padding {
			return nil, fmt.Errorf("invalid PKCS7 padding")
		}
	}
	return src[:len(src)-padding], nil
}
