//go:build unit

package admin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdateProxyRequestCredentialPresence(t *testing.T) {
	for _, tc := range []struct {
		name, body         string
		username, password *string
	}{
		{name: "omitted", body: `{}`},
		{name: "null", body: `{"username":null,"password":null}`},
		{name: "clear", body: `{"username":"","password":""}`, username: ptrString(""), password: ptrString("")},
		{name: "replace", body: `{"username":"user","password":"pass"}`, username: ptrString("user"), password: ptrString("pass")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var req UpdateProxyRequest
			require.NoError(t, json.Unmarshal([]byte(tc.body), &req))
			require.Equal(t, tc.username, req.Username)
			require.Equal(t, tc.password, req.Password)
		})
	}
}

func ptrString(s string) *string { return &s }

func TestUpdateProxyRequestSettingsPresence(t *testing.T) {
	var omitted, cleared, set UpdateProxyRequest
	require.NoError(t, json.Unmarshal([]byte(`{"status":"inactive"}`), &omitted))
	require.False(t, omitted.ExpiresAt.Set)
	require.False(t, omitted.BackupProxyID.Set)
	require.Nil(t, omitted.ExpiryWarnDays)
	require.NoError(t, json.Unmarshal([]byte(`{"expires_at":null,"backup_proxy_id":null,"expiry_warn_days":0}`), &cleared))
	require.True(t, cleared.ExpiresAt.Set)
	require.Nil(t, cleared.ExpiresAt.Value)
	require.True(t, cleared.BackupProxyID.Set)
	require.Nil(t, cleared.BackupProxyID.Value)
	require.NotNil(t, cleared.ExpiryWarnDays)
	require.Zero(t, *cleared.ExpiryWarnDays)
	require.NoError(t, json.Unmarshal([]byte(`{"expires_at":123,"backup_proxy_id":10}`), &set))
	require.EqualValues(t, 123, *set.ExpiresAt.Value)
	require.EqualValues(t, 10, *set.BackupProxyID.Value)
}
