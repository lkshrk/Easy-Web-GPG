package app_test

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	gcrypto "github.com/ProtonMail/gopenpgp/v2/crypto"
	_ "github.com/glebarez/sqlite"
	"github.com/jmoiron/sqlx"

	apppkg "h-cloud.io/web-gpg/internal/app"
	cm "h-cloud.io/web-gpg/internal/crypto"
	dbpkg "h-cloud.io/web-gpg/internal/db"
)

// setupTestApp creates a fully wired App with in-memory SQLite, migrations,
// and crypto service. It returns the App and the underlying DB for direct queries.
func setupTestApp(t *testing.T) (*apppkg.App, *sqlx.DB) {
	t.Helper()

	t.Setenv("MASTER_PASSWORD", "test-master-password")
	t.Setenv("MASTER_SALT_FILE", t.TempDir()+"/master_salt")

	db, err := sqlx.Open("sqlite", "file::memory:?mode=memory&cache=shared&_busy_timeout=5000")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := dbpkg.ApplySQLMigrations(db, "../../migrations/sql"); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	crypto := cm.NewCryptoService(db)

	files := []string{"../../templates/index.html", "../../templates/login.html"}
	tmpl, err := template.ParseFiles(files...)
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}

	a := &apppkg.App{DB: db, Templates: tmpl, Crypto: crypto}
	return a, db
}

// --- Integration / User Story Tests ---

