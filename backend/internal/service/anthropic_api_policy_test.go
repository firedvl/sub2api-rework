package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAnthropicAPIOnlyRejectsOtherAuthenticationBeforeForwarding(t *testing.T) {
	ctx := WithAnthropicAPIOnly(context.Background())
	for _, account := range []*Account{
		nil,
		{Platform: PlatformAnthropic, Type: AccountTypeOAuth},
		{Platform: PlatformAnthropic, Type: AccountTypeSetupToken},
		{Platform: PlatformAnthropic, Type: AccountTypeBedrock},
		{Platform: PlatformAnthropic, Type: AccountTypeServiceAccount},
		{Platform: PlatformAntigravity, Type: AccountTypeAPIKey},
		{Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
	} {
		for _, countTokens := range []bool{false, true} {
			upstream := &anthropicHTTPUpstreamRecorder{}
			svc := &GatewayService{httpUpstream: upstream}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			if countTokens {
				require.Error(t, svc.ForwardCountTokens(ctx, c, account, nil))
			} else {
				_, err := svc.Forward(ctx, c, account, nil)
				require.Error(t, err)
			}
			require.Equal(t, http.StatusForbidden, w.Code)
			require.Nil(t, upstream.lastReq)
		}
	}
}

func TestAnthropicAPIOnlyForcesProtocolPassthrough(t *testing.T) {
	for _, countTokens := range []bool{false, true} {
		ctx := WithAnthropicAPIOnly(context.Background())
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/anthropic-api/v1/messages", nil).WithContext(ctx)
		c.Request.Header.Set("Authorization", "Bearer downstream-secret")
		c.Request.Header.Set("X-Api-Key", "downstream-secret")
		c.Request.Header.Set("Cookie", "private=downstream")
		c.Request.Header.Set("Anthropic-Beta", "context-management-2025-06-27")
		body := []byte(`{"model":"claude-sonnet-4-6","max_tokens":32,"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"done"}]}],"tools":[{"name":"read","input_schema":{"type":"object"}}],"thinking":{"type":"enabled","budget_tokens":16},"system":[{"type":"text","text":"keep","cache_control":{"type":"ephemeral"}}],"context_management":{"edits":[]},"metadata":{"user_id":"native-session"}}`)
		response := `{"id":"test","type":"message","model":"claude-sonnet-4-6","usage":{"input_tokens":12,"output_tokens":2}}`
		if countTokens {
			response = `{"input_tokens":12}`
		}
		upstream := &anthropicHTTPUpstreamRecorder{resp: &http.Response{
			StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(response)),
		}}
		svc := &GatewayService{cfg: &config.Config{}, httpUpstream: upstream}
		account := newAnthropicAPIKeyAccountForTest()
		account.Extra = nil // The dedicated endpoint must not depend on a saved passthrough toggle.
		parsed := &ParsedRequest{Body: NewRequestBodyRef(body), Model: "claude-sonnet-4-6"}
		if countTokens {
			require.NoError(t, svc.ForwardCountTokens(ctx, c, account, parsed))
			require.Contains(t, upstream.lastReq.URL.Path, "count_tokens")
		} else {
			result, err := svc.Forward(ctx, c, account, parsed)
			require.NoError(t, err)
			require.Equal(t, 12, result.Usage.InputTokens)
			require.JSONEq(t, string(body), string(upstream.lastBody))
		}
		require.Equal(t, "upstream-anthropic-key", getHeaderRaw(upstream.lastReq.Header, "x-api-key"))
		require.Empty(t, getHeaderRaw(upstream.lastReq.Header, "authorization"))
		require.Empty(t, getHeaderRaw(upstream.lastReq.Header, "cookie"))
		require.Empty(t, getHeaderRaw(upstream.lastReq.Header, "x-stainless-lang"))
		require.Nil(t, account.Extra)
	}
}

func TestAnthropicAPIOnlySessionIsolationAndSlotRelease(t *testing.T) {
	plain := context.Background()
	ctx := WithAnthropicAPIOnly(plain)
	require.Equal(t, "", ScopeAnthropicAPISession(ctx, ""))
	require.Equal(t, "session", ScopeAnthropicAPISession(plain, "session"))
	require.Equal(t, "anthropic-api:session", ScopeAnthropicAPISession(ctx, "session"))
	require.Equal(t, "legacy:anthropic-api:session", ScopeAnthropicAPISession(plain, "anthropic-api:session"))
	require.True(t, IsAnthropicAPIOnly(context.WithoutCancel(ctx)))
	account := &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth}
	released := 0
	svc := &GatewayService{}
	result, err := svc.newSelectionResult(ctx, account, true, func() { released++ }, nil)
	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, 1, released)
	_, err = svc.hydrateSelectedAccount(plain, account)
	require.NoError(t, err, "legacy authentication policy is unchanged")
}
