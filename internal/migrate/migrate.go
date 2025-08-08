package migrate

import (
	"fmt"
	"os"

	migrate "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// RunMigrations runs SQL migrations using golang-migrate reading files from migrations/sql.
// It reads DATABASE_URL; if unset, it uses sqlite at file:data.db
func RunMigrations() error {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// sqlite3 URL for golang-migrate: sqlite3://file:data.db?cache=shared&_foreign_keys=1
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getwd: %w", err)
		}
		dbPath := cwd + "/data.db"
		dbURL = "sqlite3://file:" + dbPath + "?_foreign_keys=1"
	}

	// prefer absolute path if present (useful in container runtime where files are at /migrations/sql)
	src := "file://migrations/sql"
	if _, err := os.Stat("/migrations/sql"); err == nil {
		src = "file:///migrations/sql"
	}

	m, err := migrate.New(src, dbURL)
	if err != nil {
		return fmt.Errorf("migrate new: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
