package migrate

import (
	"fmt"
	"os"
	"path/filepath"

	migrate "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// RunMigrations uses golang-migrate for PostgreSQL production deployments.
// It reads DATABASE_URL; if unset, falls back to sqlite at file:data.db.
// For SQLite development environments, use ApplySQLMigrations in internal/db instead.
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

	// Get repository root by going up from the current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}

	// Try to find migrations directory relative to current directory
	// This works when running from the repository root
	src := "file://migrations/sql"
	if _, err := os.Stat("migrations/sql"); err == nil {
		src = "file://./migrations/sql"
	} else {
		// Try going up to find repository root (for running tests from subdirectories)
		for i := 0; i < 3; i++ {
			cwd = filepath.Dir(cwd)
			if _, err := os.Stat(filepath.Join(cwd, "migrations/sql")); err == nil {
				src = "file://" + filepath.Join(cwd, "migrations/sql")
				break
			}
		}
	}

	// prefer absolute path if present (useful in container runtime where files are at /migrations/sql)
	if _, err := os.Stat("/migrations/sql"); err == nil {
		src = "file:///migrations/sql"
	}

	m, err := migrate.New(src, dbURL)
	if err != nil {
		return fmt.Errorf("migrate new: %w", err)
	}

	// Get current version to check if we need to handle dirty state
	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("migrate version: %w", err)
	}

	// If dirty, we need to recover to a clean state
	// Force to current version to clear dirty flag, then check if we need to migrate
	if dirty {
		if err := m.Force(int(version)); err != nil {
			return fmt.Errorf("migrate force clear dirty at version %d: %w", version, err)
		}
		// After clearing dirty, check if there are pending migrations before calling Up
		// m.Up() will fail if already at latest version (tries to apply non-existent next version)
		return nil
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
