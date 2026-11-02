package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGatewayModelsClaudeOfficialNames(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const groupID = int64(301)
	h := newGatewayModelsHandlerForTest(&gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
		groupID: {
			{ID: 1, Platform: service.PlatformAnthropic},
			{ID: 2, Platform: service.PlatformAntigravity, Credentials: map[string]any{"model_mapping": map[string]any{
				"ag-claude-sonnet": "claude-sonnet-4-5", "claude-sonnet-4-5-thinking": "claude-sonnet-4-5-thinking",
				"claude-haiku-4-5": "claude-haiku-4-5", "gemini-2.5-flash": "gemini-2.5-flash",
			}}},
		},
	}})
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{Group: &service.Group{ID: groupID, Platform: service.PlatformComposite}})
	h.Models(c)
	require.Equal(t, http.StatusOK, rec.Code)
	var got struct {
		Data []claude.Model `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	byID := make(map[string]claude.Model)
	for _, model := range got.Data {
		require.NotContains(t, byID, model.ID)
		byID[model.ID] = model
	}
	for id, name := range map[string]string{"claude-haiku-5-5": "Claude Haiku 5.5", "claude-sonnet-5-5": "Claude Sonnet 5.5", "claude-haiku-4-5": "Claude Haiku 4.5"} {
		require.Equal(t, name, byID[id].DisplayName, id)
	}
	require.Contains(t, byID, "gemini-2.5-flash")
	for _, id := range []string{"ag-claude-sonnet", "claude-sonnet-4-5-thinking", "claude-haiku-4-5-20251001"} {
		require.NotContains(t, byID, id)
	}
	for _, id := range []string{"claude-haiku-5-5", "claude-sonnet-5-5", "claude-haiku-4-5"} {
		require.Equal(t, http.StatusOK, requestModelForTest(h, &service.Group{ID: groupID, Platform: service.PlatformComposite}, id, "").Code)
	}
	for _, path := range []string{"/models?client_version=0.147.0", "/v1/gateway/capabilities", "/v1/gateway/capabilities?schema_version=2"} {
		rec = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, path, nil)
		platform := service.PlatformComposite
		if path == "/v1/gateway/capabilities?schema_version=2" {
			platform = service.PlatformAnthropic
		}
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{Group: &service.Group{ID: groupID, Platform: platform}})
		if path == "/models?client_version=0.147.0" {
			h.CodexModels(c)
		} else {
			h.Capabilities(c)
		}
		require.Equal(t, http.StatusOK, rec.Code, path)
		require.Contains(t, rec.Body.String(), "claude-haiku-5-5", path)
		require.NotContains(t, rec.Body.String(), "ag-claude-sonnet", path)
		require.NotContains(t, rec.Body.String(), "claude-sonnet-4-5-thinking", path)
		require.NotContains(t, rec.Body.String(), "claude-haiku-4-5-20251001", path)
	}
}
