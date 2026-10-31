//go:build unit

package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type claudeOAuthPoolRepo struct {
	*anthropicAPIHandlerAccountRepo
}

func (r *claudeOAuthPoolRepo) SetRateLimited(_ context.Context, id int64, reset time.Time) error {
	a, err := r.GetByID(context.Background(), id)
	if err != nil {
		return err
	}
	a.RateLimitResetAt = &reset
	return nil
}
func (r *claudeOAuthPoolRepo) UpdateSessionWindow(_ context.Context, id int64, start, end *time.Time, status string) error {
	a, err := r.GetByID(context.Background(), id)
	if err != nil {
		return err
	}
	a.SessionWindowStart, a.SessionWindowEnd, a.SessionWindowStatus = start, end, status
	return nil
}
func (r *claudeOAuthPoolRepo) UpdateExtra(_ context.Context, id int64, updates map[string]any) error {
	a, err := r.GetByID(context.Background(), id)
	if err != nil {
		return err
	}
	if a.Extra == nil {
		a.Extra = map[string]any{}
	}
	for k, v := range updates {
		a.Extra[k] = v
	}
	return nil
}

type claudeOAuthPoolUpstream struct {
	service.HTTPUpstream
	ids    []int64
	tokens []string
	bodies [][]byte
	stream bool
}

func (u *claudeOAuthPoolUpstream) DoWithTLS(req *http.Request, _ string, id int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	u.ids = append(u.ids, id)
	auth := ""
	for name, values := range req.Header {
		if strings.EqualFold(name, "Authorization") && len(values) > 0 {
			auth = values[0]
			break
		}
	}
	u.tokens = append(u.tokens, auth)
	u.bodies = append(u.bodies, body)
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	if id == 71 {
		headers.Set("anthropic-ratelimit-unified-5h-status", "rejected")
		headers.Set("anthropic-ratelimit-unified-5h-reset", strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10))
		headers.Set("anthropic-ratelimit-unified-5h-utilization", "1")
		return &http.Response{StatusCode: 429, Header: headers, Body: io.NopCloser(strings.NewReader(`{"type":"error","error":{"type":"rate_limit_error","message":"five-hour usage exhausted"}}`))}, nil
	}
	response := `{"id":"msg_second","type":"message","role":"assistant","model":"claude-sonnet-4-6","content":[{"type":"text","text":"SECOND_ACCOUNT_OK"}],"stop_reason":"end_turn","usage":{"input_tokens":2,"output_tokens":1}}`
	if u.stream {
		headers.Set("Content-Type", "text/event-stream")
		response = `event: message_start
data: {"type":"message_start","message":{"id":"msg_second","type":"message","role":"assistant","model":"claude-sonnet-4-6","content":[],"usage":{"input_tokens":2,"output_tokens":0}}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"SECOND_ACCOUNT_OK"}}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}

event: message_stop
data: {"type":"message_stop"}

`
	}
	return &http.Response{StatusCode: 200, Header: headers, Body: io.NopCloser(strings.NewReader(response)), Request: req}, nil
}

func TestGatewayMessagesClaudeOAuthQuotaFailover(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		stream, alreadyLimited bool
	}{
		{"replays the request on the other OAuth account", false, false},
		{"streams only the successful account response", true, false},
		{"skips an account whose quota is already exhausted", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			groupID := int64(70)
			group := &service.Group{ID: groupID, Hydrated: true, Platform: service.PlatformAnthropic, Status: service.StatusActive, RateMultiplier: 1}
			accounts := []*service.Account{}
			for i, id := range []int64{71, 72} {
				accounts = append(accounts, &service.Account{ID: id, Name: fmt.Sprintf("oauth-%d", id), Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth, Priority: i, Concurrency: 1, Schedulable: true, Status: service.StatusActive, Credentials: map[string]any{"access_token": fmt.Sprintf("oauth-token-%d", id)}, AccountGroups: []service.AccountGroup{{AccountID: id, GroupID: groupID}}})
			}
			if tc.alreadyLimited {
				until := time.Now().Add(time.Hour)
				accounts[0].RateLimitResetAt = &until
			}
			repo := &claudeOAuthPoolRepo{&anthropicAPIHandlerAccountRepo{accounts: accounts}}
			cfg := &config.Config{RunMode: config.RunModeSimple}
			cfg.Default.RateMultiplier = 1
			upstream := &claudeOAuthPoolUpstream{stream: tc.stream}
			usageRepo := &openAIWSUsageHandlerUsageLogRepoStub{created: make(chan *service.UsageLog, 2)}
			rateLimits := service.NewRateLimitService(repo, usageRepo, cfg, nil, nil)
			gateway := service.NewGatewayService(repo, &fakeGroupRepo{group: group}, usageRepo, nil, nil, nil, nil, nil, cfg,
				service.NewSchedulerSnapshotService(&fakeSchedulerCache{accounts: accounts}, nil, nil, nil, nil), nil, service.NewBillingService(cfg, nil), rateLimits, nil, nil, upstream, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			billingCache := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
			t.Cleanup(billingCache.Stop)
			h := &GatewayHandler{gatewayService: gateway, billingCacheService: billingCache, cfg: cfg, maxAccountSwitches: 1, concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(&fakeConcurrencyCache{}), SSEPingFormatClaude, 0), usageRecordWorkerPool: newUsageRecordTestPool(t)}
			metadataID := service.FormatMetadataUserID(strings.Repeat("a", 64), "", "11111111-1111-4111-8111-111111111111", "2.1.300")
			body := fmt.Sprintf(`{"model":"claude-sonnet-4-6","max_tokens":16,"stream":%t,"messages":[{"role":"user","content":"keep this conversation"}],"metadata":{"user_id":%q}}`, tc.stream, metadataID)
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body)).WithContext(context.WithValue(context.Background(), ctxkey.Group, group))
			ctx.Request.Header.Set("Content-Type", "application/json")
			ctx.Request.Header.Set("User-Agent", "claude-cli/2.1.300")
			key := &service.APIKey{ID: 75, UserID: 76, GroupID: &groupID, Group: group, Status: service.StatusActive, User: &service.User{ID: 76, Concurrency: 10, Balance: 100}}
			ctx.Set(string(middleware.ContextKeyAPIKey), key)
			ctx.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: key.UserID, Concurrency: 10})
			h.Messages(ctx)
			require.Equal(t, 200, recorder.Code, recorder.Body.String())
			require.Contains(t, recorder.Body.String(), "SECOND_ACCOUNT_OK")
			wantIDs := []int64{71, 72}
			wantTokens := []string{"Bearer oauth-token-71", "Bearer oauth-token-72"}
			if tc.alreadyLimited {
				wantIDs = wantIDs[1:]
				wantTokens = wantTokens[1:]
			}
			require.Equal(t, wantIDs, upstream.ids)
			require.Equal(t, wantTokens, upstream.tokens)
			require.NotNil(t, accounts[0].RateLimitResetAt)
			for _, sent := range upstream.bodies {
				require.Equal(t, "keep this conversation", gjson.GetBytes(sent, "messages.0.content").String())
			}
			if tc.stream {
				require.Equal(t, 1, strings.Count(recorder.Body.String(), "event: message_start"))
			}
			select {
			case usage := <-usageRepo.created:
				require.Equal(t, int64(72), usage.AccountID)
			case <-time.After(5 * time.Second):
				t.Fatal("successful account was not attributed")
			}
		})
	}
}
