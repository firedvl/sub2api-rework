package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestResponsesWebSocketPolicyIngressIsIndependentOfUpstreamResolver(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tc := range []struct {
		name               string
		accountType        string
		configure          func(*config.Config)
		wantTransport      service.OpenAIUpstreamTransport
		wantResolverReason string
	}{
		{
			name:               "global enabled v2 oauth",
			accountType:        service.AccountTypeOAuth,
			configure:          func(*config.Config) {},
			wantTransport:      service.OpenAIUpstreamTransportResponsesWebsocketV2,
			wantResolverReason: "ws_v2_enabled",
		},
		{
			name:               "global enabled v2 api key",
			accountType:        service.AccountTypeAPIKey,
			configure:          func(*config.Config) {},
			wantTransport:      service.OpenAIUpstreamTransportResponsesWebsocketV2,
			wantResolverReason: "ws_v2_enabled",
		},
		{
			name:        "global disabled oauth",
			accountType: service.AccountTypeOAuth,
			configure: func(cfg *config.Config) {
				cfg.Gateway.OpenAIWS.Enabled = false
			},
			wantTransport:      service.OpenAIUpstreamTransportHTTPSSE,
			wantResolverReason: "global_disabled",
		},
		{
			name:        "global disabled api key",
			accountType: service.AccountTypeAPIKey,
			configure: func(cfg *config.Config) {
				cfg.Gateway.OpenAIWS.Enabled = false
			},
			wantTransport:      service.OpenAIUpstreamTransportHTTPSSE,
			wantResolverReason: "global_disabled",
		},
		{
			name:        "v1 enabled oauth",
			accountType: service.AccountTypeOAuth,
			configure: func(cfg *config.Config) {
				cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = false
				cfg.Gateway.OpenAIWS.ResponsesWebsockets = true
			},
			wantTransport:      service.OpenAIUpstreamTransportResponsesWebsocket,
			wantResolverReason: "ws_v1_enabled",
		},
		{
			name:        "v1 enabled api key",
			accountType: service.AccountTypeAPIKey,
			configure: func(cfg *config.Config) {
				cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = false
				cfg.Gateway.OpenAIWS.ResponsesWebsockets = true
			},
			wantTransport:      service.OpenAIUpstreamTransportResponsesWebsocket,
			wantResolverReason: "ws_v1_enabled",
		},
		{
			name:        "v1 and v2 disabled oauth",
			accountType: service.AccountTypeOAuth,
			configure: func(cfg *config.Config) {
				cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = false
			},
			wantTransport:      service.OpenAIUpstreamTransportHTTPSSE,
			wantResolverReason: "feature_disabled",
		},
		{
			name:        "v1 and v2 disabled api key",
			accountType: service.AccountTypeAPIKey,
			configure: func(cfg *config.Config) {
				cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = false
			},
			wantTransport:      service.OpenAIUpstreamTransportHTTPSSE,
			wantResolverReason: "feature_disabled",
		},
		{
			name:        "oauth disabled",
			accountType: service.AccountTypeOAuth,
			configure: func(cfg *config.Config) {
				cfg.Gateway.OpenAIWS.OAuthEnabled = false
			},
			wantTransport:      service.OpenAIUpstreamTransportHTTPSSE,
			wantResolverReason: "oauth_disabled",
		},
		{
			name:        "api key disabled",
			accountType: service.AccountTypeAPIKey,
			configure: func(cfg *config.Config) {
				cfg.Gateway.OpenAIWS.APIKeyEnabled = false
			},
			wantTransport:      service.OpenAIUpstreamTransportHTTPSSE,
			wantResolverReason: "apikey_disabled",
		},
		{
			name:        "force http api key",
			accountType: service.AccountTypeAPIKey,
			configure: func(cfg *config.Config) {
				cfg.Gateway.OpenAIWS.ForceHTTP = true
			},
			wantTransport:      service.OpenAIUpstreamTransportHTTPSSE,
			wantResolverReason: "global_force_http",
		},
		{
			name:        "force http oauth",
			accountType: service.AccountTypeOAuth,
			configure: func(cfg *config.Config) {
				cfg.Gateway.OpenAIWS.ForceHTTP = true
			},
			wantTransport:      service.OpenAIUpstreamTransportHTTPSSE,
			wantResolverReason: "global_force_http",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := websocketPolicyConfig()
			tc.configure(cfg)
			decision := service.NewOpenAIWSProtocolResolver(cfg).Resolve(websocketPolicyAccount(tc.accountType))
			require.Equal(t, tc.wantTransport, decision.Transport)
			require.Equal(t, tc.wantResolverReason, decision.Reason)

			h := newOpenAIHandlerForPreviousResponseIDValidation(t, nil)
			h.cfg = cfg
			server := newOpenAIWSHandlerTestServer(t, h, middleware.AuthSubject{UserID: 1, Concurrency: 1})
			defer server.Close()

			conn, response, err := websocketPolicyDial(server.URL)
			require.NoError(t, err)
			require.NotNil(t, response)
			require.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)
			defer func() { _ = conn.CloseNow() }()

			writeCtx, cancelWrite := context.WithTimeout(context.Background(), 3*time.Second)
			err = conn.Write(writeCtx, coderws.MessageText, []byte("not json"))
			cancelWrite()
			require.NoError(t, err)

			readCtx, cancelRead := context.WithTimeout(context.Background(), 3*time.Second)
			_, _, err = conn.Read(readCtx)
			cancelRead()
			var closeErr coderws.CloseError
			require.ErrorAs(t, err, &closeErr)
			require.Equal(t, coderws.StatusPolicyViolation, closeErr.Code)
			require.Equal(t, "invalid JSON payload", closeErr.Reason)
		})
	}
}

func TestResponsesWebSocketPolicyUnauthenticatedHandshakeIsRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newOpenAIHandlerForPreviousResponseIDValidation(t, nil)
	h.cfg = websocketPolicyConfig()
	router := gin.New()
	router.GET("/openai/v1/responses", h.ResponsesWebSocket)
	server := httptest.NewServer(router)
	defer server.Close()

	conn, response, err := websocketPolicyDial(server.URL)
	require.Error(t, err)
	require.Nil(t, conn)
	require.NotNil(t, response)
	require.Equal(t, http.StatusUnauthorized, response.StatusCode)
	_ = response.Body.Close()
}

func websocketPolicyConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	return cfg
}

func websocketPolicyAccount(accountType string) *service.Account {
	extra := map[string]any{"openai_oauth_responses_websockets_v2_enabled": true}
	if accountType == service.AccountTypeAPIKey {
		extra = map[string]any{"openai_apikey_responses_websockets_v2_enabled": true}
	}
	return &service.Account{Platform: service.PlatformOpenAI, Type: accountType, Extra: extra}
}

func websocketPolicyDial(serverURL string) (*coderws.Conn, *http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return coderws.Dial(ctx, "ws"+strings.TrimPrefix(serverURL, "http")+"/openai/v1/responses", nil)
}
