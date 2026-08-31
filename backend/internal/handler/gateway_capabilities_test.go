package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type gatewayCapabilitiesHandlerRepoStub struct {
	service.AccountRepository
	accounts []service.Account
}

func (r *gatewayCapabilitiesHandlerRepoStub) ListSchedulableByGroupID(context.Context, int64) ([]service.Account, error) {
	return r.accounts, nil
}

func (r *gatewayCapabilitiesHandlerRepoStub) ListModelAvailabilityCandidates(context.Context, *int64, []string, bool) ([]service.Account, error) {
	return r.accounts, nil
}

func TestGatewayCapabilitiesContractShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groupID := int64(42)
	account := service.Account{
		ID:       1,
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeOAuth,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"contract-test-model": "contract-test-model"},
		},
		Extra:       map[string]any{},
		Status:      service.StatusActive,
		Schedulable: true,
	}
	handler := newGatewayModelsHandlerForTest(&gatewayCapabilitiesHandlerRepoStub{accounts: []service.Account{account}})
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/gateway/capabilities", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{Group: &service.Group{
		ID:       groupID,
		Platform: service.PlatformOpenAI,
		ModelsListConfig: service.GroupModelsListConfig{
			Enabled: true,
			Models:  []string{"contract-test-model"},
		},
	}})

	handler.Capabilities(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	var response map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	generatedAt, ok := response["generated_at"].(string)
	require.True(t, ok)
	_, err := time.Parse(time.RFC3339Nano, generatedAt)
	require.NoError(t, err)
	response["generated_at"] = "<generated_at>"
	gateway, ok := response["gateway"].(map[string]any)
	require.True(t, ok)
	require.NotEmpty(t, gateway["version"])
	gateway["version"] = "<gateway_version>"
	normalized, err := json.Marshal(response)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"schema_version": 1,
		"generated_at": "<generated_at>",
		"gateway": {
			"version": "<gateway_version>",
			"upstream_version": "v0.1.183"
		},
		"transport": {
			"http": true,
			"responses": true,
			"chat_completions": true,
			"sse": true,
			"websocket": false
		},
		"models": [{
			"id": "contract-test-model",
			"display_name": "contract-test-model",
			"availability": "unknown",
			"routing": {
				"type": "direct"
			},
			"capacity": {"status": "unknown"}
		}]
	}`, string(normalized))
}

func TestGatewayCapabilitiesRequiresAPIKeyContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/gateway/capabilities", nil)

	(&GatewayHandler{}).Capabilities(c)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Invalid API key")
}

func TestGatewayCapabilityMetadataIsSparseAndEvidenceBased(t *testing.T) {
	nonReasoning := gatewayCapabilityMetadata("grok-4.20-0309-non-reasoning")
	require.NotNil(t, nonReasoning)
	require.NotNil(t, nonReasoning.Reasoning)
	require.False(t, *nonReasoning.Reasoning)
	require.Nil(t, nonReasoning.ImageOutput)

	image := gatewayCapabilityMetadata("gpt-image-2")
	require.NotNil(t, image)
	require.NotNil(t, image.ImageOutput)
	require.True(t, *image.ImageOutput)

	require.Nil(t, gatewayCapabilityMetadata("synthetic-unknown-model"))
}

func TestGatewayCapabilityFallbacksOmitCNProviders(t *testing.T) {
	fallbacks := gatewayCapabilityFallbacks()
	for _, platform := range []string{service.PlatformKimi, service.PlatformZhipu, service.PlatformDeepseek} {
		require.Empty(t, fallbacks[platform], platform)
	}
}