// TestStory_AddKeyEncryptDecryptRoundtrip tests the full user journey:
// add a PGP key pair → encrypt a message → decrypt it back.
func TestStory_AddKeyEncryptDecryptRoundtrip(t *testing.T) {
	a, db := setupTestApp(t)

	// Generate a test key pair
	priv, err := gcrypto.GenerateKey("Test User", "test@example.com", "pass", 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	privArmored, _ := priv.Armor()
	pubArmored, _ := priv.GetArmoredPublicKey()

	// Step 1: Add public key via handler
	form := url.Values{}
	form.Set("name", "test-pub")
	form.Set("armored", pubArmored)
	req := httptest.NewRequest(http.MethodPost, "/keys", bytes.NewBufferString(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	a.AddKeyHandler(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("add public key: expected 303, got %d: %s", w.Code, w.Body.String())
	}

	// Step 2: Add private key with passphrase
	encPass, err := a.Crypto.Encrypt([]byte("pass"))
	if err != nil {
		t.Fatalf("encrypt passphrase: %v", err)
	}
	_, err = db.Exec("INSERT INTO keys (name, armored, is_private, encrypted_password, created_at) VALUES (?, ?, ?, ?, ?)",
		"test-priv", privArmored, true, &encPass, time.Now())
	if err != nil {
		t.Fatalf("insert private key: %v", err)
	}

	// Get key IDs
	var pubID, privID int64
	db.Get(&pubID, "SELECT id FROM keys WHERE name = 'test-pub'")
	db.Get(&privID, "SELECT id FROM keys WHERE name = 'test-priv'")

	// Step 3: Encrypt a message with the public key
	encForm := url.Values{}
	encForm.Set("key", fmt.Sprint(pubID))
	encForm.Set("input", "hello world")
	encReq := httptest.NewRequest(http.MethodPost, "/encrypt", bytes.NewBufferString(encForm.Encode()))
	encReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	encW := httptest.NewRecorder()
	a.EncryptHandler(encW, encReq)
	if encW.Code != http.StatusOK {
		t.Fatalf("encrypt: expected 200, got %d: %s", encW.Code, encW.Body.String())
	}
	armored := strings.TrimSpace(encW.Body.String())
	if !strings.HasPrefix(armored, "-----BEGIN PGP MESSAGE-----") {
		t.Fatalf("expected PGP armored output, got: %s", armored)
	}

	// Step 4: Decrypt the message with the private key
	decForm := url.Values{}
	decForm.Set("key", fmt.Sprint(privID))
	decForm.Set("input", armored)
	decReq := httptest.NewRequest(http.MethodPost, "/decrypt", bytes.NewBufferString(decForm.Encode()))
	decReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	decW := httptest.NewRecorder()
	a.DecryptHandler(decW, decReq)
	if decW.Code != http.StatusOK {
		t.Fatalf("decrypt: expected 200, got %d: %s", decW.Code, decW.Body.String())
	}
	if got := strings.TrimSpace(decW.Body.String()); got != "hello world" {
		t.Fatalf("decrypt mismatch: got %q, want %q", got, "hello world")
	}
}

// TestStory_AuthLoginLogout tests the full auth flow:
// unauthenticated → login page → auth with password → authenticated → logout → unauthenticated.
func TestStory_AuthLoginLogout(t *testing.T) {
	a, _ := setupTestApp(t)
	t.Setenv("MASTER_PASSWORD", "s3cret")

	// Step 1: Unauthenticated request to / shows login page
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	a.WithAuth(a.IndexHandler)(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for login page, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `action="/auth"`) {
		t.Fatalf("expected login form, got: %s", w.Body.String())
	}

	// Step 2: Wrong password shows error
	form := url.Values{}
	form.Set("password", "wrong")
	req2 := httptest.NewRequest(http.MethodPost, "/auth", strings.NewReader(form.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := httptest.NewRecorder()
	a.AuthHandler(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("wrong password: expected 200 (login page with error), got %d", w2.Code)
	}

	// Step 3: Correct password → redirect with cookie
	form.Set("password", "s3cret")
	req3 := httptest.NewRequest(http.MethodPost, "/auth", strings.NewReader(form.Encode()))
	req3.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w3 := httptest.NewRecorder()
	a.AuthHandler(w3, req3)
	if w3.Code != http.StatusSeeOther {
		t.Fatalf("correct password: expected 303, got %d: %s", w3.Code, w3.Body.String())
	}
	var authCookie *http.Cookie
	for _, c := range w3.Result().Cookies() {
		if c.Name == "webgpg_auth" {
			authCookie = c
			break
		}
	}
	if authCookie == nil {
		t.Fatal("auth cookie not set after successful login")
	}

	// Step 4: Authenticated request shows the app
	req4 := httptest.NewRequest(http.MethodGet, "/", nil)
	req4.AddCookie(authCookie)
	w4 := httptest.NewRecorder()
	a.WithAuth(a.IndexHandler)(w4, req4)
	if w4.Code != http.StatusOK {
		t.Fatalf("authenticated request: expected 200, got %d", w4.Code)
	}
	if !strings.Contains(w4.Body.String(), "Add") {
		t.Fatalf("expected app UI after auth, got: %s", w4.Body.String())
	}

	// Step 5: Logout clears cookie
	req5 := httptest.NewRequest(http.MethodGet, "/logout", nil)
	req5.AddCookie(authCookie)
	w5 := httptest.NewRecorder()
	a.LogoutHandler(w5, req5)
	if w5.Code != http.StatusSeeOther {
		t.Fatalf("logout: expected 303, got %d", w5.Code)
	}
}

// TestStory_ProtectedRoutesRequireAuth verifies all protected routes
// redirect to / when unauthenticated.
func TestStory_ProtectedRoutesRequireAuth(t *testing.T) {
	a, _ := setupTestApp(t)

	routes := []struct {
		method  string
		path    string
		handler http.HandlerFunc
	}{
		{http.MethodPost, "/encrypt", a.EncryptHandler},
		{http.MethodPost, "/decrypt", a.DecryptHandler},
		{http.MethodPost, "/keys", a.AddKeyHandler},
		{http.MethodPost, "/keys/delete", a.DeleteKeyHandler},
		{http.MethodGet, "/keys/view?id=1", a.ViewKeyHandler},
	}

	for _, rt := range routes {
		req := httptest.NewRequest(rt.method, rt.path, nil)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		a.WithAuth(rt.handler)(w, req)

		if w.Code != http.StatusSeeOther {
			t.Errorf("%s %s: expected 303 redirect, got %d", rt.method, rt.path, w.Code)
		}
		if loc := w.Header().Get("Location"); loc != "/" {
			t.Errorf("%s %s: expected redirect to /, got %q", rt.method, rt.path, loc)
		}
	}
}

// TestStory_AddAndDeleteKey tests adding a key and then deleting it.
func TestStory_AddAndDeleteKey(t *testing.T) {
	a, db := setupTestApp(t)

	priv, err := gcrypto.GenerateKey("Test Key", "test@test.com", "", 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pubArmored, _ := priv.GetArmoredPublicKey()

	// Add key
	form := url.Values{}
	form.Set("name", "my-test-key")
	form.Set("armored", pubArmored)
	req := httptest.NewRequest(http.MethodPost, "/keys", bytes.NewBufferString(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	a.AddKeyHandler(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("add key: expected 303, got %d: %s", w.Code, w.Body.String())
	}

	var count int
	db.Get(&count, "SELECT COUNT(*) FROM keys WHERE name = 'my-test-key'")
	if count != 1 {
		t.Fatalf("expected 1 key after add, got %d", count)
	}

	// Delete key
	var keyID int64
	db.Get(&keyID, "SELECT id FROM keys WHERE name = 'my-test-key'")

	delForm := url.Values{}
	delForm.Set("id", fmt.Sprint(keyID))
	req2 := httptest.NewRequest(http.MethodPost, "/keys/delete", bytes.NewBufferString(delForm.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := httptest.NewRecorder()
	a.DeleteKeyHandler(w2, req2)
	if w2.Code != http.StatusSeeOther {
		t.Fatalf("delete key: expected 303, got %d: %s", w2.Code, w2.Body.String())
	}

	db.Get(&count, "SELECT COUNT(*) FROM keys WHERE name = 'my-test-key'")
	if count != 0 {
		t.Fatalf("expected 0 keys after delete, got %d", count)
	}
}

// TestStory_DecryptRejectsPublicKey ensures decryption with a public-only key
// returns a clear error.
func TestStory_DecryptRejectsPublicKey(t *testing.T) {
	a, db := setupTestApp(t)

	priv, _ := gcrypto.GenerateKey("Test", "t@t.com", "", 2048)
	pubArmored, _ := priv.GetArmoredPublicKey()
	res, _ := db.Exec("INSERT INTO keys (name, armored, is_private, created_at) VALUES (?, ?, ?, ?)",
		"pub-only", pubArmored, false, time.Now())
	pubID, _ := res.LastInsertId()

	form := url.Values{}
	form.Set("key", fmt.Sprint(pubID))
	form.Set("input", "-----BEGIN PGP MESSAGE-----\nfake\n-----END PGP MESSAGE-----")
	req := httptest.NewRequest(http.MethodPost, "/decrypt", bytes.NewBufferString(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	a.DecryptHandler(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for public key decrypt, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "not a private key") {
		t.Fatalf("expected private key error, got: %s", w.Body.String())
	}
}
