package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestMappedGPT55LiteCompatibilityPreservesRequestContent(t *testing.T) {
	for _, tc := range []struct {
		model, kind string
		wantLite    bool
	}{
		{"gpt-5.5", AccountTypeOAuth, false},
		{"gpt-6-astra", AccountTypeOAuth, true},
		{"gpt-5.5", AccountTypeAPIKey, true},
	} {
		a := &Account{Platform: PlatformOpenAI, Type: tc.kind}
		body := []byte(`{"model":"` + tc.model + `","input":[{"type":"additional_tools","tools":[]},{"type":"function_call_output","call_id":"call1","output":"history"}],"reasoning":{"context":"all_turns"},"client_metadata":{"ws_request_header_x_openai_internal_codex_responses_lite":"true","keep":"yes"}}`)
		req, err := http.NewRequest(http.MethodPost, "http://localhost", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set(responsesLiteHeader, "true")
		require.NoError(t, applyMappedGPT55LiteCompatibility(req, a, body))
		got, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		require.Equal(t, tc.wantLite, isOpenAIResponsesLiteHeader(req.Header.Get(responsesLiteHeader)))
		require.Equal(t, tc.wantLite, isOpenAIResponsesLiteWebSocketPayload(got))
		require.Equal(t, gjson.GetBytes(body, "input").Raw, gjson.GetBytes(got, "input").Raw)
		require.Equal(t, "all_turns", gjson.GetBytes(got, "reasoning.context").String())
		require.Equal(t, "yes", gjson.GetBytes(got, "client_metadata.keep").String())
		replay, err := req.GetBody()
		require.NoError(t, err)
		again, err := io.ReadAll(replay)
		require.NoError(t, err)
		require.NoError(t, replay.Close())
		require.Equal(t, got, again)
		require.True(t, isOpenAIResponsesLiteWebSocketPayload(body))
	}
}

func TestMappedGPT55LiteBuildersPreserveIngressForFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, passthrough := range []bool{false, true} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
		c.Request.Header.Set(responsesLiteHeader, "true")
		s := &OpenAIGatewayService{cfg: &config.Config{}}
		a := &Account{ID: 12, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
		build := func(model string) *http.Request {
			body := []byte(`{"model":"` + model + `","stream":true,"input":[]}`)
			var req *http.Request
			var err error
			if passthrough {
				req, err = s.buildUpstreamRequestOpenAIPassthrough(context.Background(), c, a, body, "test-token")
			} else {
				req, err = s.buildUpstreamRequest(context.Background(), c, a, body, "test-token", true, "", true)
			}
			require.NoError(t, err)
			return req
		}
		require.Empty(t, build("gpt-5.5").Header.Get(responsesLiteHeader))
		require.Equal(t, "true", c.GetHeader(responsesLiteHeader))
		require.Equal(t, "true", build("gpt-6-astra").Header.Get(responsesLiteHeader))
	}
}
