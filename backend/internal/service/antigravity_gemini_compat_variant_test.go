//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAntigravityCompatLowEffortSelectsDiscoveredLowVariant(t *testing.T) {
	for _, tc := range []struct {
		name, path, body string
		forward          func(*AntigravityGatewayService, context.Context, *gin.Context, *Account, []byte, *ParsedRequest) (*ForwardResult, error)
	}{
		{"chat", "/v1/chat/completions", `{"model":"gemini-3.6-flash","messages":[{"role":"user","content":"ok"}],"reasoning_effort":"low"}`, (*AntigravityGatewayService).ForwardAsChatCompletions},
		{"responses", "/v1/responses", `{"model":"gemini-3.6-flash","input":"ok","reasoning":{"effort":"low"}}`, (*AntigravityGatewayService).ForwardAsResponses},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := &queuedHTTPUpstreamStub{responses: []*http.Response{antigravityCompatSuccessResponse()}}
			svc := newAntigravityCompatService(config.GatewayConfig{MaxLineSize: defaultMaxLineSize}, upstream)
			account := newAntigravityCompatAccount(AccountTypeOAuth)
			account.SetUpstreamModelInventorySnapshot(UpstreamModelInventorySnapshot{Source: "account", Models: []string{
				"gemini-3.6-flash-low", "gemini-3.6-flash-high",
			}})
			c, _ := newAntigravityCompatContext(http.MethodPost, tc.path, []byte(tc.body))
			_, err := tc.forward(svc, context.Background(), c, account, []byte(tc.body), nil)
			require.NoError(t, err)
			require.Len(t, upstream.requestBodies, 1)
			require.Equal(t, "gemini-3.6-flash-low", gjson.GetBytes(upstream.requestBodies[0], "model").String())
		})
	}
}
