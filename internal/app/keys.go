package app

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ProtonMail/gopenpgp/v3/crypto"
	"golang.org/x/crypto/bcrypt"

	cm "h-cloud.io/web-gpg/internal/crypto"
	mm "h-cloud.io/web-gpg/internal/models"
)

// sanitizeArmored cleans common paste artifacts from an armored PGP block:
//   - strips BOM, zero-width spaces, and non-breaking spaces from the outer edges
//   - normalises line endings to LF
//   - discards any content before -----BEGIN PGP and after -----END PGP
//   - trims trailing whitespace from every line inside the block
//
// Base64 characters (A-Za-z0-9+/=) never include whitespace, so trimming
// individual lines is safe and does not alter the encoded key material.
func sanitizeArmored(s string) string {
	// Strip invisible lead/trail chars that copy-paste sometimes introduces:
	// BOM (U+FEFF), zero-width space (U+200B), non-breaking space (U+00A0),
	// plus ordinary whitespace.
	invisible := "\ufeff\u200b\u00a0"
	s = strings.TrimFunc(s, func(r rune) bool {
		return strings.ContainsRune(invisible, r) || r == ' ' || r == '\t' ||
			r == '\n' || r == '\r'
	})

	// Normalise line endings.
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	lines := strings.Split(s, "\n")

	// Locate the outermost BEGIN / END PGP markers.
	begin, end := -1, -1
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "-----BEGIN PGP") && begin == -1 {
			begin = i
		}
		if strings.HasPrefix(t, "-----END PGP") {
			end = i
		}
	}

	if begin == -1 || end == -1 || end < begin {
		// No recognisable PGP block — return trimmed input unchanged;
		// the parser will produce a useful error message.
		return s
	}

	block := lines[begin : end+1]
	for i, line := range block {
		block[i] = strings.TrimFunc(line, func(r rune) bool {
			return r == ' ' || r == '\t' || r == '\u00a0'
		})
	}

	return strings.Join(block, "\n") + "\n"
}

// AddKeyHandler stores a new PGP key.
func (a *App) AddKeyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := r.FormValue("name")
	password := r.FormValue("password")

	armored := sanitizeArmored(r.FormValue("armored"))

	k, err := crypto.NewKeyFromArmored(armored)
	if err != nil {
		slog.Error("failed to parse armored PGP key", "name", name, "err", err)
		http.Error(w, "invalid PGP key: "+err.Error(), http.StatusBadRequest)
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
			slog.Error("failed to encrypt passphrase for storage", "name", name, "err", err)
			http.Error(w, "failed to encrypt passphrase: "+err.Error(), http.StatusInternalServerError)
			return
		}
		encrypted = &enc
		// Pre-hash with SHA-256 so input is always 32 bytes; bcrypt silently
		// truncates at 72 bytes and rejects longer inputs in recent versions.
		ph := sha256.Sum256([]byte(password))
		if h, err := bcrypt.GenerateFromPassword(ph[:], bcrypt.DefaultCost); err != nil {
			slog.Warn("failed to bcrypt passphrase; key stored without bcrypt hash", "name", name, "err", err)
		} else {
			hs := string(h)
			bcryptHash = &hs
		}
	}

	q := a.DB.Rebind("INSERT INTO keys (name, armored, is_private, encrypted_password, password_bcrypt, created_at) VALUES (?, ?, ?, ?, ?, ?)")
	if _, err = a.DB.ExecContext(r.Context(), q, name, armored, isPrivate, encrypted, bcryptHash, time.Now()); err != nil {
		slog.Error("failed to insert key", "name", name, "err", err)
		http.Error(w, "failed to store key: "+err.Error(), http.StatusInternalServerError)
		return
	}

	slog.Info("key added", "name", name, "private", isPrivate)
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
	fmt.Fprintf(w, `<div class="p-3 border border-[#292e42] rounded-md bg-[#24283b]"><strong class="text-[#c0caf5]">%s</strong> <span class="text-[#565f89]">—</span> <span class="text-[#7aa2f7]">%s</span> <span class="text-[#565f89]">— Added %s</span><pre class="mt-2 p-2 bg-[#16161e] text-sm text-[#a9b1d6] rounded overflow-x-auto">%s</pre></div>`,
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
		slog.Error("failed to delete key", "id", id, "err", err)
		http.Error(w, "failed to delete key: "+err.Error(), http.StatusInternalServerError)
		return
	}
	slog.Info("key deleted", "id", id)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
