package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"h-cloud.io/web-gpg/internal/app"
	cm "h-cloud.io/web-gpg/internal/crypto"
	dbpkg "h-cloud.io/web-gpg/internal/db"
	migratepkg "h-cloud.io/web-gpg/internal/migrate"
)

// findFile returns the first existing file path from candidates.
func findFile(candidates []string) string {
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// findDirectory returns the first existing directory from candidates.
func findDirectory(name string, candidates []string) string {
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			return c
		}
	}
	log.Printf("warning: %s directory not found; using default", name)
	return candidates[0]
}

// loadTemplates loads all HTML templates.
func loadTemplates() *template.Template {
	indexCandidates := []string{"templates/index.html", "./templates/index.html", "../templates/index.html", "../../templates/index.html", "/templates/index.html"}
	loginCandidates := []string{"templates/login.html", "./templates/login.html", "../templates/login.html", "../../templates/login.html", "/templates/login.html"}

	indexPath := findFile(indexCandidates)
	if indexPath == "" {
		log.Fatalf("templates/index.html not found in candidate paths")
	}

	files := []string{indexPath}
	if loginPath := findFile(loginCandidates); loginPath != "" {
		files = append(files, loginPath)
	}

	return template.Must(template.ParseFiles(files...))
}

func main() {
	// Run migrations via golang-migrate only when DATABASE_URL is set (postgres).
	// For the default sqlite (local file) we skip golang-migrate at runtime and
	// instead use the simple SQL executor below to avoid cgo sqlite driver issues
	// inside minimal container images.
	if os.Getenv("DATABASE_URL") != "" {
		if err := migratepkg.RunMigrations(); err != nil {
			log.Fatalf("migration error: %v", err)
		}
	}

	db, err := dbpkg.OpenDB()
	if err != nil {
		log.Fatalf("db open: %v", err)
	}

	// Apply simple SQL migrations (development fallback)
	if err := dbpkg.ApplySQLMigrations(db, "migrations/sql"); err != nil {
		log.Printf("migration error (dev): %v", err)
	}

	// Provide DB to crypto package so it can store/read master salt
	cm.SetDB(db)

	// Load templates
	tmpl := loadTemplates()

	// Resolve static directory
	staticDir := findDirectory("static", []string{"static", "./static", "../static", "../../static", "/static"})

	fsHandler := http.FileServer(http.Dir(staticDir))
	a := &app.App{DB: db, Templates: tmpl}

	http.Handle("/static/", http.StripPrefix("/static/", fsHandler))
	http.HandleFunc("/", a.IndexHandler)
	http.HandleFunc("/time", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(time.Now().Format("15:04:05 MST — Jan 2 2006")))
	})
	http.HandleFunc("/keys", a.AddKeyHandler)
	http.HandleFunc("/keys/view", a.ViewKeyHandler)
	http.HandleFunc("/keys/delete", a.DeleteKeyHandler)
	http.HandleFunc("/auth", a.AuthHandler)
	http.HandleFunc("/logout", a.LogoutHandler)
	http.HandleFunc("/encrypt", a.EncryptHandler)
	http.HandleFunc("/decrypt", a.DecryptHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	log.Printf("Listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
