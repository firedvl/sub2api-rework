package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCyberAllowlistSettingsAuditNamesFieldWithoutUserValues(t *testing.T) {
	before := &service.SystemSettings{CyberPolicyUserAllowlist: "12"}
	after := &service.SystemSettings{CyberPolicyUserAllowlist: "34,56"}
	require.Equal(t, []string{"cyber_policy_user_allowlist"}, diffSettings(before, after, nil, nil, UpdateSettingsRequest{}))
	require.Empty(t, diffSettings(after, after, nil, nil, UpdateSettingsRequest{}))
}
