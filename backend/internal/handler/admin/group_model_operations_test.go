package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
