package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGatewayRoutesRetrievePinnedModel(test *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &pinnedModelsRoutesRepository{account: service.Account{
		ID: 7, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true,
		Credentials: map[string]any{"api_key": "test-models-key", "base_url": "https://models.example/v1"},
	}}
	upstream := &pinnedModelsRoutesUpstream{}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	gatewayService := service.NewOpenAIGatewayService(repo, nil, nil, nil, nil, nil, nil, cfg,
		nil, nil, nil, nil, nil, upstream, nil, nil, nil, nil, nil, nil, nil, nil)
	handlers := &handler.Handlers{
		Gateway:       handler.NewGatewayHandler(nil, gatewayService, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, cfg, nil),
		OpenAIGateway: handler.NewOpenAIGatewayHandler(gatewayService, nil, nil, nil, nil, nil, nil, nil, cfg),
		AsyncImage:    handler.NewAsyncImageHandler(nil, nil),
	}
	group := &service.Group{ID: 1, Platform: service.PlatformOpenAI,
		CodexModelsManifestConfig: service.GroupCodexModelsManifestConfig{Enabled: true, AccountIDs: []int64{7}}}
	router := gin.New()
	RegisterGatewayRoutes(router, handlers, servermiddleware.APIKeyAuthMiddleware(func(ctx *gin.Context) {
		if ctx.GetHeader("Authorization") != "Bearer client-key" {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		ctx.Set(string(servermiddleware.ContextKeyAPIKey), &service.APIKey{GroupID: &group.ID, Group: group})
		ctx.Next()
	}), nil, nil, nil, nil, nil, cfg)
	request := func(path, key, etag string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		requestBody := httptest.NewRequest(http.MethodGet, path, nil)
		requestBody.Header.Set("Authorization", key)
		requestBody.Header.Set("If-None-Match", etag)
		router.ServeHTTP(recorder, requestBody)
		return recorder
	}
	list := request("/v1/models", "Bearer client-key", "")
	require.Equal(test, http.StatusOK, list.Code)
	var catalog struct {
		Data []json.RawMessage `json:"data"`
	}
	require.NoError(test, json.Unmarshal(list.Body.Bytes(), &catalog))
	require.Len(test, catalog.Data, 1)
	for _, base := range []string{"/v1/models/", "/models/"} {
		path := base + "ordinary-upstream-model?client_version=ignored"
		require.Equal(test, http.StatusUnauthorized, request(path, "", "").Code)
		got := request(path, "Bearer client-key", list.Header().Get("ETag"))
		require.Equal(test, http.StatusOK, got.Code, got.Body.String())
		require.JSONEq(test, string(catalog.Data[0]), got.Body.String())
		require.Empty(test, got.Header().Get("ETag"), "a collection validator cannot validate one model")
		missing := request(base+"unknown-model", "Bearer client-key", "")
		require.Equal(test, http.StatusNotFound, missing.Code)
		require.Contains(test, missing.Body.String(), `"error"`)
	}
	require.Zero(test, upstream.codexCalls.Load(), "retrieve never dispatches a Codex manifest")
}
