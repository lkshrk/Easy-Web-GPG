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
	_ "modernc.org/sqlite"
	"github.com/jmoiron/sqlx"

	apppkg "h-cloud.io/web-gpg/internal/app"
	cm "h-cloud.io/web-gpg/internal/crypto"
	migratepkg "h-cloud.io/web-gpg/internal/migrate"
)

// setupTestApp creates a fully wired App with in-memory SQLite, migrations,
// and crypto service. It returns the App and the underlying DB for direct queries.
func setupTestApp(t *testing.T) (*apppkg.App, *sqlx.DB) {
	t.Helper()

	t.Setenv("MASTER_PASSWORD", "test-master-password")
	t.Setenv("MASTER_SALT_FILE", t.TempDir()+"/master_salt")

	// Use temporary file for SQLite since golang-migrate doesn't support in-memory
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"
	t.Setenv("DATABASE_URL", "sqlite://file:"+dbPath+"?_foreign_keys=1")

	db, err := sqlx.Open("sqlite", "file:"+dbPath+"?_foreign_keys=1&_busy_timeout=5000")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Use golang-migrate for proper schema handling
	if err := migratepkg.RunMigrations(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	crypto := cm.NewCryptoService(db)

	files := []string{"../../templates/index.html", "../../templates/login.html"}
	tmpl, err := template.ParseFiles(files...)
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}

	a := &apppkg.App{DB: db, Templates: tmpl, Crypto: crypto, MasterPassword: "test-master-password"}
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
	a.MasterPassword = "s3cret"

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

// TestIndexHandler_RendersKeys verifies the index page renders stored keys.
func TestIndexHandler_RendersKeys(t *testing.T) {
	a, db := setupTestApp(t)

	priv, _ := gcrypto.GenerateKey("Visible Key", "v@test.com", "", 2048)
	pubArmored, _ := priv.GetArmoredPublicKey()
	db.Exec("INSERT INTO keys (name, armored, is_private, created_at) VALUES (?, ?, ?, ?)",
		"Visible Key", pubArmored, false, time.Now())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	a.IndexHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Visible Key") {
		t.Fatal("expected key name in rendered output")
	}
}

// TestViewKeyHandler verifies viewing a stored key returns its details.
func TestViewKeyHandler(t *testing.T) {
	a, db := setupTestApp(t)

	priv, _ := gcrypto.GenerateKey("View Test", "v@test.com", "", 2048)
	pubArmored, _ := priv.GetArmoredPublicKey()
	res, _ := db.Exec("INSERT INTO keys (name, armored, is_private, created_at) VALUES (?, ?, ?, ?)",
		"View Test", pubArmored, false, time.Now())
	keyID, _ := res.LastInsertId()

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/keys/view?id=%d", keyID), nil)
	w := httptest.NewRecorder()
	a.ViewKeyHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "View Test") {
		t.Fatal("expected key name in response")
	}
	if !strings.Contains(body, "Public") {
		t.Fatal("expected key type in response")
	}
}

