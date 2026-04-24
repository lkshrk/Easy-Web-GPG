package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	migrate "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// postgresOnlyMarker matches the same constant used in internal/db.
const postgresOnlyMarker = "-- +migrate postgres-only"

// RunMigrations uses golang-migrate for schema management.
// It reads DATABASE_URL; if unset, falls back to a local sqlite file.
//
// When running against SQLite, migration files marked with
// "-- +migrate postgres-only" are automatically excluded because they
// contain PostgreSQL-specific SQL (e.g. DO $$ ... $$ blocks).
func RunMigrations() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}

	dbURL := os.Getenv("DATABASE_URL")
	isSQLite := false
	if dbURL == "" {
		dbPath := cwd + "/data.db"
		dbURL = "sqlite3://file:" + dbPath + "?_foreign_keys=1"
		isSQLite = true
	} else {
		isSQLite = strings.HasPrefix(strings.ToLower(dbURL), "sqlite3://")
	}

	// Locate the migrations directory.
	migrationsDir := findMigrationsDir(cwd)

	// For SQLite, build a filtered temp directory that excludes postgres-only files.
	src := "file://" + migrationsDir
	var tmpCleanup func()
	if isSQLite {
		filtered, cleanup, err := filteredMigrationsDir(migrationsDir)
		if err != nil {
			return fmt.Errorf("prepare SQLite migrations: %w", err)
		}
		src = "file://" + filtered
		tmpCleanup = cleanup
	}
	if tmpCleanup != nil {
		defer tmpCleanup()
	}

	m, err := migrate.New(src, dbURL)
	if err != nil {
		return fmt.Errorf("migrate new: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("migrate version: %w", err)
	}

	if dirty {
		if err := m.Force(int(version)); err != nil {
			return fmt.Errorf("migrate force clear dirty at version %d: %w", version, err)
		}
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// sqliteCompatible rewrites PostgreSQL-specific DDL to SQLite-compatible SQL.
// Only a small set of substitutions is needed for this project's migrations.
func sqliteCompatible(sql string) string {
	replacements := [][2]string{
		{"SERIAL PRIMARY KEY", "INTEGER PRIMARY KEY"},
		{"BIGSERIAL PRIMARY KEY", "INTEGER PRIMARY KEY"},
		{"BOOLEAN NOT NULL DEFAULT FALSE", "INTEGER NOT NULL DEFAULT 0"},
		{"BOOLEAN NOT NULL DEFAULT TRUE", "INTEGER NOT NULL DEFAULT 1"},
		{"BOOLEAN DEFAULT FALSE", "INTEGER DEFAULT 0"},
		{"BOOLEAN DEFAULT TRUE", "INTEGER DEFAULT 1"},
	}
	for _, r := range replacements {
		sql = strings.ReplaceAll(sql, r[0], r[1])
	}
	return sql
}

// findMigrationsDir returns the absolute path to migrations/sql, searching
// up from cwd and also checking /migrations/sql (container environments).
func findMigrationsDir(cwd string) string {
	// Container path takes priority.
	if _, err := os.Stat("/migrations/sql"); err == nil {
		return "/migrations/sql"
	}
	// Walk up to 3 levels looking for the directory.
	dir := cwd
	for i := 0; i <= 3; i++ {
		candidate := filepath.Join(dir, "migrations", "sql")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		dir = filepath.Dir(dir)
	}
	// Fallback: assume cwd/migrations/sql.
	return filepath.Join(cwd, "migrations", "sql")
}

// filteredMigrationsDir creates a temporary directory containing copies of
// migration files with postgres-only files removed. Returns the directory
// path and a cleanup function.
func filteredMigrationsDir(src string) (string, func(), error) {
	tmp, err := os.MkdirTemp("", "migrations-sqlite-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp migrations dir: %w", err)
	}
	cleanup := func() { os.RemoveAll(tmp) }

	entries, err := os.ReadDir(src)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("read migrations dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			cleanup()
			return "", nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		content := string(data)

		// Skip postgres-only migration files for SQLite.
		if strings.Contains(content, postgresOnlyMarker) {
			// Write an empty placeholder so golang-migrate sees a consistent
			// sequence of versions (no gaps).
			content = "-- skipped (postgres-only)\n"
		} else {
			// Translate PostgreSQL-specific syntax to SQLite equivalents.
			content = sqliteCompatible(content)
		}
		data = []byte(content)
		dest := filepath.Join(tmp, e.Name())
		if err := os.WriteFile(dest, data, 0o600); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("write %s: %w", e.Name(), err)
		}
	}
	return tmp, cleanup, nil
}
