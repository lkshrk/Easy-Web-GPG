package app

import (
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"time"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"

	cm "h-cloud.io/web-gpg/internal/crypto"
	mm "h-cloud.io/web-gpg/internal/models"
)

type App struct {
	DB        *sqlx.DB
	Templates *template.Template
}

func (a *App) IndexHandler(w http.ResponseWriter, r *http.Request) {
	// If MASTER_PASSWORD is set, require authentication via cookie
	masterPass := os.Getenv("MASTER_PASSWORD")
	if masterPass != "" {
		if c, err := r.Cookie("webgpg_auth"); err == nil {
			if cm.VerifyAuthCookieValue(c.Value, 24*60*60) {
				// proceed to render UI
			} else {
				if err := a.Templates.ExecuteTemplate(w, "login.html", nil); err != nil {
					// fallback minimal form
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.Write([]byte(`<form method="post" action="/auth"><input type="password" name="password" placeholder="Master password"/><button type="submit">Unlock</button></form>`))
				}
				return
			}
		} else {
			if err := a.Templates.ExecuteTemplate(w, "login.html", nil); err != nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Write([]byte(`<form method="post" action="/auth"><input type="password" name="password" placeholder="Master password"/><button type="submit">Unlock</button></form>`))
			}
			return
		}
	}

	var keys []mm.Key
	if err := a.DB.Select(&keys, "SELECT id, name, armored, is_private, encrypted_password, created_at FROM keys ORDER BY created_at DESC"); err != nil && err != sql.ErrNoRows {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Keys": keys,
	}
	if err := a.Templates.ExecuteTemplate(w, "index.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *App) AddKeyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := r.FormValue("name")
	armored := r.FormValue("armored")
	password := r.FormValue("password")

	// Validate key
	isPrivate := false
	if _, err := crypto.NewKeyFromArmored(armored); err == nil {
		k, _ := crypto.NewKeyFromArmored(armored)
		isPrivate = k.IsPrivate()
	}

	var encrypted *string
	var bcryptHash *string
	if password != "" {
		enc, err := cm.Encrypt([]byte(password))
		if err != nil {
			// If the server is not configured with a MASTER_PASSWORD we surface a helpful error
			if errors.Is(err, cm.ErrMasterPasswordNotSet) {
				http.Error(w, "server not configured to store passphrases: set MASTER_PASSWORD env var", http.StatusInternalServerError)
				return
			}
			http.Error(w, "failed to encrypt password", http.StatusInternalServerError)
			return
		}
		encrypted = &enc
		// additionally store bcrypt hash of the passphrase
		if h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost); err == nil {
			hs := string(h)
			bcryptHash = &hs
		}
	}

	// Try sqlite style first
	_, err := a.DB.Exec("INSERT INTO keys (name, armored, is_private, encrypted_password, password_bcrypt, created_at) VALUES (?, ?, ?, ?, ?, ?)", name, armored, isPrivate, encrypted, bcryptHash, time.Now())
	if err != nil {
		// try postgres param style
		_, err = a.DB.Exec("INSERT INTO keys (name, armored, is_private, encrypted_password, password_bcrypt, created_at) VALUES ($1, $2, $3, $4, $5, $6)", name, armored, isPrivate, encrypted, bcryptHash, time.Now())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) EncryptHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	keyID := r.FormValue("key")
	plaintext := r.FormValue("input")

	var k mm.Key
	if err := a.DB.Get(&k, "SELECT id, name, armored, is_private, encrypted_password FROM keys WHERE id = ?", keyID); err != nil {
		if err := a.DB.Get(&k, "SELECT id, name, armored, is_private, encrypted_password FROM keys WHERE id = $1", keyID); err != nil {
			http.Error(w, "key not found", http.StatusBadRequest)
			return
		}
	}

	kp, err := crypto.NewKeyFromArmored(k.Armored)
	if err != nil {
		http.Error(w, "invalid key", http.StatusBadRequest)
		return
	}
	pub, err := kp.GetArmoredPublicKey()
	if err != nil {
		http.Error(w, "failed to get public key", http.StatusInternalServerError)
		return
	}
	pubKeyObj, err := crypto.NewKeyFromArmored(pub)
	if err != nil {
		http.Error(w, "failed to parse public key", http.StatusInternalServerError)
		return
	}

	recipientKR, err := crypto.NewKeyRing(pubKeyObj)
	if err != nil {
		http.Error(w, "failed to create keyring", http.StatusInternalServerError)
		return
	}

	message := crypto.NewPlainMessageFromString(plaintext)
	pgpMsg, err := recipientKR.Encrypt(message, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	armored, err := pgpMsg.GetArmored()
	if err != nil {
		http.Error(w, "failed to armor message", http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") != "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(fmt.Sprintf("<div class=\"p-2 border rounded bg-green-50\"><pre>%s</pre></div>", armored)))
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(armored))

}

