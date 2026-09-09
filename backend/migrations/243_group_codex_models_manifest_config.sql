ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS codex_models_manifest_config JSONB NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN groups.codex_models_manifest_config IS
    'Pinned account configuration for fetching a group Codex models manifest';
