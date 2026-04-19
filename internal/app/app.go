package app

import (
	"database/sql"
	"html/template"
	"net/http"

	"github.com/jmoiron/sqlx"

	cm "h-cloud.io/web-gpg/internal/crypto"
	mm "h-cloud.io/web-gpg/internal/models"
)

const authCookieMaxAge int64 = 86400 // 24 hours in seconds

// App holds shared dependencies for all HTTP handlers.
type App struct {
	DB        *sqlx.DB
	Templates *template.Template
	Crypto    *cm.CryptoService
}

// IndexHandler renders the main page with all stored keys.
func (a *App) IndexHandler(w http.ResponseWriter, r *http.Request) {
	var keys []mm.Key
	err := a.DB.SelectContext(r.Context(), &keys,
		"SELECT id, name, armored, is_private, encrypted_password, created_at FROM keys ORDER BY created_at DESC")
	if err != nil && err != sql.ErrNoRows {
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
