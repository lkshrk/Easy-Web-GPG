package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"h-cloud.io/web-gpg/internal/app"
	dbpkg "h-cloud.io/web-gpg/internal/db"
	migratepkg "h-cloud.io/web-gpg/internal/migrate"
)

func main() {
	// Run migrations from embedded files (runtime)
	if err := migratepkg.RunMigrations(); err != nil {
		log.Fatalf("migration error: %v", err)
	}

	db, err := dbpkg.OpenDB()
	if err != nil {
		log.Fatalf("db open: %v", err)
	}

	// Apply simple SQL migrations (development fallback)
	if err := dbpkg.ApplySQLMigrations(db, "migrations/sql"); err != nil {
		log.Printf("migration error (dev): %v", err)
	}

	// Resolve template path candidates
	tplCandidates := []string{"templates/index.html", "./templates/index.html", "../templates/index.html", "../../templates/index.html", "/templates/index.html"}
	// Also look for login template
	tplCandidates2 := []string{"templates/login.html", "./templates/login.html", "../templates/login.html"}
	var tmpl *template.Template
	for _, c := range tplCandidates {
		if _, err := os.Stat(c); err == nil {
			// try to parse both index and login templates if available
			files := []string{c}
			for _, l := range tplCandidates2 {
				if _, err := os.Stat(l); err == nil {
					files = append(files, l)
					break
				}
			}
			tmpl = template.Must(template.ParseFiles(files...))
			break
		}
	}
	if tmpl == nil {
		log.Fatalf("templates/index.html not found in candidate paths")
	}

	// Resolve static dir candidates
	staticCandidates := []string{"static/dist", "./static/dist", "../static/dist", "../../static/dist", "/static/dist"}
	var staticDir string
	for _, s := range staticCandidates {
		if fi, err := os.Stat(s); err == nil && fi.IsDir() {
			staticDir = s
			break
		}
	}
	if staticDir == "" {
		log.Printf("warning: no static/dist directory found among candidates; static files may 404")
		staticDir = "static/dist"
	}

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

	addr := ":8080"
	log.Printf("Listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
