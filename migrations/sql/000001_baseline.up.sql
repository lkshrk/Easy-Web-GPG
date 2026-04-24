-- baseline migration: initial schema
-- SERIAL PRIMARY KEY is correct PostgreSQL syntax for auto-incrementing IDs.
-- SQLite does not recognise SERIAL; the Go layer (repairSQLiteSchema) detects
-- this and recreates the table using INTEGER PRIMARY KEY on first startup.
CREATE TABLE IF NOT EXISTS keys (
  id SERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  armored TEXT NOT NULL,
  is_private BOOLEAN NOT NULL DEFAULT FALSE,
  encrypted_password TEXT,
  password_bcrypt TEXT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS secrets (
  name TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
