package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGatewayModelsMixedGeminiPreservesCallerAndAllowlistScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	accounts := func(model string) []service.Account {
		return []service.Account{{ID: 1, Platform: service.PlatformAntigravity,
			Extra:       map[string]any{"mixed_scheduling": true},
			Credentials: map[string]any{"model_mapping": map[string]any{model: "gemini-provider", "claude-hidden": "claude-provider"}}}}
	}
	handler := newGatewayModelsHandlerForTest(&gatewayModelsAccountRepoStub{
		catalogByGroup: map[int64][]service.Account{41: accounts("gemini-caller-one"), 42: accounts("gemini-caller-two")},
	})
	for _, scenario := range []struct {
		groupID   int64
		allowlist service.GroupModelAllowlist
		want      []string
	}{
		{41, service.GroupModelAllowlist{Enabled: true, Models: []string{"gemini-caller-one"}}, []string{"gemini-caller-one"}},
		{42, service.GroupModelAllowlist{Enabled: true, Models: []string{"gemini-caller-two"}}, []string{"gemini-caller-two"}},
		{41, service.GroupModelAllowlist{Enabled: true, Models: []string{"gemini-caller-two"}}, []string{}},
	} {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		context.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{GroupID: &scenario.groupID,
			Group: &service.Group{ID: scenario.groupID, Platform: service.PlatformGemini, ModelAllowlist: scenario.allowlist}})
		handler.Models(context)
		require.Equal(t, http.StatusOK, recorder.Code)
		var response gatewayModelsResponseForTest
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
		ids := make([]string, 0, len(response.Data))
		for _, model := range response.Data {
			ids = append(ids, model.ID)
		}
		require.Equal(t, scenario.want, ids)
	}
}
