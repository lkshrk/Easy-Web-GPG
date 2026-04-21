package migrate

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// setupTestDB creates a fresh test database using SQLite
func setupTestDB(t *testing.T) (*sql.DB, string, func()) {
	// Create temp directory for test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open SQLite database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// SQLite connection string for migrate
	dbURL := "sqlite3://file:" + dbPath + "?_foreign_keys=1"

	cleanup := func() {
		db.Close()
		// TempDir will be cleaned up automatically
	}

	return db, dbURL, cleanup
}

func TestMigrationsExist(t *testing.T) {
	// Get repository root directory
	_, filePath, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(filePath), "..", "..")

	// Verify migration files exist
	migrationsDir := filepath.Join(repoRoot, "migrations", "sql")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("failed to read migrations directory: %v", err)
	}

	if len(entries) == 0 {
		t.Error("no migration files found")
	}

	// Check for expected migration files
	expectedFiles := map[string]bool{
		"000001_baseline.up.sql":   false,
		"000001_baseline.down.sql": false,
	}

	for _, entry := range entries {
		expectedFiles[entry.Name()] = true
	}

	for name, found := range expectedFiles {
		if !found {
			t.Errorf("expected migration file %s not found", name)
		}
	}
}

func TestSQLiteMigration(t *testing.T) {
	db, dbURL, cleanup := setupTestDB(t)
	defer cleanup()

	// Set DATABASE_URL to point to our test database
	oldDBURL := os.Getenv("DATABASE_URL")
	os.Setenv("DATABASE_URL", dbURL)
	defer os.Setenv("DATABASE_URL", oldDBURL)

	// Run migrations - this should work with SQLite
	err := RunMigrations()
	if err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// Verify schema_migrations table exists and is not dirty
	var version int
	var dirty bool
	err = db.QueryRow("SELECT version, dirty FROM schema_migrations").Scan(&version, &dirty)
	if err != nil {
		t.Fatalf("failed to query schema_migrations: %v", err)
	}

	if dirty {
		t.Error("schema_migrations is marked as dirty after migration")
	}

	// Verify all tables were created
	tables := []string{"keys", "secrets"}
	for _, table := range tables {
		var count int
		err = db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
		if err != nil {
			t.Errorf("table %s does not exist: %v", table, err)
		}
	}
}

func TestMigrationDirtyStateHandling(t *testing.T) {
	db, dbURL, cleanup := setupTestDB(t)
	defer cleanup()

	// Set DATABASE_URL
	oldDBURL := os.Getenv("DATABASE_URL")
	os.Setenv("DATABASE_URL", dbURL)
	defer os.Setenv("DATABASE_URL", oldDBURL)

	// Run initial migration
	err := RunMigrations()
	if err != nil {
		t.Fatalf("initial migration failed: %v", err)
	}

	// Manually set dirty state to simulate a failed migration
	_, err = db.Exec("UPDATE schema_migrations SET dirty = true")
	if err != nil {
		t.Fatalf("failed to set dirty state: %v", err)
	}

	// Run migrations again - should handle dirty state and recover
	err = RunMigrations()
	if err != nil {
		t.Fatalf("RunMigrations failed to handle dirty state: %v", err)
	}

	// Verify clean state
	var dirty bool
	err = db.QueryRow("SELECT dirty FROM schema_migrations").Scan(&dirty)
	if err != nil {
		t.Fatalf("failed to query schema_migrations: %v", err)
	}

	if dirty {
		t.Error("schema_migrations is still marked as dirty after recovery")
	}

	// Run migrations a third time - should not fail with "no migration found for version X"
	// This tests that we don't call m.Up() when already at latest version
	err = RunMigrations()
	if err != nil {
		t.Fatalf("RunMigrations failed when already at latest version: %v", err)
	}

	var dirty2 bool
	err = db.QueryRow("SELECT dirty FROM schema_migrations").Scan(&dirty2)
	if err != nil {
		t.Fatalf("failed to query schema_migrations: %v", err)
	}

	if dirty2 {
		t.Error("schema_migrations is marked as dirty after running at latest version")
	}
}
