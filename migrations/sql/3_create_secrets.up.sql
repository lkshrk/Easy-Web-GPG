-- migration: create secrets table for storing small values like master_salt
CREATE TABLE IF NOT EXISTS secrets (
  name TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

