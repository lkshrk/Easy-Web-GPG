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
