package crypto_test

import (
	"os"
	"testing"

	_ "github.com/glebarez/sqlite"
	"github.com/jmoiron/sqlx"

	c "h-cloud.io/web-gpg/internal/crypto"
	dbpkg "h-cloud.io/web-gpg/internal/db"
)

func TestEncryptDecrypt(t *testing.T) {
	// set MASTER_PASSWORD and in-memory sqlite DB for salt storage
	os.Setenv("MASTER_PASSWORD", "test-master-password")
	db, err := sqlx.Open("sqlite", "file::memory:?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := dbpkg.ApplySQLMigrations(db, "../../migrations/sql"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	c.SetDB(db)

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
