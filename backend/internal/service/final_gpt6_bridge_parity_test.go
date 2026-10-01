package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGPT6RawChatRejectsReasoningToolCalls(t *testing.T) {
	for _, model := range []string{"gpt-6-sol", "gpt-6-luna"} {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{"public": model}}}
		svc := &OpenAIGatewayService{}
		_, err := svc.forwardAsRawChatCompletions(context.Background(), c, account, []byte(`{"model":"public","messages":[{"role":"user","content":"hello"}],"tools":[{"type":"function","function":{"name":"lookup"}}]}`), "")
		require.ErrorContains(t, err, "requires Responses")
		require.Equal(t, 400, rec.Code)
	}
}

func TestGPT6SolLunaCompatCacheIdentity(t *testing.T) {
	for _, model := range []string{"gpt-6-sol", "openai/gpt-6-luna", "gpt-6-sol-max"} {
		require.True(t, shouldAutoInjectPromptCacheKeyForCompat(model), model)
	}
	for _, model := range []string{"gpt-6-sol-preview", "gpt-6-solitude", "gpt-6-luna-preview"} {
		require.False(t, shouldAutoInjectPromptCacheKeyForCompat(model), model)
	}
}

func TestGPT6RawChatNoneToolsAreForwarded(t *testing.T) {
	for _, model := range []string{"gpt-6-sol", "gpt-6-luna"} {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		body := []byte(`{"model":"` + model + `","reasoning_effort":"none","messages":[{"role":"user","content":"hello"}],"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}}]}`)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"id":"chat_1","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1000,"completion_tokens":10,"prompt_tokens_details":{"cached_tokens":200,"cache_write_tokens":300}}}`))}}
		svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
		account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{"api_key": "fixture-key", "base_url": "https://api.openai.com"}}
		result, err := svc.forwardAsRawChatCompletions(context.Background(), c, account, body, "")
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, "none", gjson.GetBytes(upstream.lastBody, "reasoning_effort").String())
		require.Len(t, gjson.GetBytes(upstream.lastBody, "tools").Array(), 1)
		require.Equal(t, 1000, result.Usage.InputTokens)
		require.Equal(t, 300, result.Usage.CacheCreationInputTokens)
	}
}

func TestGPT6MappedCompatibilityBridgesKeepReasoningAndTools(t *testing.T) {
	for _, model := range []string{"gpt-6-sol", "gpt-6-luna"} {
		for _, messages := range []bool{false, true} {
			body := []byte(`{"model":"public","reasoning_effort":"max","temperature":0.7,"top_p":0.9,"prompt_cache_options":{"ttl":"30m"},"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}}],"messages":[{"role":"user","content":"hello"}]}`)
			if messages {
				body = []byte(`{"model":"public","max_tokens":1000,"output_config":{"effort":"max"},"temperature":0.7,"top_p":0.9,"tools":[{"name":"lookup","input_schema":{"type":"object"}}],"messages":[{"role":"user","content":"hello"}]}`)
			}
			response := `data: {"type":"response.completed","response":{"id":"resp_1","model":"` + model + `","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":1000,"output_tokens":10,"input_tokens_details":{"cached_tokens":200,"cache_write_tokens":300}}}}` + "\n\n"
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(response))}}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
			account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{"api_key": "fixture-key", "base_url": "https://api.openai.com", "model_mapping": map[string]any{"public": model}}}
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
			var result *OpenAIForwardResult
			var err error
			if messages {
				result, err = svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "")
			} else {
				result, err = svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
			}
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, upstream.lastReq)
			require.Equal(t, "/v1/responses", upstream.lastReq.URL.Path)
			require.Equal(t, model, gjson.GetBytes(upstream.lastBody, "model").String())
			require.Equal(t, "max", gjson.GetBytes(upstream.lastBody, "reasoning.effort").String())
			require.Len(t, gjson.GetBytes(upstream.lastBody, "tools").Array(), 1)
			require.False(t, gjson.GetBytes(upstream.lastBody, "temperature").Exists())
			require.False(t, gjson.GetBytes(upstream.lastBody, "top_p").Exists())
			if !messages {
				require.Equal(t, "30m", gjson.GetBytes(upstream.lastBody, "prompt_cache_options.ttl").String())
			}
			require.Equal(t, 300, result.Usage.CacheCreationInputTokens)
		}
	}
}