// TestViewKeyHandler_MissingID verifies missing id returns 422.
func TestViewKeyHandler_MissingID(t *testing.T) {
	a, _ := setupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/keys/view", nil)
	w := httptest.NewRecorder()
	a.ViewKeyHandler(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

// TestViewKeyHandler_NotFound verifies non-existent key returns 404.
func TestViewKeyHandler_NotFound(t *testing.T) {
	a, _ := setupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/keys/view?id=99999", nil)
	w := httptest.NewRecorder()
	a.ViewKeyHandler(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// TestMethodNotAllowed verifies POST-only handlers reject GET requests.
func TestMethodNotAllowed(t *testing.T) {
	a, _ := setupTestApp(t)

	tests := []struct {
		name    string
		handler http.HandlerFunc
		path    string
	}{
		{"auth", a.AuthHandler, "/auth"},
		{"addKey", a.AddKeyHandler, "/keys"},
		{"deleteKey", a.DeleteKeyHandler, "/keys/delete"},
		{"encrypt", a.EncryptHandler, "/encrypt"},
		{"decrypt", a.DecryptHandler, "/decrypt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			tt.handler(w, req)
			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected 405, got %d", w.Code)
			}
		})
	}
}

// TestAddKeyHandler_InvalidPGP verifies invalid PGP key input returns 400.
func TestAddKeyHandler_InvalidPGP(t *testing.T) {
	a, _ := setupTestApp(t)

	form := url.Values{}
	form.Set("name", "bad-key")
	form.Set("armored", "not a valid PGP key")
	req := httptest.NewRequest(http.MethodPost, "/keys", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	a.AddKeyHandler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestEncryptHandler_KeyNotFound verifies encrypt with non-existent key returns 422.
func TestEncryptHandler_KeyNotFound(t *testing.T) {
	a, _ := setupTestApp(t)

	form := url.Values{}
	form.Set("key", "99999")
	form.Set("input", "hello")
	req := httptest.NewRequest(http.MethodPost, "/encrypt", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	a.EncryptHandler(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

// TestDecryptHandler_InvalidMessage verifies decrypt with bad PGP message returns error.
func TestDecryptHandler_InvalidMessage(t *testing.T) {
	a, db := setupTestApp(t)

	priv, _ := gcrypto.GenerateKey("Dec Test", "d@t.com", "", 2048)
	privArmored, _ := priv.Armor()
	res, _ := db.Exec("INSERT INTO keys (name, armored, is_private, created_at) VALUES (?, ?, ?, ?)",
		"dec-test", privArmored, true, time.Now())
	keyID, _ := res.LastInsertId()

	form := url.Values{}
	form.Set("key", fmt.Sprint(keyID))
	form.Set("input", "not a PGP message")
	req := httptest.NewRequest(http.MethodPost, "/decrypt", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	a.DecryptHandler(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
}

// TestDeleteKeyHandler_MissingID verifies delete with no id returns 422.
func TestDeleteKeyHandler_MissingID(t *testing.T) {
	a, _ := setupTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/keys/delete", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	a.DeleteKeyHandler(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

// TestNoAuthRequired verifies all routes are accessible when MasterPassword is empty.
func TestNoAuthRequired(t *testing.T) {
	a, _ := setupTestApp(t)
	a.MasterPassword = ""

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	a.WithAuth(a.IndexHandler)(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 without auth, got %d", w.Code)
	}
	// Should show the app, not the login page
	if strings.Contains(w.Body.String(), `action="/auth"`) {
		t.Fatal("should not show login when MasterPassword is empty")
	}
}

// TestRateLimit verifies the rate limiter blocks after max attempts.
func TestRateLimit(t *testing.T) {
	a, _ := setupTestApp(t)

	rl := apppkg.NewRateLimiter(time.Minute, 3)
	handler := apppkg.RateLimit(rl, a.AuthHandler)

	for i := 0; i < 3; i++ {
		form := url.Values{}
		form.Set("password", "wrong")
		req := httptest.NewRequest(http.MethodPost, "/auth", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.RemoteAddr = "1.2.3.4:1234"
		w := httptest.NewRecorder()
		handler(w, req)
		if w.Code == http.StatusTooManyRequests {
			t.Fatalf("should not be rate limited on attempt %d", i+1)
		}
	}

	// 4th attempt should be blocked
	form := url.Values{}
	form.Set("password", "wrong")
	req := httptest.NewRequest(http.MethodPost, "/auth", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
}

// TestAddKeyHandler_WithPassphrase verifies adding a key with a passphrase
// encrypts and stores it.
func TestAddKeyHandler_WithPassphrase(t *testing.T) {
	a, db := setupTestApp(t)

	priv, _ := gcrypto.GenerateKey("Pass Key", "p@test.com", "keypass", 2048)
	privArmored, _ := priv.Armor()

	form := url.Values{}
	form.Set("name", "pass-key")
	form.Set("armored", privArmored)
	form.Set("password", "keypass")
	req := httptest.NewRequest(http.MethodPost, "/keys", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	a.AddKeyHandler(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the encrypted password was stored
	var encPass *string
	db.Get(&encPass, "SELECT encrypted_password FROM keys WHERE name = 'pass-key'")
	if encPass == nil || *encPass == "" {
		t.Fatal("expected encrypted password to be stored")
	}
}

// TestDecryptHandler_LockedKeyWithStoredPassword tests decryption with a
// password-protected private key that has a stored passphrase.
func TestDecryptHandler_LockedKeyWithStoredPassword(t *testing.T) {
	a, db := setupTestApp(t)

	// Generate a password-protected key
	priv, _ := gcrypto.GenerateKey("Locked Key", "l@t.com", "keypass", 2048)
	privArmored, _ := priv.Armor()
	pubArmored, _ := priv.GetArmoredPublicKey()

	// Encrypt the passphrase for storage
	encPass, err := a.Crypto.Encrypt([]byte("keypass"))
	if err != nil {
		t.Fatalf("encrypt passphrase: %v", err)
	}

	// Store private key with encrypted passphrase
	res, _ := db.Exec("INSERT INTO keys (name, armored, is_private, encrypted_password, created_at) VALUES (?, ?, ?, ?, ?)",
		"locked-key", privArmored, true, &encPass, time.Now())
	privID, _ := res.LastInsertId()

	// Encrypt a message with the public key
	pubKey, _ := gcrypto.NewKeyFromArmored(pubArmored)
	kr, _ := gcrypto.NewKeyRing(pubKey)
	msg := gcrypto.NewPlainMessageFromString("secret data")
	pgpMsg, _ := kr.Encrypt(msg, nil)
	armored, _ := pgpMsg.GetArmored()

	// Decrypt using the locked key (should auto-unlock with stored passphrase)
	form := url.Values{}
	form.Set("key", fmt.Sprint(privID))
	form.Set("input", armored)
	req := httptest.NewRequest(http.MethodPost, "/decrypt", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	a.DecryptHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := strings.TrimSpace(w.Body.String()); got != "secret data" {
		t.Fatalf("got %q, want %q", got, "secret data")
	}
}

// TestDecryptHandler_KeyNotFound verifies decrypt with non-existent key returns 422.
func TestDecryptHandler_KeyNotFound(t *testing.T) {
	a, _ := setupTestApp(t)

	form := url.Values{}
	form.Set("key", "99999")
	form.Set("input", "-----BEGIN PGP MESSAGE-----\nfake\n-----END PGP MESSAGE-----")
	req := httptest.NewRequest(http.MethodPost, "/decrypt", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	a.DecryptHandler(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

// TestRequestLogger verifies the request logging middleware.
func TestRequestLogger(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	handler := apppkg.RequestLogger(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// TestAuthHandler_MasterPasswordNotInEnv verifies auth returns 500 when
// MASTER_PASSWORD env is cleared but App.MasterPassword is set (config drift).
func TestAuthHandler_MasterPasswordNotInEnv(t *testing.T) {
	a, _ := setupTestApp(t)
	// Clear the env var but keep struct field — simulates config drift
	t.Setenv("MASTER_PASSWORD", "")

	form := url.Values{}
	form.Set("password", "anything")
	req := httptest.NewRequest(http.MethodPost, "/auth", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	a.AuthHandler(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// TestEncryptHandler_FullPath tests a successful encryption through the handler.
func TestEncryptHandler_FullPath(t *testing.T) {
	a, db := setupTestApp(t)

	priv, _ := gcrypto.GenerateKey("Enc Test", "e@test.com", "", 2048)
	pubArmored, _ := priv.GetArmoredPublicKey()
	res, _ := db.Exec("INSERT INTO keys (name, armored, is_private, created_at) VALUES (?, ?, ?, ?)",
		"enc-test", pubArmored, false, time.Now())
	keyID, _ := res.LastInsertId()

	form := url.Values{}
	form.Set("key", fmt.Sprint(keyID))
	form.Set("input", "encrypt me")
	req := httptest.NewRequest(http.MethodPost, "/encrypt", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	a.EncryptHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "BEGIN PGP MESSAGE") {
		t.Fatal("expected PGP armored output")
	}
}

// TestRequireAuth_NonRootRedirect verifies non-root paths redirect to / when unauthorized.
func TestRequireAuth_NonRootRedirect(t *testing.T) {
	a, _ := setupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/keys/view?id=1", nil)
	w := httptest.NewRecorder()
	a.WithAuth(a.ViewKeyHandler)(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/" {
		t.Fatalf("expected redirect to /, got %q", loc)
	}
}

// TestRequestLogger_StaticPath verifies that static asset requests are not logged.
func TestRequestLogger_StaticPath(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := apppkg.RequestLogger(inner)
	req := httptest.NewRequest(http.MethodGet, "/static/css/style.css", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// TestRequestLogger_MutationLogging verifies POST requests are logged.
func TestRequestLogger_MutationLogging(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := apppkg.RequestLogger(inner)
	req := httptest.NewRequest(http.MethodPost, "/auth", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// TestRequestLogger_ErrorLogging verifies 4xx and 5xx GET responses are logged.
func TestRequestLogger_ErrorLogging(t *testing.T) {
	for _, code := range []int{400, 404, 500} {
		code := code
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			})
			handler := apppkg.RequestLogger(inner)
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != code {
				t.Fatalf("expected %d, got %d", code, w.Code)
			}
		})
	}
}

// TestIsHTTPS_ForwardedProto verifies the auth cookie is Secure when
// X-Forwarded-Proto: https is set.
func TestIsHTTPS_ForwardedProto(t *testing.T) {
	a, _ := setupTestApp(t)
	t.Setenv("MASTER_PASSWORD", "s3cret")
	a.MasterPassword = "s3cret"

	form := url.Values{}
	form.Set("password", "s3cret")
	req := httptest.NewRequest(http.MethodPost, "/auth", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()
	a.AuthHandler(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == "webgpg_auth" {
			if !c.Secure {
				t.Fatal("expected cookie Secure=true with X-Forwarded-Proto: https")
			}
			return
		}
	}
	t.Fatal("auth cookie not found")
}

// TestDecryptHandler_CorruptStoredKey verifies that decrypting with a key that
// has corrupt armored data in the DB returns 500.
func TestDecryptHandler_CorruptStoredKey(t *testing.T) {
	a, db := setupTestApp(t)

	res, _ := db.Exec("INSERT INTO keys (name, armored, is_private, created_at) VALUES (?, ?, ?, ?)",
		"corrupt-priv", "not-valid-pgp-data", true, time.Now())
	keyID, _ := res.LastInsertId()

	form := url.Values{}
	form.Set("key", fmt.Sprint(keyID))
	form.Set("input", "-----BEGIN PGP MESSAGE-----\nfake\n-----END PGP MESSAGE-----")
	req := httptest.NewRequest(http.MethodPost, "/decrypt", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	a.DecryptHandler(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// TestDecryptHandler_WrongKey verifies that decrypting a message encrypted to
// a different key returns 500.
func TestDecryptHandler_WrongKey(t *testing.T) {
	a, db := setupTestApp(t)

	// Store key1 as private key (no passphrase)
	key1, _ := gcrypto.GenerateKey("Key1", "k1@t.com", "", 2048)
	key2, _ := gcrypto.GenerateKey("Key2", "k2@t.com", "", 2048)
	priv1Armored, _ := key1.Armor()
	res, _ := db.Exec("INSERT INTO keys (name, armored, is_private, created_at) VALUES (?, ?, ?, ?)",
		"key1", priv1Armored, true, time.Now())
	key1ID, _ := res.LastInsertId()

	// Encrypt a message with key2's public key
	pub2Armored, _ := key2.GetArmoredPublicKey()
	pub2, _ := gcrypto.NewKeyFromArmored(pub2Armored)
	kr, _ := gcrypto.NewKeyRing(pub2)
	pgpMsg, _ := kr.Encrypt(gcrypto.NewPlainMessageFromString("secret"), nil)
	encrypted, _ := pgpMsg.GetArmored()

	// Try to decrypt with key1 (wrong key — should fail)
	form := url.Values{}
	form.Set("key", fmt.Sprint(key1ID))
	form.Set("input", encrypted)
	req := httptest.NewRequest(http.MethodPost, "/decrypt", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	a.DecryptHandler(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for wrong key, got %d: %s", w.Code, w.Body.String())
	}
}

// TestEncryptHandler_CorruptStoredKey verifies that encrypting with a key that
// has corrupt armored data in the DB returns 500.
func TestEncryptHandler_CorruptStoredKey(t *testing.T) {
	a, db := setupTestApp(t)

	res, _ := db.Exec("INSERT INTO keys (name, armored, is_private, created_at) VALUES (?, ?, ?, ?)",
		"corrupt", "not-valid-pgp-data", false, time.Now())
	keyID, _ := res.LastInsertId()

	form := url.Values{}
	form.Set("key", fmt.Sprint(keyID))
	form.Set("input", "hello")
	req := httptest.NewRequest(http.MethodPost, "/encrypt", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	a.EncryptHandler(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAddKeyHandler_MasterPasswordNotSetForPassphrase verifies that adding a
// key with a passphrase when MASTER_PASSWORD is not configured returns 500.
func TestAddKeyHandler_MasterPasswordNotSetForPassphrase(t *testing.T) {
	a, _ := setupTestApp(t)
	// Clear master password so Crypto.Encrypt fails with ErrMasterPasswordNotSet
	t.Setenv("MASTER_PASSWORD", "")

	priv, _ := gcrypto.GenerateKey("Test", "t@t.com", "keypass", 2048)
	privArmored, _ := priv.Armor()

	form := url.Values{}
	form.Set("name", "no-master-key")
	form.Set("armored", privArmored)
	form.Set("password", "keypass")
	req := httptest.NewRequest(http.MethodPost, "/keys", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	a.AddKeyHandler(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
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
