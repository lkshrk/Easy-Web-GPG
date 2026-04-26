-- +migrate postgres-only
-- Revert is_private from BOOLEAN back to INTEGER.
ALTER TABLE keys ALTER COLUMN is_private TYPE INTEGER USING is_private::INTEGER;
