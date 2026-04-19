package app

import (
	"database/sql"
	"errors"
	"html/template"
	"log"
	"net/http"

	"github.com/jmoiron/sqlx"

	cm "h-cloud.io/web-gpg/internal/crypto"
	mm "h-cloud.io/web-gpg/internal/models"
)

const authCookieMaxAge int64 = 86400 // 24 hours in seconds

// App holds shared dependencies for all HTTP handlers.
type App struct {
	DB             *sqlx.DB
	Templates      *template.Template
	Crypto         *cm.CryptoService
	MasterPassword string // read once at startup from MASTER_PASSWORD env
}

// IndexHandler renders the main page with all stored keys.
func (a *App) IndexHandler(w http.ResponseWriter, r *http.Request) {
	var keys []mm.Key
	err := a.DB.SelectContext(r.Context(), &keys,
		"SELECT id, name, armored, is_private, encrypted_password, created_at FROM keys ORDER BY created_at DESC")
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Printf("error loading keys: %v", err)
		http.Error(w, "failed to load keys", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Keys": keys,
	}
	if err := a.Templates.ExecuteTemplate(w, "index.html", data); err != nil {
		log.Printf("error rendering template: %v", err)
		http.Error(w, "failed to render page", http.StatusInternalServerError)
	}
}
