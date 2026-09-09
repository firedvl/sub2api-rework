-- Add request enforcement independently from the pre-existing display-only
-- models_list_config. Policy cannot be reconstructed from display selections.
DO $$
DECLARE
    display_checksum TEXT;
    latest_checksum TEXT;
BEGIN
    LOCK TABLE groups IN ACCESS EXCLUSIVE MODE;

    SELECT checksum INTO display_checksum
      FROM schema_migrations WHERE filename = '143_group_models_list_config.sql';
    SELECT checksum INTO latest_checksum
      FROM schema_migrations WHERE filename = '239_group_free_openai_fast.sql';

    IF display_checksum IS DISTINCT FROM 'b0a2cac2567db903a8967456fff348f59530c2633b4dae363c32a0e3b6503cb3'
       OR latest_checksum IS DISTINCT FROM '80925a7deb54ef23ed5fae1eb1e519764d3952686be7101e2cf739028269f9c3' THEN
        RAISE EXCEPTION 'cannot add groups.model_allowlist: migration provenance is unknown; restore a verified Rework backup before retrying';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_attribute
        WHERE attrelid = 'groups'::regclass
          AND attname = 'models_list_config'
          AND atttypid = 'jsonb'::regtype
          AND attnotnull
          AND NOT attisdropped
    ) THEN
        RAISE EXCEPTION 'cannot add groups.model_allowlist: display-only models_list_config is missing; restore a verified Rework backup before retrying';
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_attribute
        WHERE attrelid = 'groups'::regclass AND attname = 'model_allowlist' AND NOT attisdropped
    ) THEN
        RAISE EXCEPTION 'cannot add groups.model_allowlist: preexisting enforcement policy has unknown provenance; restore a verified backup explicitly';
    END IF;
END
$$;

ALTER TABLE groups
    ADD COLUMN model_allowlist JSONB NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN groups.model_allowlist IS
    'Group model allowlist: constrains model listing responses and request admission';
