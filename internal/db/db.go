package db

import (
	"os"
	"time"

	_ "github.com/glebarez/sqlite"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

// OpenDB opens a DB connection. If DATABASE_URL env var is set it will use that
// otherwise it will create/ open a local sqlite file `data.db`.
func OpenDB() (*sqlx.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	var driver string
	if dsn == "" {
		driver = "sqlite"
		dsn = "file:data.db?_foreign_keys=1"
	} else {
		// assume postgres URL for production
		driver = "pgx"
	}
	db, err := sqlx.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)

	return db, nil
}
