package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type anthropicAPIAdmissionRepo struct {
	AccountRepository
	account *Account
	err     error
	reads   int
}

func (r *anthropicAPIAdmissionRepo) GetByID(context.Context, int64) (*Account, error) {
	r.reads++
	return r.account, r.err
}

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
		svc.accountRepo = &anthropicAPIAdmissionRepo{account: account}
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

func TestAnthropicAPIOnlyAuthoritativeAdmissionAndForward(t *testing.T) {
	for _, mutation := range []string{"oauth", "platform", "key", "concurrency", "disabled", "missing", "read_error"} {
		for _, boundary := range []string{"admission", "messages", "count_tokens", "token"} {
			t.Run(mutation+"/"+boundary, func(t *testing.T) {
				selected := newAnthropicAPIKeyAccountForTest()
				current := *selected
				repo := &anthropicAPIAdmissionRepo{account: &current}
				switch mutation {
				case "oauth":
					current.Type = AccountTypeOAuth
				case "platform":
					current.Platform = PlatformAntigravity
				case "key":
					current.Credentials = map[string]any{"api_key": "replaced"}
				case "concurrency":
					current.Concurrency++
				case "disabled":
					current.Schedulable = false
				case "missing":
					repo.account = nil
				case "read_error":
					repo.err = errors.New("synthetic read failure")
				}
				upstream := &anthropicHTTPUpstreamRecorder{}
				svc := &GatewayService{accountRepo: repo, httpUpstream: upstream}
				ctx := WithAnthropicAPIOnly(t.Context())
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				switch boundary {
				case "admission":
					_, vetoed, reason := svc.GatewayProfitControlVetoLatest(ctx, selected)
					require.True(t, vetoed, "default group without profit control must refresh canonical state")
					require.Equal(t, "anthropic_api_auth_policy", reason)
				case "messages":
					_, err := svc.Forward(ctx, c, selected, nil)
					require.Error(t, err)
				case "count_tokens":
					require.Error(t, svc.ForwardCountTokens(ctx, c, selected, nil))
				case "token":
					_, _, err := svc.GetAccessToken(ctx, selected)
					require.Error(t, err)
				}
				require.Equal(t, 1, repo.reads)
				require.Nil(t, upstream.lastReq)
			})
		}
	}
}

func TestAnthropicAPIOnlyAdmitsUsageUpdates(t *testing.T) {
	selected := newAnthropicAPIKeyAccountForTest()
	current := *selected
	current.UpdatedAt = time.Now()
	current.LastUsedAt = &current.UpdatedAt
	current.Extra = map[string]any{"quota_used": 10, "session_window_utilization": .2}
	svc := &GatewayService{accountRepo: &anthropicAPIAdmissionRepo{account: &current}}
	latest, vetoed, _ := svc.GatewayProfitControlVetoLatest(WithAnthropicAPIOnly(t.Context()), selected)
	require.False(t, vetoed, "ordinary usage and LRU writes are not configuration changes")
	require.Same(t, &current, latest)
}

func TestAnthropicAPIOnlyRechecksGroupMembership(t *testing.T) {
	selected := newAnthropicAPIKeyAccountForTest()
	selected.AccountGroups = []AccountGroup{{AccountID: selected.ID, GroupID: 42}}
	current := *selected
	current.AccountGroups = nil
	svc := &GatewayService{accountRepo: &anthropicAPIAdmissionRepo{account: &current}}
	ctx := WithAnthropicAPIOnly(context.WithValue(t.Context(), ctxkey.Group, &Group{ID: 42, Platform: PlatformAnthropic}))
	_, vetoed, _ := svc.GatewayProfitControlVetoLatest(ctx, selected)
	require.True(t, vetoed)
	_, _, err := svc.GetAccessToken(ctx, selected)
	require.Error(t, err)
	require.False(t, svc.isAccountSchedulableForModelSelection(ctx, selected, "claude-sonnet-4-6"))
	current.AccountGroups = selected.AccountGroups
	_, vetoed, _ = svc.GatewayProfitControlVetoLatest(ctx, selected)
	require.False(t, vetoed)
}

func TestAnthropicAPIOnlyRetryClassification(t *testing.T) {
	for _, code := range []int{400, 404, 413, 422, 503} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			account := newAnthropicAPIKeyAccountForTest()
			account.Credentials["custom_error_codes_enabled"] = true
			account.Credentials["custom_error_codes"] = []any{float64(401), float64(403), float64(429), float64(500)}
			upstream := &anthropicCancellationUpstream{do: func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: code, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"type":"error","error":{"message":"synthetic"}}`))}, nil
			}}
			svc := &GatewayService{cfg: &config.Config{}, httpUpstream: upstream, accountRepo: &anthropicAPIAdmissionRepo{account: account}}
			ctx := WithAnthropicAPIOnly(t.Context())
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/anthropic-api/v1/messages", nil).WithContext(ctx)
			_, err := svc.forwardAnthropicAPIKeyPassthrough(ctx, c, account, []byte(`{"model":"x"}`), "x", "x", false, time.Now())
			require.Error(t, err)
			if code == 503 {
				require.Equal(t, maxRetryAttempts, upstream.attempts)
			} else {
				require.Equal(t, 1, upstream.attempts)
			}
		})
	}
}

func TestAnthropicAPIOnlySessionIsolationAndSlotRelease(t *testing.T) {
	plain := context.Background()
	ctx := WithAnthropicAPIOnly(plain)
	require.Equal(t, "", ScopeAnthropicAPISession(ctx, "", 1))
	require.Equal(t, "session", ScopeAnthropicAPISession(plain, "session", 1))
	scoped := ScopeAnthropicAPISession(ctx, "session", 1)
	require.True(t, strings.HasPrefix(scoped, "anthropic-api:1:"))
	require.NotContains(t, scoped, "session")
	require.Equal(t, scoped, ScopeAnthropicAPISession(ctx, "session", 1))
	require.NotEqual(t, scoped, ScopeAnthropicAPISession(ctx, "session", 2))
	require.Equal(t, "", ScopeAnthropicAPISession(ctx, "session", 0))
	require.Equal(t, "", ScopeAnthropicAPISession(ctx, "session", -1))
	require.Equal(t, "legacy:"+scoped, ScopeAnthropicAPISession(plain, scoped, 1))
	require.NotEqual(t, scoped, ScopeAnthropicAPISession(ctx, scoped, 2), "client input cannot impersonate a scoped key")
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
