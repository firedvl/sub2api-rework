package handler

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGPT61PublicCatalogRequiresAccountEvidenceAndGroupAdmission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groupID := int64(620)
	for _, tc := range []struct {
		name                                     string
		discovered, restricted, denied, cooldown bool
		want                                     bool
	}{
		{name: "no-global-exposure"},
		{name: "observed-provider-model", discovered: true, want: true},
		{name: "account-restriction", discovered: true, restricted: true},
		{name: "group-allowlist", discovered: true, denied: true},
		{name: "cooldown-retains-membership", discovered: true, cooldown: true, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := service.Account{ID: 1, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"chatgpt_account_id": "synthetic-account"}}
			if tc.discovered {
				identity := sha256.Sum256([]byte("chatgpt:synthetic-account"))
				account.Extra = map[string]any{service.OpenAICodexManifestSnapshotExtraKey: map[string]any{
					"identity": fmt.Sprintf("%x", identity[:16]),
					"versions": map[string]any{"0.153.0": map[string]any{"synced_at": time.Now().UTC().Format(time.RFC3339Nano), "body": json.RawMessage(`{"models":[{"slug":"gpt-6.1-sol","visibility":"list"},{"slug":"gpt-6-sol","visibility":"list"}]}`)}},
				}}
			}
			if tc.restricted {
				account.Credentials["model_mapping"] = map[string]any{"gpt-6-sol": "gpt-6-sol"}
			}
			if tc.cooldown {
				until := time.Now().Add(time.Hour)
				account.RateLimitResetAt = &until
			}
			group := &service.Group{ID: groupID, Platform: service.PlatformOpenAI}
			if tc.denied {
				group.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"gpt-6-sol"}}
			}
			h := newGatewayModelsHandlerForTest(&gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{groupID: {account}}, catalogByGroup: map[int64][]service.Account{groupID: {account}}})
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
			c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{GroupID: &groupID, Group: group})
			h.Models(c)
			require.Equal(t, http.StatusOK, rec.Code)
			var response gatewayModelsResponseForTest
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
			present := false
			for _, model := range response.Data {
				if model.ID == "gpt-6.1-sol" {
					present = true
				}
			}
			require.Equal(t, tc.want, present)
		})
	}
}
