package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/argon2"
)

// ErrMasterPasswordNotSet is returned when MASTER_PASSWORD env var is empty.
var ErrMasterPasswordNotSet = errors.New("MASTER_PASSWORD not set")

const defaultSaltFile = "./data/master_salt"

// CryptoService provides encryption, decryption, and authentication
// operations using a master password and a persisted salt.
type CryptoService struct {
	db *sqlx.DB
}

// NewCryptoService creates a CryptoService backed by the given database
// for salt persistence. The DB may be nil for file-based salt fallback.
func NewCryptoService(db *sqlx.DB) *CryptoService {
	return &CryptoService{db: db}
}

// argon2Params returns (time, memoryKB, threads) from env or defaults.
func argon2Params() (uint32, uint32, uint8) {
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

func deriveKey(password string, salt []byte) []byte {
	t, m, th := argon2Params()
	return argon2.IDKey([]byte(password), salt, t, m, th, 32)
}

// readOrCreateSalt reads the salt from the DB or falls back to a local file.
func (cs *CryptoService) readOrCreateSalt() ([]byte, error) {
	if cs.db != nil {
		var val string
		q := cs.db.Rebind("SELECT value FROM secrets WHERE name = ? LIMIT 1")
		err := cs.db.Get(&val, q, "master_salt")
		if err == nil {
			s, err := base64.StdEncoding.DecodeString(strings.TrimSpace(val))
			if err != nil {
				return nil, fmt.Errorf("failed to decode master_salt: %w", err)
			}
			return s, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("failed to read master_salt from DB: %w", err)
		}
		s := make([]byte, 16)
		if _, err := rand.Read(s); err != nil {
			return nil, err
		}
		enc := base64.StdEncoding.EncodeToString(s)
		_, err = cs.db.NamedExec("INSERT INTO secrets (name, value) VALUES (:name, :value)", map[string]interface{}{"name": "master_salt", "value": enc})
		if err != nil {
			return nil, fmt.Errorf("failed to insert master_salt: %w", err)
		}
		slog.Info("generated master salt and stored in DB", "secrets_key", "master_salt")
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

// masterKey derives the 32-byte master key from MASTER_PASSWORD and persisted salt.
func (cs *CryptoService) masterKey() ([]byte, error) {
	pass := os.Getenv("MASTER_PASSWORD")
	if pass == "" {
		return nil, ErrMasterPasswordNotSet
	}
	salt, err := cs.readOrCreateSalt()
	if err != nil {
		return nil, err
	}
	return deriveKey(pass, salt), nil
}

// VerifyMasterPassword compares a candidate password against MASTER_PASSWORD
// using argon2 key derivation with the persisted salt.
func (cs *CryptoService) VerifyMasterPassword(candidate string) (bool, error) {
	if os.Getenv("MASTER_PASSWORD") == "" {
		return false, ErrMasterPasswordNotSet
	}
	salt, err := cs.readOrCreateSalt()
	if err != nil {
		return false, err
	}
	derivedCandidate := deriveKey(candidate, salt)
	derivedReal := deriveKey(os.Getenv("MASTER_PASSWORD"), salt)
	return hmac.Equal(derivedCandidate, derivedReal), nil
}

// Encrypt encrypts plaintext and returns base64(nonce|ciphertext).
func (cs *CryptoService) Encrypt(plaintext []byte) (string, error) {
	key, err := cs.masterKey()
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

// Decrypt decodes base64(nonce|ciphertext) and returns plaintext.
func (cs *CryptoService) Decrypt(b64 string) ([]byte, error) {
	key, err := cs.masterKey()
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

// CreateAuthCookieValue creates a signed timestamp token for the auth cookie.
// Format: "<unix_ts>:<hex_hmac>" signed with the master key.
func (cs *CryptoService) CreateAuthCookieValue() (string, error) {
	key, err := cs.masterKey()
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

// VerifyAuthCookieValue verifies a cookie value and checks the timestamp
// is not older than maxAgeSeconds.
func (cs *CryptoService) VerifyAuthCookieValue(val string, maxAgeSeconds int64) bool {
	key, err := cs.masterKey()
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
	return time.Now().Unix()-ts <= maxAgeSeconds
}
