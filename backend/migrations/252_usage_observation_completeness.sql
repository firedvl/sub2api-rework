-- Historical and uninstrumented observations remain unknown; never infer completion from charges.
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS usage_observation_complete BOOLEAN;

COMMENT ON COLUMN usage_logs.usage_observation_complete IS
    'Gateway response observation: true complete, false incomplete, NULL unknown; not final provider spend';
