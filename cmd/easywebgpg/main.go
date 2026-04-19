package main

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	// Run golang-migrate migrations for PostgreSQL only.
	if os.Getenv("DATABASE_URL") != "" {
		if err := migratepkg.RunMigrations(); err != nil {
			log.Fatalf("migration error: %v", err)
		}
	}

	db, err := dbpkg.OpenDB()
	if err != nil {
		log.Fatalf("db open: %v", err)
	}

	// Apply simple SQL migrations (SQLite development fallback).
	migrationPath := findDirectory("migrations", []string{"migrations/sql", "./migrations/sql", "../migrations/sql", "/migrations/sql"})
	if err := dbpkg.ApplySQLMigrations(db, migrationPath); err != nil {
		log.Printf("migration error (dev): %v", err)
	}

	cryptoSvc := cm.NewCryptoService(db)
	tmpl := loadTemplates()

	staticDir := findDirectory("static", []string{"static", "./static", "../static", "../../static", "/static"})
	fsHandler := http.FileServer(http.Dir(staticDir))

	a := &app.App{
		DB:             db,
		Templates:      tmpl,
		Crypto:         cryptoSvc,
		MasterPassword: os.Getenv("MASTER_PASSWORD"),
	}

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", fsHandler))
	mux.HandleFunc("/time", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(time.Now().Format("15:04:05 MST — Jan 2 2006")))
	})
	mux.HandleFunc("/auth", app.RateLimit(app.AuthRateLimiter, a.AuthHandler))
	mux.HandleFunc("/logout", a.LogoutHandler)

	mux.HandleFunc("/", a.WithAuth(a.IndexHandler))
	mux.HandleFunc("/keys", a.WithAuth(a.AddKeyHandler))
	mux.HandleFunc("/keys/view", a.WithAuth(a.ViewKeyHandler))
	mux.HandleFunc("/keys/delete", a.WithAuth(a.DeleteKeyHandler))
	mux.HandleFunc("/encrypt", a.WithAuth(a.EncryptHandler))
	mux.HandleFunc("/decrypt", a.WithAuth(a.DecryptHandler))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port

	srv := &http.Server{
		Addr:         addr,
		Handler:      app.RequestLogger(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown on SIGINT/SIGTERM.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		log.Printf("Received %s, shutting down...", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
		db.Close()
	}()

	log.Printf("Listening on %s", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
