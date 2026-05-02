package protocol

func EncodeGamePayload(jsonText string) (string, error) {
	return EncryptAES(CompressToBase64(jsonText))
}

func DecodeMsgData(msgData string) (string, error) {
	decrypted, err := DecryptAES(msgData)
	if err != nil {
		return "", err
	}
	return DecompressFromBase64(decrypted)
}