func (a *App) DecryptHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	keyID := r.FormValue("key")
	input := r.FormValue("input")

	var k mm.Key
	if err := a.DB.Get(&k, "SELECT id, name, armored, is_private, encrypted_password FROM keys WHERE id = ?", keyID); err != nil {
		if err := a.DB.Get(&k, "SELECT id, name, armored, is_private, encrypted_password FROM keys WHERE id = $1", keyID); err != nil {
			http.Error(w, "key not found", http.StatusBadRequest)
			return
		}
	}

	if !k.IsPrivate {
		http.Error(w, "selected key is not a private key", http.StatusBadRequest)
		return
	}

	priv, err := crypto.NewKeyFromArmored(k.Armored)
	if err != nil {
		http.Error(w, "invalid private key", http.StatusBadRequest)
		return
	}

	locked, err := priv.IsLocked()
	if err != nil {
		http.Error(w, "failed to inspect private key", http.StatusInternalServerError)
		return
	}

	var keyToUse *crypto.Key
	if locked {
		if k.EncryptedPasshex != nil && *k.EncryptedPasshex != "" {
			pwBytes, err := cm.Decrypt(*k.EncryptedPasshex)
			if err != nil {
				http.Error(w, "failed to decrypt stored password", http.StatusInternalServerError)
				return
			}
			unlocked, err := priv.Unlock(pwBytes)
			if err != nil {
				http.Error(w, "failed to unlock private key with stored password", http.StatusBadRequest)
				return
			}
			keyToUse = unlocked
		} else {
			http.Error(w, "private key is password protected; no stored password", http.StatusBadRequest)
			return
		}
	} else {
		keyToUse = priv
	}

	kr, err := crypto.NewKeyRing(keyToUse)
	if err != nil {
		http.Error(w, "failed to create keyring", http.StatusInternalServerError)
		return
	}

	encMessage, err := crypto.NewPGPMessageFromArmored(input)
	if err != nil {
		http.Error(w, "invalid armored message", http.StatusBadRequest)
		return
	}

	decrypted, err := kr.Decrypt(encMessage, nil, 0)
	if err != nil {
		http.Error(w, fmt.Sprintf("decrypt error: %v", err), http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") != "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(fmt.Sprintf("<div class=\"p-2 border rounded bg-blue-50\"><pre>%s</pre></div>", decrypted.GetString())))
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(decrypted.GetString()))

}

// ViewKeyHandler shows details for a single key (armored text and metadata).
func (a *App) ViewKeyHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	var k mm.Key
	if err := a.DB.Get(&k, "SELECT id, name, armored, is_private, encrypted_password, created_at FROM keys WHERE id = ?", id); err != nil {
		if err := a.DB.Get(&k, "SELECT id, name, armored, is_private, encrypted_password, created_at FROM keys WHERE id = $1", id); err != nil {
			http.Error(w, "key not found", http.StatusNotFound)
			return
		}
	}

	// Render a small HTML fragment with key details
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "<div class=\"p-2 border rounded bg-gray-50\"><strong>%s</strong> — %s — Added %s<pre class=\"mt-2 p-2 bg-white text-sm\">%s</pre></div>",
		template.HTMLEscapeString(k.Name),
		func() string {
			if k.IsPrivate {
				return "Private"
			} else {
				return "Public"
			}
		}(),
		template.HTMLEscapeString(k.CreatedAt.String()),
		template.HTMLEscapeString(k.Armored),
	)
}

// DeleteKeyHandler deletes a key by id and redirects back to index.
func (a *App) DeleteKeyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.FormValue("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	// Try sqlite param style first
	if _, err := a.DB.Exec("DELETE FROM keys WHERE id = ?", id); err != nil {
		if _, err := a.DB.Exec("DELETE FROM keys WHERE id = $1", id); err != nil {
			http.Error(w, "failed to delete key", http.StatusInternalServerError)
			return
		}
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// AuthHandler verifies the posted master password and sets a signed cookie
// valid for one day. The expected password is read from env MASTER_PASSWORD.
func (a *App) AuthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pass := r.FormValue("password")
	// verify posted password by deriving keys via Argon2id and comparing
	ok, err := cm.VerifyMasterPassword(pass)
	if err != nil {
		if err == cm.ErrMasterPasswordNotSet {
			http.Error(w, "server not configured for master password", http.StatusInternalServerError)
			return
		}
		http.Error(w, "internal error verifying password", http.StatusInternalServerError)
		return
	}
	if !ok {
		a.Templates.ExecuteTemplate(w, "login.html", map[string]interface{}{"Error": "invalid password"})
		return
	}
	// create signed cookie value
	val, err := cm.CreateAuthCookieValue()
	if err != nil {
		http.Error(w, "failed to create auth token", http.StatusInternalServerError)
		return
	}
	cookie := &http.Cookie{
		Name:     "webgpg_auth",
		Value:    val,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   24 * 60 * 60,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// LogoutHandler removes the auth cookie.
func (a *App) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie := &http.Cookie{Name: "webgpg_auth", Value: "", Path: "/", MaxAge: -1}
	http.SetCookie(w, cookie)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
