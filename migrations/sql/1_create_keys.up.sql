-- migration: create keys table (sqlite)
CREATE TABLE IF NOT EXISTS keys (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  armored TEXT NOT NULL,
  is_private BOOLEAN NOT NULL DEFAULT 0,
  encrypted_password TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
