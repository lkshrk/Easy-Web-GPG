-- +migrate postgres-only
ALTER TABLE keys ALTER COLUMN is_private TYPE INTEGER USING is_private::INTEGER;
