package app

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"golang.org/x/crypto/bcrypt"

	cm "h-cloud.io/web-gpg/internal/crypto"
	mm "h-cloud.io/web-gpg/internal/models"
)

// AddKeyHandler stores a new PGP key.
func (a *App) AddKeyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := r.FormValue("name")
	armored := r.FormValue("armored")
	password := r.FormValue("password")

	k, err := crypto.NewKeyFromArmored(armored)
	if err != nil {
		http.Error(w, "invalid PGP key", http.StatusBadRequest)
		return
	}
	isPrivate := k.IsPrivate()

	var encrypted *string
	var bcryptHash *string
	if password != "" {
		enc, err := a.Crypto.Encrypt([]byte(password))
		if err != nil {
			if errors.Is(err, cm.ErrMasterPasswordNotSet) {
				http.Error(w, "server not configured to store passphrases: set MASTER_PASSWORD env var", http.StatusInternalServerError)
				return
			}
			http.Error(w, "failed to encrypt password", http.StatusInternalServerError)
			return
		}
		encrypted = &enc
		if h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost); err == nil {
			hs := string(h)
			bcryptHash = &hs
		}
	}

	q := a.DB.Rebind("INSERT INTO keys (name, armored, is_private, encrypted_password, password_bcrypt, created_at) VALUES (?, ?, ?, ?, ?, ?)")
	if _, err = a.DB.ExecContext(r.Context(), q, name, armored, isPrivate, encrypted, bcryptHash, time.Now()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ViewKeyHandler returns key details as an HTML fragment.
func (a *App) ViewKeyHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusUnprocessableEntity)
		return
	}
	var k mm.Key
	q := a.DB.Rebind("SELECT id, name, armored, is_private, encrypted_password, created_at FROM keys WHERE id = ?")
	if err := a.DB.GetContext(r.Context(), &k, q, id); err != nil {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}

	keyType := "Public"
	if k.IsPrivate {
		keyType = "Private"
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<div class="p-2 border rounded bg-gray-50"><strong>%s</strong> — %s — Added %s<pre class="mt-2 p-2 bg-white text-sm">%s</pre></div>`,
		template.HTMLEscapeString(k.Name),
		keyType,
		template.HTMLEscapeString(k.CreatedAt.String()),
		template.HTMLEscapeString(k.Armored),
	)
}

// DeleteKeyHandler removes a key by ID.
func (a *App) DeleteKeyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.FormValue("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusUnprocessableEntity)
		return
	}
	q := a.DB.Rebind("DELETE FROM keys WHERE id = ?")
	if _, err := a.DB.ExecContext(r.Context(), q, id); err != nil {
		http.Error(w, "failed to delete key", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
