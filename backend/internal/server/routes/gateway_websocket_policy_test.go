package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGatewayWebSocketPolicyRegisteredRouteAuthenticationAndFirstFrameValidation(t *testing.T) {
	router, apiKey := newGatewayWebSocketPolicyRouter(t)
	server := httptest.NewServer(router)
	defer server.Close()

	t.Run("unauthenticated handshake is rejected", func(t *testing.T) {
		conn, response, err := gatewayWebSocketPolicyDial(server.URL, "")
		require.Error(t, err)
		require.Nil(t, conn)
		require.NotNil(t, response)
		require.Equal(t, http.StatusUnauthorized, response.StatusCode)
		_ = response.Body.Close()
	})

	for _, tc := range []struct {
		name       string
		firstFrame string
		wantReason string
	}{
		{name: "invalid JSON", firstFrame: "not json", wantReason: "invalid JSON payload"},
		{name: "model is not allowed", firstFrame: `{"type":"response.create","model":"blocked-model"}`, wantReason: `Model "blocked-model" is not available for this group`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, response, err := gatewayWebSocketPolicyDial(server.URL, apiKey)
			require.NoError(t, err)
			require.NotNil(t, response)
			require.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)
			defer func() { _ = conn.CloseNow() }()

			writeCtx, cancelWrite := context.WithTimeout(context.Background(), 3*time.Second)
			err = conn.Write(writeCtx, coderws.MessageText, []byte(tc.firstFrame))
			cancelWrite()
			require.NoError(t, err)

			readCtx, cancelRead := context.WithTimeout(context.Background(), 3*time.Second)
			_, _, err = conn.Read(readCtx)
			cancelRead()
			var closeErr coderws.CloseError
			require.ErrorAs(t, err, &closeErr)
			require.Equal(t, coderws.StatusPolicyViolation, closeErr.Code)
			require.Equal(t, tc.wantReason, closeErr.Reason)
		})
	}
}

type gatewayWebSocketPolicyAPIKeyRepo struct {
	service.APIKeyRepository
	apiKey *service.APIKey
}

func (r *gatewayWebSocketPolicyAPIKeyRepo) GetByKeyForAuth(_ context.Context, key string) (*service.APIKey, error) {
	if r.apiKey == nil || key != r.apiKey.Key {
		return nil, service.ErrAPIKeyNotFound
	}
	clone := *r.apiKey
	return &clone, nil
}

func (r *gatewayWebSocketPolicyAPIKeyRepo) UpdateLastUsed(context.Context, int64, time.Time) error {
	return nil
}

func newGatewayWebSocketPolicyRouter(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	groupID := int64(401)
	group := &service.Group{
		ID:       groupID,
		Status:   service.StatusActive,
		Hydrated: true,
		Platform: service.PlatformOpenAI,
		ModelAllowlist: service.GroupModelAllowlist{
			Enabled: true,
			Models:  []string{"allowed-model"},
		},
	}
	user := &service.User{ID: 402, Role: service.RoleUser, Status: service.StatusActive, Concurrency: 1}
	apiKey := &service.APIKey{
		ID:      403,
		UserID:  user.ID,
		Key:     "gateway-websocket-policy-key",
		Status:  service.StatusActive,
		User:    user,
		GroupID: &groupID,
		Group:   group,
	}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	apiKeyService := service.NewAPIKeyService(&gatewayWebSocketPolicyAPIKeyRepo{apiKey: apiKey}, nil, nil, nil, nil, nil, cfg)
	openAIHandler := handler.NewOpenAIGatewayHandler(
		&service.OpenAIGatewayService{},
		service.NewConcurrencyService(nil),
		&service.BillingCacheService{},
		apiKeyService,
		nil, nil, nil, nil, cfg,
	)
	router := gin.New()
	RegisterGatewayRoutes(
		router,
		&handler.Handlers{Gateway: &handler.GatewayHandler{}, OpenAIGateway: openAIHandler},
		servermiddleware.NewAPIKeyAuthMiddleware(apiKeyService, nil, cfg),
		apiKeyService,
		nil, nil, nil, nil,
		cfg,
	)
	return router, apiKey.Key
}

func gatewayWebSocketPolicyDial(serverURL, apiKey string) (*coderws.Conn, *http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	headers := http.Header{}
	if apiKey != "" {
		headers.Set("Authorization", "Bearer "+apiKey)
	}
	return coderws.Dial(ctx, "ws"+strings.TrimPrefix(serverURL, "http")+"/v1/responses", &coderws.DialOptions{HTTPHeader: headers})
}
