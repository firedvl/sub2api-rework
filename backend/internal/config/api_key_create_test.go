package config

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAPIKeyCreateDefaultsAndValidation(t *testing.T) {
	resetViperWithJWTSecret(t)
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, 200, cfg.APIKeyCreate.MaxActivePerUser)
	require.Equal(t, 60, cfg.APIKeyCreate.MaxPerUserPerHour)
	cfg.APIKeyCreate.MaxActivePerUser = -1
	require.ErrorContains(t, cfg.Validate(), "api_key_create.max_active_per_user")
	cfg.APIKeyCreate.MaxActivePerUser = 0
	cfg.APIKeyCreate.MaxPerUserPerHour = -1
	require.ErrorContains(t, cfg.Validate(), "api_key_create.max_per_user_per_hour")
	cfg.APIKeyCreate.MaxPerUserPerHour = 0
	require.NoError(t, cfg.Validate())
}
