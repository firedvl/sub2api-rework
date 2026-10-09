package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestInlineFilePolicyBlocksBeforeGatewaySideEffects(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, endpoint := range []struct {
		name, field string
		call        func(*GatewayHandler, *OpenAIGatewayHandler, *gin.Context)
	}{
		{"gateway chat", "messages", func(g *GatewayHandler, _ *OpenAIGatewayHandler, c *gin.Context) { g.ChatCompletions(c) }},
		{"gateway responses", "input", func(g *GatewayHandler, _ *OpenAIGatewayHandler, c *gin.Context) { g.Responses(c) }},
		{"openai chat", "messages", func(_ *GatewayHandler, g *OpenAIGatewayHandler, c *gin.Context) { g.ChatCompletions(c) }},
		{"openai responses", "input", func(_ *GatewayHandler, g *OpenAIGatewayHandler, c *gin.Context) { g.Responses(c) }},
	} {
		t.Run(endpoint.name, func(t *testing.T) {
			cfg := service.ContentModerationConfig{
				Enabled: true, Mode: service.ContentModerationModePreBlock, AllGroups: true,
				KeywordBlockingMode: service.ContentModerationKeywordModeKeywordOnly, BlockedKeywords: []string{"POLICY_BLOCK"},
				BlockStatus: http.StatusForbidden, BlockMessage: "blocked local file policy", SampleRate: 100,
			}
			settings, err := json.Marshal(cfg)
			require.NoError(t, err)
			moderation := service.NewContentModerationService(&contentModerationHandlerSettingRepo{values: map[string]string{
				service.SettingKeyRiskControlEnabled: "true", service.SettingKeyContentModerationConfig: string(settings),
			}}, &contentModerationHandlerTestRepo{}, nil, nil, nil, nil, nil, nil)
			slots := 0
			concurrency := NewConcurrencyHelper(service.NewConcurrencyService(&concurrencyCacheMock{
				acquireUserSlotFn: func(context.Context, int64, int, string) (bool, error) {
					slots++
					return false, errors.New("downstream gate reached")
				},
			}), SSEPingFormatNone, time.Second)
			gateway := &GatewayHandler{gatewayService: &service.GatewayService{}, contentModerationService: moderation, concurrencyHelper: concurrency}
			openAI := &OpenAIGatewayHandler{
				gatewayService: &service.OpenAIGatewayService{}, contentModerationService: moderation, concurrencyHelper: concurrency,
				billingCacheService: &service.BillingCacheService{}, apiKeyService: &service.APIKeyService{},
			}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 9, UserID: 7, User: &service.User{ID: 7}})
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7, Concurrency: 1})
				c.Next()
			})
			router.POST("/policy-test", func(c *gin.Context) { endpoint.call(gateway, openAI, c) })

			for _, contentKind := range []string{"file", "text", "mixed"} {
				part := map[string]any{"type": "input_text", "text": "POLICY_BLOCK ordinary text"}
				if contentKind != "text" {
					fileData := "data:text/plain;base64," + base64.StdEncoding.EncodeToString([]byte("POLICY_BLOCK file text"))
					part = map[string]any{"type": "input_file", "file_data": fileData}
					if endpoint.field == "messages" {
						part = map[string]any{"type": "file", "file": map[string]any{"file_data": fileData}}
					}
				}
				parts := []any{part}
				if contentKind == "mixed" {
					parts = append([]any{map[string]any{"type": "text", "text": "ordinary text"}}, parts...)
				}
				body, err := json.Marshal(map[string]any{"model": "claude-sonnet-4-5", endpoint.field: []any{map[string]any{"role": "user", "content": parts}}})
				require.NoError(t, err)
				recorder := httptest.NewRecorder()
				request := httptest.NewRequest(http.MethodPost, "/policy-test", strings.NewReader(string(body)))
				request.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(recorder, request)
				require.Equal(t, http.StatusForbidden, recorder.Code, "contentKind=%s body=%s", contentKind, recorder.Body.String())
				require.Contains(t, recorder.Body.String(), "content_policy_violation")
				require.Zero(t, slots, "guard must stop requests before concurrency, billing, and provider dispatch")
			}
		})
	}
}
