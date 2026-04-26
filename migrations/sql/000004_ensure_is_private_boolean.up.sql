-- +migrate postgres-only
-- Ensure is_private is BOOLEAN. Migration 000003 may have been a no-op if the
-- table lived outside the 'public' schema (custom search_path, cloud providers)
-- because it hard-coded table_schema = 'public'. This migration uses udt_name
-- which is schema-independent and is the authoritative PostgreSQL type name.
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name  = 'keys'
      AND column_name = 'is_private'
      AND udt_name   != 'bool'
  ) THEN
    ALTER TABLE keys ALTER COLUMN is_private TYPE BOOLEAN USING (is_private != 0);
  END IF;
END $$;
