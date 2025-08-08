package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/argon2"
)

var ErrMasterPasswordNotSet = errors.New("MASTER_PASSWORD not set")

const defaultSaltFile = "./data/master_salt"

// argon2Params returns (time, memoryKB, threads)
func argon2Params() (uint32, uint32, uint8) {
	// defaults: time=1, memory=32MB, threads=2
	t := uint32(1)
	m := uint32(32 * 1024)
	th := uint8(2)
	if v := os.Getenv("ARGON2_TIME"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			t = uint32(n)
		}
	}
	if v := os.Getenv("ARGON2_MEMORY_KB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			m = uint32(n)
		}
	}
	if v := os.Getenv("ARGON2_THREADS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			th = uint8(n)
		}
	}
	return t, m, th
}

var sqlDB *sqlx.DB

// readOrCreateSalt reads the salt from the configured DB (if SetDB was called)
// and falls back to a local file if no DB is configured.
func readOrCreateSalt() ([]byte, error) {
	if sqlDB != nil {
		var val string
		q := sqlDB.Rebind("SELECT value FROM secrets WHERE name = ? LIMIT 1")
		err := sqlDB.Get(&val, q, "master_salt")
		if err == nil {
			s, err := base64.StdEncoding.DecodeString(strings.TrimSpace(val))
			if err != nil {
				return nil, err
			}
			return s, nil
		}
		// Not found, create and store
		s := make([]byte, 16)
		if _, err := rand.Read(s); err != nil {
			return nil, err
		}
		enc := base64.StdEncoding.EncodeToString(s)
		_, err = sqlDB.NamedExec("INSERT INTO secrets (name, value) VALUES (:name, :value)", map[string]interface{}{"name": "master_salt", "value": enc})
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(os.Stderr, "INFO: generated master salt and stored in DB (secrets.name=master_salt); keep DB backups\n")
		return s, nil
	}

	// Fallback to file-based salt for compatibility
	path := os.Getenv("MASTER_SALT_FILE")
	if path == "" {
		path = defaultSaltFile
	}
	if data, err := os.ReadFile(path); err == nil {
		s, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
		if err != nil {
			return nil, err
		}
		return s, nil
	}
	// create parent dir
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	s := make([]byte, 16)
	if _, err := rand.Read(s); err != nil {
		return nil, err
	}
	enc := base64.StdEncoding.EncodeToString(s)
	if err := os.WriteFile(path, []byte(enc+"\n"), 0o600); err != nil {
		return nil, err
	}
	fmt.Fprintf(os.Stderr, "INFO: generated master salt and wrote to %s; keep this file safe\n", path)
	return s, nil
}

// SetDB provides the database connection the crypto package should use for
// persisting and reading the master salt. Call this after applying migrations.
func SetDB(db *sqlx.DB) {
	sqlDB = db
}

func deriveKey(password string, salt []byte) []byte {
	t, m, th := argon2Params()
	return argon2.IDKey([]byte(password), salt, t, m, th, 32)
}

// VerifyMasterPassword derives a key from the candidate password using the
// persisted salt and compares it to the derived key from the configured
// MASTER_PASSWORD. Returns true when they match.
func VerifyMasterPassword(candidate string) (bool, error) {
	if os.Getenv("MASTER_PASSWORD") == "" {
		return false, ErrMasterPasswordNotSet
	}
	salt, err := readOrCreateSalt()
	if err != nil {
		return false, err
	}
	derivedCandidate := deriveKey(candidate, salt)
	derivedReal := deriveKey(os.Getenv("MASTER_PASSWORD"), salt)
	if hmac.Equal(derivedCandidate, derivedReal) {
		return true, nil
	}
	return false, nil
}

// masterKey derives the 32-byte master key from MASTER_PASSWORD and persisted salt.
func masterKey() ([]byte, error) {
	pass := os.Getenv("MASTER_PASSWORD")
	if pass == "" {
		return nil, ErrMasterPasswordNotSet
	}
	salt, err := readOrCreateSalt()
	if err != nil {
		return nil, err
	}
	return deriveKey(pass, salt), nil
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
