package app_test

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
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

func TestEncryptDecryptHandlers(t *testing.T) {
	// Setup master key
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i + 1)
	}
	os.Setenv("MASTER_KEY", base64.StdEncoding.EncodeToString(raw))

	db, err := sqlx.Open("sqlite", "file::memory:?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := dbpkg.ApplySQLMigrations(db, "../../migrations/sql"); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	tmpl := templateMust()
	a := &apppkg.App{DB: db, Templates: tmpl}

	// generate key
	priv, err := gcrypto.GenerateKey("Test User", "test@example.com", "pass", 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	privArmored, _ := priv.Armor()
	pubArmored, _ := priv.GetArmoredPublicKey()

	// insert public key
	res, err := db.Exec("INSERT INTO keys (name, armored, is_private, encrypted_password, created_at) VALUES (?, ?, ?, ?, ?)", "pub", pubArmored, false, nil, time.Now())
	if err != nil {
		t.Fatalf("insert pub: %v", err)
	}
	pubID, _ := res.LastInsertId()

	// insert private key
	encPass, err := cm.Encrypt([]byte("pass"))
	if err != nil {
		t.Fatalf("encrypt pw: %v", err)
	}
	res2, err := db.Exec("INSERT INTO keys (name, armored, is_private, encrypted_password, created_at) VALUES (?, ?, ?, ?, ?)", "priv", privArmored, true, &encPass, time.Now())
	if err != nil {
		t.Fatalf("insert priv: %v", err)
	}
	privID, _ := res2.LastInsertId()

	// Encrypt
	form := url.Values{}
	form.Set("key", strings.TrimSpace(fmt.Sprint(pubID)))
	form.Set("input", "hello world")
	req := httptest.NewRequest(http.MethodPost, "/encrypt", bytes.NewBufferString(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	a.EncryptHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("encrypt handler status: %d body: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	// extract armored from <pre>
	start := strings.Index(body, "<pre>")
	end := strings.Index(body, "</pre>")
	if start < 0 || end < 0 {
		t.Fatalf("unexpected encrypt response: %s", body)
	}
	armored := body[start+5 : end]

	// Decrypt
	form2 := url.Values{}
	form2.Set("key", strings.TrimSpace(fmt.Sprint(privID)))
	form2.Set("input", armored)
	req2 := httptest.NewRequest(http.MethodPost, "/decrypt", bytes.NewBufferString(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.Header.Set("HX-Request", "true")
	w2 := httptest.NewRecorder()
	a.DecryptHandler(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("decrypt handler status: %d body: %s", w2.Code, w2.Body.String())
	}
	body2 := w2.Body.String()
	s2start := strings.Index(body2, "<pre>")
	s2end := strings.Index(body2, "</pre>")
	if s2start < 0 || s2end < 0 {
		t.Fatalf("unexpected decrypt response: %s", body2)
	}
	got := body2[s2start+5 : s2end]
	if strings.TrimSpace(got) != "hello world" {
		t.Fatalf("decrypt mismatch: got %q", got)
	}
}

// templateMust tries to parse the templates file used by the app for tests.
func templateMust() *template.Template {
	// parse both index and login templates if present
	files := []string{"../../templates/index.html", "../../templates/login.html"}
	t, err := template.ParseFiles(files...)
	if err != nil {
		panic(err)
	}
	return t
}

func TestAuthFlow(t *testing.T) {
	// Setup master key and password
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i + 1)
	}
	os.Setenv("MASTER_KEY", base64.StdEncoding.EncodeToString(raw))
	os.Setenv("MASTER_PASSWORD", "s3cret")

	db, err := sqlx.Open("sqlite", "file::memory:?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := dbpkg.ApplySQLMigrations(db, "../../migrations/sql"); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	tmpl := templateMust()
	a := &apppkg.App{DB: db, Templates: tmpl}

	// Visit index -> should see login form
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	a.IndexHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("index status before auth: %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Enter master password") {
		t.Fatalf("expected login page, got: %s", w.Body.String())
	}

	// Post correct password to /auth
	form := url.Values{}
	form.Set("password", "s3cret")
	req2 := httptest.NewRequest(http.MethodPost, "/auth", strings.NewReader(form.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := httptest.NewRecorder()
	a.AuthHandler(w2, req2)
	if w2.Code != http.StatusSeeOther {
		t.Fatalf("auth handler status: %d body: %s", w2.Code, w2.Body.String())
	}
	// grab cookie
	res := w2.Result()
	cookies := res.Cookies()
	var authCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "webgpg_auth" {
			authCookie = c
			break
		}
	}
	if authCookie == nil {
		t.Fatalf("auth cookie not set")
	}

	// Visit index with cookie -> should see app UI
	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	req3.AddCookie(authCookie)
	w3 := httptest.NewRecorder()
	a.IndexHandler(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("index status after auth: %d", w3.Code)
	}
	if !strings.Contains(w3.Body.String(), "Add Key") {
		t.Fatalf("expected app UI after auth, got: %s", w3.Body.String())
	}
}
