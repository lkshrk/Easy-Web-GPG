-- migration: create keys table
CREATE TABLE IF NOT EXISTS keys (
  id SERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  armored TEXT NOT NULL,
  is_private BOOLEAN NOT NULL DEFAULT FALSE,
  encrypted_password TEXT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
