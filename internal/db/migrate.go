package db

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jmoiron/sqlx"
)

// ApplySQLMigrations is a simple SQL file executor for SQLite development
// environments. For PostgreSQL production deployments, use the golang-migrate
// based RunMigrations in internal/migrate instead.
func ApplySQLMigrations(db *sqlx.DB, migrationsDir string) error {
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		if _, err := db.Exec(string(b)); err != nil {
			// ignore benign sqlite errors like duplicate column when migration already applied
			if err != nil && (strings.Contains(err.Error(), "duplicate column") || strings.Contains(err.Error(), "already exists")) {
				continue
			}
			return fmt.Errorf("running %s: %w", f, err)
		}
	}
	return nil
}
