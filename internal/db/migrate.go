package db

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jmoiron/sqlx"
)

// postgresOnlyMarker marks a migration as PostgreSQL-only and causes
// ApplySQLMigrations to skip it when connected to SQLite.
const postgresOnlyMarker = "-- +migrate postgres-only"

// ApplySQLMigrations is a simple SQL file executor for SQLite development
// environments. For PostgreSQL production deployments, use the golang-migrate
// based RunMigrations in internal/migrate instead.
//
// Files containing the line "-- +migrate postgres-only" are skipped on SQLite.
// After all files run, repairSQLiteSchema fixes any SERIAL-column artefacts.
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
		content := string(b)

		// Skip migrations marked as PostgreSQL-only when running on SQLite.
		if strings.Contains(content, postgresOnlyMarker) {
			continue
		}

		if _, err := db.Exec(content); err != nil {
			// Ignore benign "already exists" errors when a migration was already applied.
			if strings.Contains(err.Error(), "duplicate column") ||
				strings.Contains(err.Error(), "already exists") {
				continue
			}
			return fmt.Errorf("running %s: %w", f, err)
		}
	}

	// Repair SQLite schema artefacts produced by PostgreSQL-syntax migrations
	// (e.g. SERIAL PRIMARY KEY → INTEGER PRIMARY KEY). No-op on PostgreSQL.
	if err := repairSQLiteSchema(db); err != nil {
		log.Printf("warning: SQLite schema repair: %v", err)
	}
	return nil
}

// repairSQLiteSchema detects and fixes a stale SQLite schema where the keys
// table was created with PostgreSQL syntax (SERIAL PRIMARY KEY instead of
// INTEGER PRIMARY KEY). In that case rows end up with NULL ids, which breaks
// scanning. It recreates the table with the correct schema and copies all rows.
//
// This is a no-op when the schema is already correct or when connected to
// PostgreSQL (PRAGMA is not valid SQL there and returns an error, which we
// interpret as "nothing to do").
func repairSQLiteSchema(db *sqlx.DB) error {
	type colInfo struct {
		CID       int     `db:"cid"`
		Name      string  `db:"name"`
		ColType   string  `db:"type"`
		NotNull   int     `db:"notnull"`
		DfltValue *string `db:"dflt_value"` // nullable
		PK        int     `db:"pk"`
	}

	var cols []colInfo
	if err := db.Select(&cols, "PRAGMA table_info(keys)"); err != nil || len(cols) == 0 {
		// Not SQLite, or table doesn't exist yet — nothing to do.
		return nil
	}

	// Find the id column and check its type.
	idType := ""
	for _, c := range cols {
		if strings.EqualFold(c.Name, "id") {
			idType = c.ColType
			break
		}
	}
	if strings.EqualFold(idType, "INTEGER") {
		return nil // already correct
	}

	log.Printf("INFO: SQLite keys.id has type %q instead of INTEGER — repairing schema", idType)

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin repair tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS keys_repair (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			armored TEXT NOT NULL,
			is_private INTEGER NOT NULL DEFAULT 0,
			encrypted_password TEXT,
			password_bcrypt TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		// Normalise is_private regardless of how it was stored (boolean text vs int).
		`INSERT INTO keys_repair (name, armored, is_private, encrypted_password, password_bcrypt, created_at)
		 SELECT name, armored,
		        CASE WHEN is_private IN (1, TRUE, 'true', 'TRUE', 'True') THEN 1 ELSE 0 END,
		        encrypted_password, password_bcrypt, created_at
		 FROM keys`,
		`DROP TABLE keys`,
		`ALTER TABLE keys_repair RENAME TO keys`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("schema repair step failed: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit repair tx: %w", err)
	}
	log.Printf("INFO: SQLite keys table schema repaired successfully")
	return nil
}
