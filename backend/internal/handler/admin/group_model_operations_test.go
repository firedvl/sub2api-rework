package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
	accounts []service.Account
}

func (r modelOperationsAccountRepo) ListModelAvailabilityCandidates(context.Context, *int64, []string, bool) ([]service.Account, error) {
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
		modelOperationsAccountRepo{accounts: []service.Account{account}},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	handler := NewGroupHandler(modelOperationsAdminService{group: group}, nil, nil, gateway, nil, nil)
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
