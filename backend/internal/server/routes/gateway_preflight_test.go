package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type gatewayPreflightRouteAPIKeyRepo struct {
	service.APIKeyRepository
	keys map[string]*service.APIKey
}

type gatewayPreflightRouteAccountRepo struct {
	service.AccountRepository
	byGroup map[int64][]service.Account
}

func (r *gatewayPreflightRouteAccountRepo) ListModelAvailabilityCandidates(_ context.Context, groupID *int64, _ []string, _ bool) ([]service.Account, error) {
	if groupID == nil {
		return nil, nil
	}
	return append([]service.Account(nil), r.byGroup[*groupID]...), nil
}

func gatewayPreflightRouteAccount(id int64, publicModel, upstreamModel string, modalities []string) service.Account {
	account := service.Account{
		ID: id, Name: "private-route-account", Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth,
		Status: service.StatusActive, Schedulable: true,
		Credentials: map[string]any{
			"access_token":  "route-test-secret",
			"model_mapping": map[string]any{publicModel: upstreamModel},
		},
		Extra: map[string]any{},
	}
	account.SetUpstreamModelInventorySnapshot(service.UpstreamModelInventorySnapshot{Source: "upstream", Models: []string{upstreamModel}})
	account.SetUpstreamModelMetadataSnapshot(service.UpstreamModelMetadataSnapshot{Source: "upstream", Models: map[string]service.UpstreamModelMetadata{
		upstreamModel: {ID: upstreamModel, InputModalities: modalities},
	}})
	return account
}

func (r *gatewayPreflightRouteAPIKeyRepo) GetByKeyForAuth(_ context.Context, key string) (*service.APIKey, error) {
	apiKey := r.keys[key]
	if apiKey == nil {
		return nil, service.ErrAPIKeyNotFound
	}
	clone := *apiKey
	return &clone, nil
}

func (r *gatewayPreflightRouteAPIKeyRepo) UpdateLastUsed(context.Context, int64, time.Time) error {
	return nil
}

func newGatewayPreflightRouteTestRouter(t *testing.T) (*gin.Engine, string, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	blockedGroup := &service.Group{ID: 42, Status: service.StatusActive, Hydrated: true, Platform: service.PlatformAnthropic,
		ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"public-model"}}}
	allowedGroup := &service.Group{ID: 43, Status: service.StatusActive, Hydrated: true, Platform: service.PlatformAnthropic,
		ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"public-model"}}}
	user := &service.User{ID: 7, Role: service.RoleUser, Status: service.StatusActive, Balance: 10}
	blockedKey := &service.APIKey{ID: 100, UserID: user.ID, Key: "preflight-blocked-key", Status: service.StatusActive, User: user, GroupID: &blockedGroup.ID, Group: blockedGroup}
	allowedKey := &service.APIKey{ID: 101, UserID: user.ID, Key: "preflight-allowed-key", Status: service.StatusActive, User: user, GroupID: &allowedGroup.ID, Group: allowedGroup}
	cfg := &config.Config{RunMode: config.RunModeStandard, Gateway: config.GatewayConfig{MaxBodySize: 1 << 20, TextMaxBodySize: 1 << 20}}
	apiKeyService := service.NewAPIKeyService(&gatewayPreflightRouteAPIKeyRepo{keys: map[string]*service.APIKey{
		blockedKey.Key: blockedKey, allowedKey.Key: allowedKey,
	}}, nil, nil, nil, nil, nil, cfg)
	accountRepo := &gatewayPreflightRouteAccountRepo{byGroup: map[int64][]service.Account{
		blockedGroup.ID: {gatewayPreflightRouteAccount(1, "public-model", "upstream-a", []string{"text"})},
		allowedGroup.ID: {gatewayPreflightRouteAccount(2, "public-model", "upstream-b", []string{"text", "image"})},
	}}
	gatewayService := service.NewGatewayService(
		accountRepo, nil, nil, nil, nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	gatewayHandler := handler.NewGatewayHandler(gatewayService, nil, nil, nil, nil, nil, nil, nil, apiKeyService, nil, nil, nil, nil, cfg, nil)
	router := gin.New()
	RegisterGatewayRoutes(router, &handler.Handlers{Gateway: gatewayHandler, OpenAIGateway: &handler.OpenAIGatewayHandler{}},
		servermiddleware.NewAPIKeyAuthMiddleware(apiKeyService, nil, cfg), apiKeyService, nil, nil, nil, nil, cfg)
	return router, blockedKey.Key, allowedKey.Key
}

func TestGatewayPreflightRouteRequiresAPIKey(t *testing.T) {
	router, _, allowedKey := newGatewayPreflightRouteTestRouter(t)
	body := `{"schema_version":2,"model":"public-model","protocol":"responses","input_modalities":["image"]}`

	for _, tc := range []struct {
		name string
		key  string
	}{
		{name: "no key despite admin session"},
		{name: "invalid key", key: "not-a-key"},
		{name: "valid key", key: allowedKey},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/gateway/preflight", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(&http.Cookie{Name: "admin_session", Value: "synthetic-admin-session"})
			if tc.key != "" {
				req.Header.Set("Authorization", "Bearer "+tc.key)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if tc.key == allowedKey {
				require.Equal(t, http.StatusOK, response.Code)
			} else {
				require.Equal(t, http.StatusUnauthorized, response.Code)
			}
		})
	}
}

func TestGatewayPreflightRoutePreservesPublicModelAndReturnsPolicyAsData(t *testing.T) {
	router, blockedKey, allowedKey := newGatewayPreflightRouteTestRouter(t)
	body := `{"schema_version":2,"model":"public-model","protocol":"responses","input_modalities":["image"]}`

	request := func(key string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/gateway/preflight", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+key)
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}

	blocked := request(blockedKey)
	require.Equal(t, http.StatusOK, blocked.Code, blocked.Body.String())
	require.Equal(t, "no-store", blocked.Header().Get("Cache-Control"))
	require.Empty(t, blocked.Header().Get("ETag"))
	for _, private := range []string{"route-test-secret", "private-route-account", "upstream-", "credentials", "model_mapping"} {
		require.NotContains(t, blocked.Body.String(), private)
	}
	var denied struct {
		Model   string `json:"model"`
		Routing struct {
			State string `json:"state"`
		} `json:"routing"`
		Support struct {
			State string `json:"state"`
		} `json:"support"`
	}
	require.NoError(t, json.Unmarshal(blocked.Body.Bytes(), &denied))
	require.Equal(t, "public-model", denied.Model)
	require.Equal(t, "configured", denied.Routing.State)
	require.Equal(t, "unknown", denied.Support.State)

	allowed := request(allowedKey)
	require.Equal(t, http.StatusOK, allowed.Code, allowed.Body.String())
	require.Equal(t, "no-store", allowed.Header().Get("Cache-Control"))
	require.Empty(t, allowed.Header().Get("ETag"))
	for _, private := range []string{"route-test-secret", "private-route-account", "upstream-", "credentials", "model_mapping"} {
		require.NotContains(t, allowed.Body.String(), private)
	}
	var permitted struct {
		Routing struct {
			State string `json:"state"`
		} `json:"routing"`
		Support struct {
			State string `json:"state"`
		} `json:"support"`
	}
	require.NoError(t, json.Unmarshal(allowed.Body.Bytes(), &permitted))
	require.Equal(t, "configured", permitted.Routing.State)
	require.Equal(t, "supported", permitted.Support.State, "same public ID must use this group's upstream metadata")
}
