package db

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"

	"github.com/jmoiron/sqlx"
)

// ApplySQLMigrations applies SQL files in migrations/sql in ascending order.
func ApplySQLMigrations(db *sqlx.DB, migrationsDir string) error {
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, f := range files {
		b, err := ioutil.ReadFile(f)
		if err != nil {
			return err
		}
		if _, err := db.Exec(string(b)); err != nil {
			return fmt.Errorf("running %s: %w", f, err)
		}
	}
	return nil
}
