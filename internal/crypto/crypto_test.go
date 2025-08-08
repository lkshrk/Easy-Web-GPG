package crypto_test

import (
	"encoding/base64"
	"os"
	"testing"

	c "h-cloud.io/web-gpg/internal/crypto"
)

func TestEncryptDecrypt(t *testing.T) {
	// set MASTER_KEY for test
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i + 1)
	}
	os.Setenv("MASTER_KEY", base64.StdEncoding.EncodeToString(raw))

	plaintext := []byte("super secret")
	enc, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}
	dec, err := c.Decrypt(enc)
	if err != nil {
		t.Fatalf("decrypt error: %v", err)
	}
	if string(dec) != string(plaintext) {
		t.Fatalf("got %q want %q", string(dec), string(plaintext))
	}
}
