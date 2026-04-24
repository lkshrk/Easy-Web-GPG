-- +migrate postgres-only
-- Ensure keys.id auto-increments via a sequence in PostgreSQL.
-- This is a no-op when the column already has a nextval() default (e.g. was
-- originally created with SERIAL PRIMARY KEY).
-- Needed to fix deployments created while the migration temporarily used
-- INTEGER PRIMARY KEY (which has no auto-increment in PostgreSQL).
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public'
      AND table_name   = 'keys'
      AND column_name  = 'id'
      AND column_default LIKE 'nextval%'
  ) THEN
    CREATE SEQUENCE IF NOT EXISTS keys_id_seq;
    PERFORM setval('keys_id_seq', COALESCE((SELECT MAX(id) FROM keys), 0) + 1, false);
    ALTER TABLE keys ALTER COLUMN id SET DEFAULT nextval('keys_id_seq');
  END IF;
END $$;
