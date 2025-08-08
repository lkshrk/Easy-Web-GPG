package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"

	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

// Master key should be provided as base64 in env MASTER_KEY
func masterKey() ([]byte, error) {
	b64 := os.Getenv("MASTER_KEY")
	if b64 == "" {
		return nil, errors.New("MASTER_KEY not set")
	}
	k, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	if len(k) != 32 {
		return nil, errors.New("MASTER_KEY must decode to 32 bytes (base64-encoded)")
	}
	return k, nil
}

// Encrypt plaintext and return base64(nonce|ciphertext)
func Encrypt(plaintext []byte) (string, error) {
	key, err := masterKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aesgcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := aesgcm.Seal(nil, nonce, plaintext, nil)
	out := append(nonce, ct...)
	return base64.StdEncoding.EncodeToString(out), nil
}

// Decrypt base64(nonce|ciphertext)
func Decrypt(b64 string) ([]byte, error) {
	key, err := masterKey()
	if err != nil {
		return nil, err
	}
	payload, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := aesgcm.NonceSize()
	if len(payload) < ns {
		return nil, errors.New("ciphertext too short")
	}
	nonce := payload[:ns]
	ct := payload[ns:]
	pt, err := aesgcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, err
	}
	return pt, nil
}

// CreateAuthCookieValue creates a signed timestamp token used for the auth cookie.
// The returned value is of the form "<ts>:<hex-hmac>" where HMAC is computed
// using the master key as secret. The token is valid when verified and the
// timestamp is within the allowed window.
func CreateAuthCookieValue() (string, error) {
	key, err := masterKey()
	if err != nil {
		return "", err
	}
	ts := time.Now().Unix()
	payload := strconv.FormatInt(ts, 10)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	sig := mac.Sum(nil)
	return payload + ":" + hex.EncodeToString(sig), nil
}

// VerifyAuthCookieValue verifies a cookie value created by CreateAuthCookieValue
// and checks that the timestamp is not older than maxAge (seconds).
func VerifyAuthCookieValue(val string, maxAgeSeconds int64) bool {
	key, err := masterKey()
	if err != nil {
		return false
	}
	parts := strings.SplitN(val, ":", 2)
	if len(parts) != 2 {
		return false
	}
	payload := parts[0]
	sigHex := parts[1]
	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	expected := mac.Sum(nil)
	if !hmac.Equal(expected, sig) {
		return false
	}
	ts, err := strconv.ParseInt(payload, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix()-ts > maxAgeSeconds {
		return false
	}
	return true
}
