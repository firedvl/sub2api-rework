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
