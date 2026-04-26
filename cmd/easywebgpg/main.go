package main

import (
	"context"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"h-cloud.io/web-gpg/internal/app"
	cm "h-cloud.io/web-gpg/internal/crypto"
	dbpkg "h-cloud.io/web-gpg/internal/db"
	migratepkg "h-cloud.io/web-gpg/internal/migrate"
)

func initLogger() {
	level := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}
	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if os.Getenv("LOG_FORMAT") == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(handler))
}

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
	slog.Warn("directory not found, using default", "name", name, "default", candidates[0])
	return candidates[0]
}

// loadTemplates loads all HTML templates.
func loadTemplates() *template.Template {
	indexCandidates := []string{"templates/index.html", "./templates/index.html", "../templates/index.html", "../../templates/index.html", "/templates/index.html"}
	loginCandidates := []string{"templates/login.html", "./templates/login.html", "../templates/login.html", "../../templates/login.html", "/templates/login.html"}

	indexPath := findFile(indexCandidates)
	if indexPath == "" {
		slog.Error("templates/index.html not found in candidate paths")
		os.Exit(1)
	}

	files := []string{indexPath}
	if loginPath := findFile(loginCandidates); loginPath != "" {
		files = append(files, loginPath)
	}

	tmpl := template.Must(template.ParseFiles(files...))
	slog.Debug("templates loaded", "files", files)
	return tmpl
}

func main() {
	initLogger()

	dbURL := os.Getenv("DATABASE_URL")
	isPostgres := dbURL != "" && !strings.HasPrefix(strings.ToLower(dbURL), "sqlite://")

	dbMode := "sqlite"
	if isPostgres {
		dbMode = "postgres"
		if err := migratepkg.RunMigrations(); err != nil {
			slog.Error("migration failed", "err", err)
			os.Exit(1)
		}
	}

	db, err := dbpkg.OpenDB()
	if err != nil {
		slog.Error("failed to open database", "err", err)
		os.Exit(1)
	}

	// Apply simple SQL migrations for SQLite only.
	// PostgreSQL uses golang-migrate (RunMigrations above) with proper version tracking.
	if !isPostgres {
		migrationPath := findDirectory("migrations", []string{"migrations/sql", "./migrations/sql", "../migrations/sql", "/migrations/sql"})
		if err := dbpkg.ApplySQLMigrations(db, migrationPath); err != nil {
			slog.Warn("dev migration error", "err", err)
		}
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

	authEnabled := os.Getenv("MASTER_PASSWORD") != ""
	slog.Info("starting server", "addr", addr, "db", dbMode, "auth", authEnabled)

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
		slog.Info("shutting down", "signal", sig.String())
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			slog.Error("shutdown error", "err", err)
		}
		db.Close()
	}()

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}
