package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type gatewayCapabilityRouteAccountRepo struct {
	service.AccountRepository
	accounts []service.Account
}

type gatewayCapabilityRouteAPIKeyRepo struct {
	service.APIKeyRepository
	apiKey *service.APIKey
}

func (r *gatewayCapabilityRouteAPIKeyRepo) GetByKeyForAuth(_ context.Context, key string) (*service.APIKey, error) {
	if r.apiKey == nil || key != r.apiKey.Key {
		return nil, service.ErrAPIKeyNotFound
	}
	clone := *r.apiKey
	return &clone, nil
}

func (r *gatewayCapabilityRouteAPIKeyRepo) UpdateLastUsed(context.Context, int64, time.Time) error {
	return nil
}

func (r *gatewayCapabilityRouteAccountRepo) ListSchedulableByGroupID(context.Context, int64) ([]service.Account, error) {
	return r.accounts, nil
}

func (r *gatewayCapabilityRouteAccountRepo) ListModelAvailabilityCandidates(context.Context, *int64, []string, bool) ([]service.Account, error) {
	return r.accounts, nil
}

func newGatewayCapabilityRouteTestRouter(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	group := &service.Group{
		ID:       42,
		Status:   service.StatusActive,
		Hydrated: true,
		Platform: service.PlatformOpenAI,
		ModelsListConfig: service.GroupModelsListConfig{
			Enabled: true,
			Models:  []string{"visible-model"},
		},
	}
	user := &service.User{ID: 7, Role: service.RoleUser, Status: service.StatusActive, Balance: 10}
	apiKey := &service.APIKey{
		ID:      100,
		UserID:  user.ID,
		Key:     "gateway-capability-route-test-key",
		Status:  service.StatusActive,
		User:    user,
		GroupID: &group.ID,
		Group:   group,
	}
	cfg := &config.Config{RunMode: config.RunModeStandard}
	apiKeyService := service.NewAPIKeyService(
		&gatewayCapabilityRouteAPIKeyRepo{apiKey: apiKey}, nil, nil, nil, nil, nil, cfg,
	)
	accountRepo := &gatewayCapabilityRouteAccountRepo{accounts: []service.Account{{
		ID:       1,
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeOAuth,
		Credentials: map[string]any{"model_mapping": map[string]any{
			"visible-model": "visible-model",
			"hidden-model":  "hidden-model",
		}},
		Extra:       map[string]any{},
		Status:      service.StatusActive,
		Schedulable: true,
	}}}
	gatewayService := service.NewGatewayService(
		accountRepo,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	gatewayHandler := handler.NewGatewayHandler(
		gatewayService, nil, nil, nil, nil, nil, nil, nil,
		apiKeyService, nil, nil, nil, nil, cfg, nil,
	)
	router := gin.New()
	RegisterGatewayRoutes(
		router,
		&handler.Handlers{Gateway: gatewayHandler, OpenAIGateway: &handler.OpenAIGatewayHandler{}},
		servermiddleware.NewAPIKeyAuthMiddleware(apiKeyService, nil, cfg),
		apiKeyService,
		nil, nil, nil, nil,
		cfg,
	)
	return router, apiKey.Key
}

func TestGatewayCapabilitiesRouteAuthScopeAndModelsCompatibility(t *testing.T) {
	router, key := newGatewayCapabilityRouteTestRouter(t)

	t.Run("missing key and admin cookie are rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/gateway/capabilities", nil)
		req.AddCookie(&http.Cookie{Name: "admin_session", Value: "synthetic-admin-session"})
		response := httptest.NewRecorder()

		router.ServeHTTP(response, req)

		require.Equal(t, http.StatusUnauthorized, response.Code)
	})

	t.Run("invalid key is rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/gateway/capabilities", nil)
		req.Header.Set("Authorization", "Bearer invalid-gateway-key")
		response := httptest.NewRecorder()

		router.ServeHTTP(response, req)

		require.Equal(t, http.StatusUnauthorized, response.Code)
	})

	t.Run("valid normal key sees only its model scope", func(t *testing.T) {
		capabilityRequest := httptest.NewRequest(http.MethodGet, "/v1/gateway/capabilities", nil)
		capabilityRequest.Header.Set("Authorization", "Bearer "+key)
		capabilityResponse := httptest.NewRecorder()
		router.ServeHTTP(capabilityResponse, capabilityRequest)
		require.Equal(t, http.StatusOK, capabilityResponse.Code)
		require.Equal(t, "no-store", capabilityResponse.Header().Get("Cache-Control"))

		var capabilities struct {
			SchemaVersion int `json:"schema_version"`
			Models        []struct {
				ID string `json:"id"`
			} `json:"models"`
		}
		require.NoError(t, json.Unmarshal(capabilityResponse.Body.Bytes(), &capabilities))
		require.Equal(t, 1, capabilities.SchemaVersion)
		require.Equal(t, "visible-model", capabilities.Models[0].ID)
		require.Len(t, capabilities.Models, 1)
		require.NotContains(t, capabilityResponse.Body.String(), "hidden-model")

		modelsRequest := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		modelsRequest.Header.Set("x-api-key", key)
		modelsResponse := httptest.NewRecorder()
		router.ServeHTTP(modelsResponse, modelsRequest)
		require.Equal(t, http.StatusOK, modelsResponse.Code)
		var models struct {
			Object string `json:"object"`
			Data   []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(modelsResponse.Body.Bytes(), &models))
		require.Equal(t, "list", models.Object)
		require.Equal(t, capabilities.Models[0].ID, models.Data[0].ID)
		require.Len(t, models.Data, 1)
	})
}
