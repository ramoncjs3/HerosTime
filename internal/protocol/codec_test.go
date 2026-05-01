package protocol

import "testing"

func TestGamePayloadRoundTrip(t *testing.T) {
	plain := `{"mod":"User","do":"quicklogin","p":{"roleID":1,"nonce":"abc123"}}`
	encoded, err := EncodeGamePayload(plain)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeMsgData(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded != plain {
		t.Fatalf("decoded mismatch\nwant: %s\n got: %s", plain, decoded)
	}
}

func TestDESEncryptRoundTrip(t *testing.T) {
	plain := `{"uid":"123","token":"abc"}`
	encoded, err := DESEncrypt(plain, "57493415")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	decoded, err := DESDecrypt(encoded, "57493415")
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if decoded != plain {
		t.Fatalf("decoded mismatch\nwant: %s\n got: %s", plain, decoded)
	}
}
