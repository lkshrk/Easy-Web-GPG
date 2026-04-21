-- baseline migration: initial schema (compatible with both SQLite and PostgreSQL)
-- Note: INTEGER PRIMARY KEY is auto-increment in SQLite, PostgreSQL accepts it
CREATE TABLE IF NOT EXISTS keys (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  armored TEXT NOT NULL,
  is_private INTEGER NOT NULL DEFAULT 0,
  encrypted_password TEXT,
  password_bcrypt TEXT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS secrets (
  name TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
