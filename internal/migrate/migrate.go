package migrate

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	migrate "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// dropSchemaMigrationsTable drops the schema_migrations table to reset migration state
func dropSchemaMigrationsTable(dbURL string) error {
	// Extract driver type from database URL
	var driverName string
	if strings.HasPrefix(dbURL, "postgres://") || strings.HasPrefix(dbURL, "postgresql://") {
		driverName = "postgres"
	} else if strings.HasPrefix(dbURL, "sqlite3://") {
		driverName = "sqlite3"
	} else {
		return fmt.Errorf("unsupported database URL format")
	}

	db, err := sql.Open(driverName, strings.TrimPrefix(strings.TrimPrefix(dbURL, "sqlite3://file:"), "postgres://"))
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Drop the schema_migrations table
	_, err = db.Exec("DROP TABLE IF EXISTS schema_migrations")
	if err != nil {
		return fmt.Errorf("failed to drop schema_migrations table: %w", err)
	}

	return nil
}

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

	// Get current version to check if we need to force version
	_, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("migrate version: %w", err)
	}

	// If dirty, we need to fix by clearing the dirty flag
	// This happens when a migration fails mid-transaction
	if dirty {
		// Close the migrate instance first
		m.Close()

		// Extract driver type from database URL
		var driverName string
		if strings.HasPrefix(dbURL, "postgres://") || strings.HasPrefix(dbURL, "postgresql://") {
			driverName = "postgres"
		} else if strings.HasPrefix(dbURL, "sqlite3://") {
			driverName = "sqlite3"
		} else {
			return fmt.Errorf("unsupported database URL format")
		}

		// Parse database URL for sql.Open
		dbConnStr := strings.TrimPrefix(dbURL, "sqlite3://file:")
		dbConnStr = strings.TrimPrefix(dbConnStr, "postgres://")
		dbConnStr = strings.TrimPrefix(dbConnStr, "postgresql://")

		db, err := sql.Open(driverName, dbConnStr)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer db.Close()

		// Clear the dirty flag
		_, err = db.Exec("UPDATE schema_migrations SET dirty = false")
		if err != nil {
			return fmt.Errorf("failed to clear dirty flag: %w", err)
		}

		// Recreate migrate instance with clean state
		m, err = migrate.New(src, dbURL)
		if err != nil {
			return fmt.Errorf("migrate new: %w", err)
		}
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
