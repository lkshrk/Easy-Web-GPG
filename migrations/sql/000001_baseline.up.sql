-- baseline migration: initial schema
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
