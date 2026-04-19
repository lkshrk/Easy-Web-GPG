package app

import (
	"errors"
	"log"
	"net/http"
	"os"

	cm "h-cloud.io/web-gpg/internal/crypto"
)

func (a *App) requireAuth(w http.ResponseWriter, r *http.Request) bool {
	if os.Getenv("MASTER_PASSWORD") == "" {
		return true
	}

	c, err := r.Cookie("webgpg_auth")
	if err != nil || !a.Crypto.VerifyAuthCookieValue(c.Value, authCookieMaxAge) {
		if r.URL.Path == "/" {
			if err := a.Templates.ExecuteTemplate(w, "login.html", map[string]interface{}{}); err != nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Write([]byte(`<form method="post" action="/auth"><input type="password" name="password" placeholder="Master password"/><button type="submit">Unlock</button></form>`))
			}
			return false
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return false
	}
	return true
}

// WithAuth wraps a handler with authentication enforcement.
func (a *App) WithAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.requireAuth(w, r) {
			return
		}
		next(w, r)
	}
}

// AuthHandler validates the master password and sets an auth cookie.
func (a *App) AuthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pass := r.FormValue("password")
	ok, err := a.Crypto.VerifyMasterPassword(pass)
	if err != nil {
		if errors.Is(err, cm.ErrMasterPasswordNotSet) {
			http.Error(w, "server not configured for master password", http.StatusInternalServerError)
			return
		}
		log.Printf("error verifying master password: %v", err)
		http.Error(w, "internal error verifying password", http.StatusInternalServerError)
		return
	}
	if !ok {
		a.Templates.ExecuteTemplate(w, "login.html", map[string]interface{}{"Error": "invalid password"})
		return
	}

	val, err := a.Crypto.CreateAuthCookieValue()
	if err != nil {
		http.Error(w, "failed to create auth token", http.StatusInternalServerError)
		return
	}
	cookie := &http.Cookie{
		Name:     "webgpg_auth",
		Value:    val,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   int(authCookieMaxAge),
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// LogoutHandler clears the auth cookie and redirects to root.
func (a *App) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie := &http.Cookie{Name: "webgpg_auth", Value: "", Path: "/", MaxAge: -1}
	http.SetCookie(w, cookie)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
