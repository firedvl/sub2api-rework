package admin

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type modelOperationsAdminService struct {
	service.AdminService
	group *service.Group
}

func (s modelOperationsAdminService) GetGroup(context.Context, int64) (*service.Group, error) {
	return s.group, nil
}

type modelOperationsAccountRepo struct {
	service.AccountRepository
	accounts        []service.Account
	configuredCalls int
	liveCalls       int
}

func (r *modelOperationsAccountRepo) ListModelAvailabilityCandidates(context.Context, *int64, []string, bool) ([]service.Account, error) {
	r.configuredCalls++
	return append([]service.Account(nil), r.accounts...), nil
}

func TestGetModelOperationsUsesProviderBackedCatalogWithoutSerializingSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	group := &service.Group{ID: 42, Name: "OpenAI", Platform: service.PlatformOpenAI, Status: service.StatusActive}
	account := service.Account{
		ID: 7, Platform: service.PlatformOpenAI, Status: service.StatusActive, Schedulable: true,
		Credentials: map[string]any{
			"access_token":  "must-not-serialize",
			"model_mapping": map[string]any{"gpt-6-astra": "gpt-6-astra"},
		},
	}
	gateway := service.NewGatewayService(
		&modelOperationsAccountRepo{accounts: []service.Account{account}},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	handler := NewGroupHandler(modelOperationsAdminService{group: group}, nil, nil, gateway, nil)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: "42"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/groups/42/model-operations", nil)

	handler.GetModelOperations(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "must-not-serialize")
	var body struct {
		Data modelOperationsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Len(t, body.Data.Models, 1)
	model := body.Data.Models[0]
	require.Equal(t, "gpt-6-astra", model.PublicID)
	require.Equal(t, service.PlatformOpenAI, model.ActualPlatform)
	require.True(t, model.CatalogMember)
	require.True(t, model.V1ModelsVisible)
	require.True(t, model.CodexPickerVisible)
	require.Equal(t, int64(0), model.RecentRequestCount)
}

func TestApplyModelOperationsUsageIncludesPassiveSpeed(t *testing.T) {
	latencyP50, latencyP95 := int64(120), int64(480)
	ttftP50, ttftP95 := int64(30), int64(90)
	throughput := 42.5
	var model modelOperationsModel

	applyModelOperationsUsage(&model, usagestats.ModelStat{
		Requests: 7, InputTokens: 100, OutputTokens: 200, TotalTokens: 300,
		LatencyP50Ms: &latencyP50, LatencyP95Ms: &latencyP95,
		TTFTP50Ms: &ttftP50, TTFTP95Ms: &ttftP95,
		OutputTokensPerSecond: &throughput, TimingSampleCount: 5,
	})

	require.Equal(t, int64(7), model.RecentRequestCount)
	require.Equal(t, int64(120), *model.LatencyP50Ms)
	require.Equal(t, int64(480), *model.LatencyP95Ms)
	require.Equal(t, int64(30), *model.TTFTP50Ms)
	require.Equal(t, int64(90), *model.TTFTP95Ms)
	require.Equal(t, 42.5, *model.OutputTokensPerSecond)
	require.Equal(t, int64(5), model.TimingSampleCount)
}

func (r *modelOperationsAccountRepo) ListSchedulableByGroupID(context.Context, int64) ([]service.Account, error) {
	r.liveCalls++
	return nil, fmt.Errorf("live discovery must not run in Model Operations")
}

func TestGetModelOperationsUsesStoredManifestWithoutLiveDiscovery(t *testing.T) {
	for _, state := range []string{"fresh", "stale", "wrong_identity", "alias"} {
		t.Run(state, func(t *testing.T) {
			group := &service.Group{ID: 42, Platform: service.PlatformOpenAI}
			now := time.Now()
			until := now.Add(time.Hour)
			identity := sha256.Sum256([]byte("chatgpt:synthetic-provider-identity"))
			syncedAt := now
			if state == "stale" {
				syncedAt = now.Add(-48 * time.Hour)
			}
			namespace := fmt.Sprintf("%x", identity[:16])
			if state == "wrong_identity" {
				namespace = "different-identity"
			}
			account := service.Account{
				ID: 7, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth,
				Status: service.StatusActive, Schedulable: true, RateLimitResetAt: &until,
				Credentials: map[string]any{"chatgpt_account_id": "synthetic-provider-identity"},
				Extra: map[string]any{service.OpenAICodexManifestSnapshotExtraKey: map[string]any{
					"identity": namespace,
					"versions": map[string]any{"test": map[string]any{
						"synced_at": syncedAt.UTC().Format(time.RFC3339Nano),
						"body":      json.RawMessage(`{"models":[{"slug":"future-model-from-observation","supported_in_api":true}]}`),
					}},
				}},
			}
			publicModel := "future-model-from-observation"
			if state == "alias" {
				publicModel = "public-alias"
				account.Credentials["model_mapping"] = map[string]any{publicModel: "future-model-from-observation"}
			}
			repo := &modelOperationsAccountRepo{accounts: []service.Account{account}}
			before, err := json.Marshal(repo.accounts)
			require.NoError(t, err)
			gateway := service.NewGatewayService(repo,
				nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
				nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			handler := NewGroupHandler(modelOperationsAdminService{group: group}, nil, nil, gateway, nil)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Params = gin.Params{{Key: "id", Value: "42"}}
			c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/groups/42/model-operations", nil)
			handler.GetModelOperations(c)
			require.Equal(t, http.StatusOK, recorder.Code)
			require.Zero(t, repo.liveCalls)
			require.Equal(t, 1, repo.configuredCalls)
			var body struct {
				Data modelOperationsResponse `json:"data"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
			found := false
			for _, model := range body.Data.Models {
				if model.PublicID == publicModel {
					found = true
					require.True(t, model.Discovered)
					require.Equal(t, "provider_discovery", model.DiscoverySource)
					require.True(t, model.CodexPickerVisible)
					require.True(t, model.V1ModelsVisible)
				}
			}
			require.Equal(t, state == "fresh" || state == "alias", found)
			require.Empty(t, body.Data.Warning)
			after, err := json.Marshal(repo.accounts)
			require.NoError(t, err)
			require.JSONEq(t, string(before), string(after))
		})
	}
}
