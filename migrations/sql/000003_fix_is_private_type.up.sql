-- +migrate postgres-only
-- Convert is_private from INTEGER (int4) to BOOLEAN on deployments that were
-- created before the column type was standardised. No-op if already BOOLEAN.
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public'
      AND table_name   = 'keys'
      AND column_name  = 'is_private'
      AND data_type    IN ('integer', 'smallint', 'bigint')
  ) THEN
    ALTER TABLE keys ALTER COLUMN is_private TYPE BOOLEAN USING (is_private = 1);
  END IF;
END $$;
