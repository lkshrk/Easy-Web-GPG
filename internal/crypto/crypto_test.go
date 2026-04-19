package crypto_test

import (
	"testing"

	_ "github.com/glebarez/sqlite"
	"github.com/jmoiron/sqlx"

	c "h-cloud.io/web-gpg/internal/crypto"
	dbpkg "h-cloud.io/web-gpg/internal/db"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	t.Setenv("MASTER_PASSWORD", "test-master-password")
	t.Setenv("MASTER_SALT_FILE", t.TempDir()+"/master_salt")

	db, err := sqlx.Open("sqlite", "file::memory:?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := dbpkg.ApplySQLMigrations(db, "../../migrations/sql"); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	svc := c.NewCryptoService(db)

	plaintext := []byte("super secret")
	enc, err := svc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	dec, err := svc.Decrypt(enc)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(dec) != string(plaintext) {
		t.Fatalf("got %q want %q", string(dec), string(plaintext))
	}
}

func TestEncrypt_NoMasterPassword(t *testing.T) {
	t.Setenv("MASTER_PASSWORD", "")
	t.Setenv("MASTER_SALT_FILE", t.TempDir()+"/master_salt")

	db, err := sqlx.Open("sqlite", "file::memory:?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := dbpkg.ApplySQLMigrations(db, "../../migrations/sql"); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	svc := c.NewCryptoService(db)
	_, err = svc.Encrypt([]byte("test"))
	if err == nil {
		t.Fatal("expected error when MASTER_PASSWORD not set")
	}
}

func TestDecrypt_BadInput(t *testing.T) {
	t.Setenv("MASTER_PASSWORD", "test-master-password")
	t.Setenv("MASTER_SALT_FILE", t.TempDir()+"/master_salt")

	db, err := sqlx.Open("sqlite", "file::memory:?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := dbpkg.ApplySQLMigrations(db, "../../migrations/sql"); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	svc := c.NewCryptoService(db)

	// Invalid base64
	_, err = svc.Decrypt("not-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}

	// Valid base64 but too short for nonce
	_, err = svc.Decrypt("AQID")
	if err == nil {
		t.Fatal("expected error for short ciphertext")
	}
}

func TestVerifyMasterPassword(t *testing.T) {
	t.Setenv("MASTER_PASSWORD", "correct-password")
	t.Setenv("MASTER_SALT_FILE", t.TempDir()+"/master_salt")

	db, err := sqlx.Open("sqlite", "file::memory:?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := dbpkg.ApplySQLMigrations(db, "../../migrations/sql"); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	svc := c.NewCryptoService(db)

	ok, err := svc.VerifyMasterPassword("correct-password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("correct password should verify")
	}

	ok, err = svc.VerifyMasterPassword("wrong-password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("wrong password should not verify")
	}
}

func TestVerifyMasterPassword_NotSet(t *testing.T) {
	t.Setenv("MASTER_PASSWORD", "")

	db, err := sqlx.Open("sqlite", "file::memory:?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := dbpkg.ApplySQLMigrations(db, "../../migrations/sql"); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	svc := c.NewCryptoService(db)
	_, err = svc.VerifyMasterPassword("anything")
	if err == nil {
		t.Fatal("expected error when MASTER_PASSWORD not set")
	}
}

func TestVerifyAuthCookieValue_Expired(t *testing.T) {
	t.Setenv("MASTER_PASSWORD", "test-master-password")
	t.Setenv("MASTER_SALT_FILE", t.TempDir()+"/master_salt")

	db, err := sqlx.Open("sqlite", "file::memory:?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := dbpkg.ApplySQLMigrations(db, "../../migrations/sql"); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	svc := c.NewCryptoService(db)
	val, err := svc.CreateAuthCookieValue()
	if err != nil {
		t.Fatalf("create cookie: %v", err)
	}

	// maxAge of -1 means expired
	if svc.VerifyAuthCookieValue(val, -1) {
		t.Fatal("cookie with negative maxAge should be expired")
	}
}

func TestEncryptDecrypt_FileSaltFallback(t *testing.T) {
	t.Setenv("MASTER_PASSWORD", "file-salt-test")
	saltFile := t.TempDir() + "/test_salt"
	t.Setenv("MASTER_SALT_FILE", saltFile)

	// nil DB forces file-based salt
	svc := c.NewCryptoService(nil)

	plaintext := []byte("file-salt secret")
	enc, err := svc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	dec, err := svc.Decrypt(enc)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(dec) != string(plaintext) {
		t.Fatalf("got %q want %q", string(dec), string(plaintext))
	}

	// Second call should read existing salt file
	enc2, err := svc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt2: %v", err)
	}
	dec2, err := svc.Decrypt(enc2)
	if err != nil {
		t.Fatalf("decrypt2: %v", err)
	}
	if string(dec2) != string(plaintext) {
		t.Fatalf("got %q want %q", string(dec2), string(plaintext))
	}
}

func TestArgon2ParamsFromEnv(t *testing.T) {
	t.Setenv("MASTER_PASSWORD", "argon-test")
	t.Setenv("MASTER_SALT_FILE", t.TempDir()+"/master_salt")
	t.Setenv("ARGON2_TIME", "2")
	t.Setenv("ARGON2_MEMORY_KB", "16384")
	t.Setenv("ARGON2_THREADS", "4")

	// Just verify it doesn't crash with custom params
	svc := c.NewCryptoService(nil)
	enc, err := svc.Encrypt([]byte("test"))
	if err != nil {
		t.Fatalf("encrypt with custom argon2 params: %v", err)
	}
	dec, err := svc.Decrypt(enc)
	if err != nil {
		t.Fatalf("decrypt with custom argon2 params: %v", err)
	}
	if string(dec) != "test" {
		t.Fatalf("got %q want %q", string(dec), "test")
	}
}

func TestAuthCookieRoundtrip(t *testing.T) {
	t.Setenv("MASTER_PASSWORD", "test-master-password")
	t.Setenv("MASTER_SALT_FILE", t.TempDir()+"/master_salt")

	db, err := sqlx.Open("sqlite", "file::memory:?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := dbpkg.ApplySQLMigrations(db, "../../migrations/sql"); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	svc := c.NewCryptoService(db)

	val, err := svc.CreateAuthCookieValue()
	if err != nil {
		t.Fatalf("create cookie: %v", err)
	}
	if !svc.VerifyAuthCookieValue(val, 86400) {
		t.Fatal("valid cookie should verify")
	}
	if svc.VerifyAuthCookieValue("garbage:value", 86400) {
		t.Fatal("garbage cookie should not verify")
	}
}
