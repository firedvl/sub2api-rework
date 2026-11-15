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

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func usageLimitTestContext(path string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, path, nil)
	return c, recorder
}

func usageLimitTestDiagnoser(limit *service.UsageExhaustion) *fakeDiagnoser {
	return &fakeDiagnoser{resp: service.ModelAvailabilityDiagnosis{
		HasAccountsInPool: true,
		HasModelSupport:   true,
		UsageExhaustion:   limit,
	}}
}

func requireRetryAfterNear(t *testing.T, header http.Header, want time.Duration) {
	t.Helper()
	seconds, err := strconv.Atoi(header.Get("Retry-After"))
	require.NoError(t, err, "Retry-After must be integer seconds")
	require.InDelta(t, want.Seconds(), seconds, 5)
}

func TestClassifyNoAccountError_UsageLimitOnMessagesSetsUnifiedHeaders(t *testing.T) {
	for _, tc := range []struct {
		window, claim string
		resetIn       time.Duration
	}{
		{"5h", "five_hour", time.Hour},
		{"7d", "seven_day", 72 * time.Hour},
	} {
		t.Run(tc.window, func(t *testing.T) {
			resetAt := time.Now().Add(tc.resetIn).Truncate(time.Second)
			c, recorder := usageLimitTestContext("/v1/messages")
			apiKey := &service.APIKey{GroupID: ptrInt64(7)}

			cls := classifyNoAccountErrorFromGin(c, usageLimitTestDiagnoser(&service.UsageExhaustion{ResetAt: resetAt, Window: tc.window}), apiKey, "claude-sonnet-4-6", "claude-sonnet-4-6", service.PlatformAnthropic)

			require.Equal(t, http.StatusTooManyRequests, cls.Status)
			require.Equal(t, "rate_limit_error", cls.ErrType)
			require.False(t, cls.ModelNotFound)
			require.NotNil(t, cls.UsageLimit)
			require.Contains(t, cls.Message, resetAt.UTC().Format(time.RFC3339), "the message states the reset time")

			reset := strconv.FormatInt(resetAt.Unix(), 10)
			header := recorder.Header()
			requireRetryAfterNear(t, header, tc.resetIn)
			require.Equal(t, "rejected", header.Get("anthropic-ratelimit-unified-status"))
			require.Equal(t, reset, header.Get("anthropic-ratelimit-unified-reset"))
			require.Equal(t, tc.claim, header.Get("anthropic-ratelimit-unified-representative-claim"))
			require.Equal(t, "rejected", header.Get("anthropic-ratelimit-unified-"+tc.window+"-status"))
			require.Equal(t, reset, header.Get("anthropic-ratelimit-unified-"+tc.window+"-reset"))
		})
	}
}

func TestClassifyNoAccountError_UsageLimitUnknownWindowOmitsClaim(t *testing.T) {
	resetAt := time.Now().Add(10 * time.Minute).Truncate(time.Second)
	c, recorder := usageLimitTestContext("/v1/messages")
	apiKey := &service.APIKey{GroupID: ptrInt64(7)}

	cls := classifyNoAccountErrorFromGin(c, usageLimitTestDiagnoser(&service.UsageExhaustion{ResetAt: resetAt}), apiKey, "claude-sonnet-4-6", "claude-sonnet-4-6", service.PlatformAnthropic)

	require.Equal(t, http.StatusTooManyRequests, cls.Status)
	header := recorder.Header()
	requireRetryAfterNear(t, header, 10*time.Minute)
	require.Equal(t, "rejected", header.Get("anthropic-ratelimit-unified-status"))
	require.Equal(t, strconv.FormatInt(resetAt.Unix(), 10), header.Get("anthropic-ratelimit-unified-reset"))
	for name := range header {
		lower := strings.ToLower(name)
		require.NotContains(t, lower, "representative-claim", "an unknown window must not be labeled")
		require.NotContains(t, lower, "unified-5h-")
		require.NotContains(t, lower, "unified-7d-")
	}
}

func TestClassifyNoAccountError_NoKnownResetKeepsExisting503(t *testing.T) {
	c, recorder := usageLimitTestContext("/v1/messages")
	apiKey := &service.APIKey{GroupID: ptrInt64(7)}

	cls := classifyNoAccountErrorFromGin(c, usageLimitTestDiagnoser(nil), apiKey, "claude-sonnet-4-6", "claude-sonnet-4-6", service.PlatformAnthropic)

	require.Equal(t, noAccountErrorClassification{
		Status:  http.StatusServiceUnavailable,
		ErrType: "api_error",
		Message: "Service temporarily unavailable",
	}, cls)
	require.Empty(t, recorder.Header().Get("Retry-After"))
	require.Empty(t, recorder.Header().Get("anthropic-ratelimit-unified-status"))
}

func TestClassifyNoAccountError_ModelNotFoundWinsOverUsageLimit(t *testing.T) {
	c, recorder := usageLimitTestContext("/v1/messages")
	apiKey := &service.APIKey{GroupID: ptrInt64(7)}
	diag := &fakeDiagnoser{resp: service.ModelAvailabilityDiagnosis{
		HasAccountsInPool: true,
		HasModelSupport:   false,
		UsageExhaustion:   &service.UsageExhaustion{ResetAt: time.Now().Add(time.Hour), Window: "5h"},
	}}

	cls := classifyNoAccountErrorFromGin(c, diag, apiKey, "made-up-model", "made-up-model", service.PlatformAnthropic)
	cls = classifySelectionFailureError(fmt.Errorf("no available accounts (model_rate_limited=2)"), cls)

	require.Equal(t, http.StatusNotFound, cls.Status)
	require.Equal(t, "model_not_found", cls.ErrType)
	require.True(t, cls.ModelNotFound)
	require.Nil(t, cls.UsageLimit)
	require.Empty(t, recorder.Header().Get("Retry-After"))
	require.Empty(t, recorder.Header().Get("anthropic-ratelimit-unified-status"))
}

func TestClassifySelectionFailureError_KeepsUsageLimitVerdict(t *testing.T) {
	usageLimit := usageLimitClassification(service.UsageExhaustion{ResetAt: time.Now().Add(time.Hour)}, time.Now())

	got := classifySelectionFailureError(fmt.Errorf("no available accounts (model_rate_limited=2)"), usageLimit)

	require.Equal(t, usageLimit, got, "the reset time must not be replaced by the generic rate-limit message")
}

func TestOpenAIResponsesNoAccount_UsageLimitBody(t *testing.T) {
	resetAt := time.Now().Add(90 * time.Minute).Truncate(time.Second)
	c, recorder := usageLimitTestContext("/v1/responses")
	apiKey := &service.APIKey{GroupID: ptrInt64(7)}

	cls := classifyNoAccountErrorFromGin(c, usageLimitTestDiagnoser(&service.UsageExhaustion{ResetAt: resetAt}), apiKey, "gpt-5.5", "gpt-5.5", service.PlatformOpenAI)
	(&OpenAIGatewayHandler{}).respondNoAccount(c, cls, false)

	require.Equal(t, http.StatusTooManyRequests, recorder.Code)
	requireRetryAfterNear(t, recorder.Header(), 90*time.Minute)
	require.Empty(t, recorder.Header().Get("anthropic-ratelimit-unified-status"), "Anthropic headers belong to the Messages protocol only")
	body := recorder.Body.Bytes()
	require.Equal(t, "usage_limit_reached", gjson.GetBytes(body, "error.type").String())
	require.Equal(t, resetAt.Unix(), gjson.GetBytes(body, "error.resets_at").Int())
	require.InDelta(t, (90 * time.Minute).Seconds(), gjson.GetBytes(body, "error.resets_in_seconds").Int(), 5)
	require.Contains(t, gjson.GetBytes(body, "error.message").String(), resetAt.UTC().Format(time.RFC3339))
}

func TestOpenAIResponsesNoAccount_StartedStreamKeepsInStreamError(t *testing.T) {
	resetAt := time.Now().Add(time.Hour).Truncate(time.Second)
	c, recorder := usageLimitTestContext("/v1/responses")
	cls := usageLimitClassification(service.UsageExhaustion{ResetAt: resetAt}, time.Now())

	(&OpenAIGatewayHandler{}).respondNoAccount(c, cls, true)

	require.Contains(t, recorder.Body.String(), "response.failed")
	require.Contains(t, recorder.Body.String(), resetAt.UTC().Format(time.RFC3339))
	require.NotContains(t, recorder.Body.String(), "resets_at")
}

func TestOpenAIResponsesNoAccount_NoKnownResetUnchanged(t *testing.T) {
	c, recorder := usageLimitTestContext("/v1/responses")
	apiKey := &service.APIKey{GroupID: ptrInt64(7)}

	cls := classifyNoAccountErrorFromGin(c, usageLimitTestDiagnoser(nil), apiKey, "gpt-5.5", "gpt-5.5", service.PlatformOpenAI)
	(&OpenAIGatewayHandler{}).respondNoAccount(c, cls, false)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Empty(t, recorder.Header().Get("Retry-After"))
	require.JSONEq(t, `{"error":{"type":"api_error","message":"Service temporarily unavailable"}}`, recorder.Body.String())
}

func TestClassifyNoAccountError_UsageLimitOnGeminiSetsRetryAfterOnly(t *testing.T) {
	resetAt := time.Now().Add(30 * time.Minute)
	c, recorder := usageLimitTestContext("/v1beta/models/gemini-2.5-pro:generateContent")
	apiKey := &service.APIKey{GroupID: ptrInt64(7)}

	cls := classifyNoAccountErrorFromGin(c, usageLimitTestDiagnoser(&service.UsageExhaustion{ResetAt: resetAt}), apiKey, "gemini-2.5-pro", "gemini-2.5-pro", service.PlatformGemini)

	require.Equal(t, http.StatusTooManyRequests, cls.Status)
	requireRetryAfterNear(t, recorder.Header(), 30*time.Minute)
	require.Len(t, recorder.Header(), 1, "Gemini has no documented usage-limit header contract beyond Retry-After")
}

func TestClassifyUsageLimitAfterUpstream429_OnlyFor429WithKnownReset(t *testing.T) {
	resetAt := time.Now().Add(time.Hour)
	apiKey := &service.APIKey{GroupID: ptrInt64(7)}
	exhausted := usageLimitTestDiagnoser(&service.UsageExhaustion{ResetAt: resetAt})
	upstream429 := &service.UpstreamFailoverError{StatusCode: http.StatusTooManyRequests}

	c, recorder := usageLimitTestContext("/v1/messages")
	cls, ok := classifyUsageLimitAfterUpstream429(c, exhausted, apiKey, "claude-sonnet-4-6", service.PlatformAnthropic, upstream429)
	require.True(t, ok)
	require.Equal(t, http.StatusTooManyRequests, cls.Status)
	require.Equal(t, "rejected", recorder.Header().Get("anthropic-ratelimit-unified-status"))

	for name, tc := range map[string]struct {
		diag    *fakeDiagnoser
		lastErr *service.UpstreamFailoverError
	}{
		"no upstream error":      {exhausted, nil},
		"upstream 529":           {exhausted, &service.UpstreamFailoverError{StatusCode: 529}},
		"pool has no known time": {usageLimitTestDiagnoser(nil), upstream429},
	} {
		t.Run(name, func(t *testing.T) {
			c, recorder := usageLimitTestContext("/v1/messages")
			_, ok := classifyUsageLimitAfterUpstream429(c, tc.diag, apiKey, "claude-sonnet-4-6", service.PlatformAnthropic, tc.lastErr)
			require.False(t, ok)
			require.Empty(t, recorder.Header())
		})
	}
}

// usageLimitPoolRepo lets the Messages harness answer the persistent-pool
// query the no-account classifier runs.
type usageLimitPoolRepo struct {
	*claudeOAuthPoolRepo
}

func (r *usageLimitPoolRepo) ListModelAvailabilityCandidates(context.Context, *int64, []string, bool) ([]service.Account, error) {
	accounts := make([]service.Account, 0, len(r.accounts))
	for _, account := range r.accounts {
		accounts = append(accounts, *account)
	}
	return accounts, nil
}

// usageLimitUpstream answers every request with a five-hour usage-limit 429
// that resets at a fixed time.
type usageLimitUpstream struct {
	service.HTTPUpstream
	resetAt time.Time
	ids     []int64
}

func (u *usageLimitUpstream) DoWithTLS(req *http.Request, _ string, id int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	u.ids = append(u.ids, id)
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("anthropic-ratelimit-unified-5h-status", "rejected")
	headers.Set("anthropic-ratelimit-unified-5h-reset", strconv.FormatInt(u.resetAt.Unix(), 10))
	headers.Set("anthropic-ratelimit-unified-5h-utilization", "1")
	body := `{"type":"error","error":{"type":"rate_limit_error","message":"five-hour usage exhausted"}}`
	return &http.Response{StatusCode: http.StatusTooManyRequests, Header: headers, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
}

type usageLimitGatewayHarness struct {
	handler  *GatewayHandler
	upstream *usageLimitUpstream
	group    *service.Group
}

func newUsageLimitGatewayHarness(t *testing.T, upstreamResetAt time.Time, accounts []*service.Account) *usageLimitGatewayHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)
	group := &service.Group{ID: 70, Hydrated: true, Platform: service.PlatformAnthropic, Status: service.StatusActive, RateMultiplier: 1}
	repo := &usageLimitPoolRepo{&claudeOAuthPoolRepo{&anthropicAPIHandlerAccountRepo{accounts: accounts}}}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	cfg.Default.RateMultiplier = 1
	upstream := &usageLimitUpstream{resetAt: upstreamResetAt}
	usageRepo := &openAIWSUsageHandlerUsageLogRepoStub{created: make(chan *service.UsageLog, 2)}
	rateLimits := service.NewRateLimitService(repo, usageRepo, cfg, nil, nil)
	gateway := service.NewGatewayService(repo, &fakeGroupRepo{group: group}, usageRepo, nil, nil, nil, nil, nil, cfg,
		service.NewSchedulerSnapshotService(&fakeSchedulerCache{accounts: accounts}, nil, nil, nil, nil), nil, service.NewBillingService(cfg, nil), rateLimits, nil, nil, upstream, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	billingCache := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billingCache.Stop)
	return &usageLimitGatewayHarness{
		handler:  &GatewayHandler{gatewayService: gateway, billingCacheService: billingCache, cfg: cfg, maxAccountSwitches: 1, concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(&fakeConcurrencyCache{}), SSEPingFormatClaude, 0), usageRecordWorkerPool: newUsageRecordTestPool(t)},
		upstream: upstream,
		group:    group,
	}
}

func (h *usageLimitGatewayHarness) request(path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)).WithContext(context.WithValue(context.Background(), ctxkey.Group, h.group))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "claude-cli/2.1.300")
	key := &service.APIKey{ID: 75, UserID: 76, GroupID: &h.group.ID, Group: h.group, Status: service.StatusActive, User: &service.User{ID: 76, Concurrency: 10, Balance: 100}}
	c.Set(string(middleware.ContextKeyAPIKey), key)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: key.UserID, Concurrency: 10})
	return c, recorder
}

func usageLimitOAuthAccount(id int64) *service.Account {
	return &service.Account{ID: id, Name: fmt.Sprintf("oauth-%d", id), Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth, Concurrency: 1, Schedulable: true, Status: service.StatusActive, Credentials: map[string]any{"access_token": fmt.Sprintf("oauth-token-%d", id)}, AccountGroups: []service.AccountGroup{{AccountID: id, GroupID: 70}}}
}

func usageLimitMessagesBody(stream bool) string {
	metadataID := service.FormatMetadataUserID(strings.Repeat("a", 64), "", "11111111-1111-4111-8111-111111111111", "2.1.300")
	return fmt.Sprintf(`{"model":"claude-sonnet-4-6","max_tokens":16,"stream":%t,"messages":[{"role":"user","content":"hello"}],"metadata":{"user_id":%q}}`, stream, metadataID)
}

func requireAnthropicFiveHourUsageLimit(t *testing.T, recorder *httptest.ResponseRecorder, resetAt time.Time) {
	t.Helper()
	require.Equal(t, http.StatusTooManyRequests, recorder.Code, recorder.Body.String())
	header := recorder.Header()
	require.Contains(t, header.Get("Content-Type"), "application/json", "the error is a plain response, not an SSE frame")
	requireRetryAfterNear(t, header, time.Until(resetAt))
	reset := strconv.FormatInt(resetAt.Unix(), 10)
	require.Equal(t, "rejected", header.Get("anthropic-ratelimit-unified-status"))
	require.Equal(t, reset, header.Get("anthropic-ratelimit-unified-reset"))
	require.Equal(t, "five_hour", header.Get("anthropic-ratelimit-unified-representative-claim"))
	require.Equal(t, "rejected", header.Get("anthropic-ratelimit-unified-5h-status"))
	require.Equal(t, reset, header.Get("anthropic-ratelimit-unified-5h-reset"))
	body := recorder.Body.Bytes()
	require.Equal(t, "error", gjson.GetBytes(body, "type").String())
	require.Equal(t, "rate_limit_error", gjson.GetBytes(body, "error.type").String())
	require.Contains(t, gjson.GetBytes(body, "error.message").String(), resetAt.UTC().Format(time.RFC3339))
}

func TestGatewayMessagesAllAccountsOutOfUsageReturnsUsageLimit(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("stream=%t", stream), func(t *testing.T) {
			later := time.Now().Add(2 * time.Hour).Truncate(time.Second)
			sooner := time.Now().Add(time.Hour).Truncate(time.Second)
			first, second := usageLimitOAuthAccount(71), usageLimitOAuthAccount(72)
			first.RateLimitResetAt = &later
			second.RateLimitResetAt = &sooner
			second.SessionWindowEnd = &sooner
			harness := newUsageLimitGatewayHarness(t, time.Time{}, []*service.Account{first, second})

			c, recorder := harness.request("/v1/messages", usageLimitMessagesBody(stream))
			harness.handler.Messages(c)

			requireAnthropicFiveHourUsageLimit(t, recorder, sooner)
			require.Empty(t, harness.upstream.ids, "no upstream request is made")
		})
	}
}

func TestGatewayMessagesUpstream429ExhaustingPoolReturnsUsageLimit(t *testing.T) {
	// The only account's upstream answers 429 for its five-hour window, so
	// the request that discovers the limit must already carry the reset. The
	// window end is on the account from an earlier successful response.
	resetAt := time.Now().Add(time.Hour).Truncate(time.Second)
	account := usageLimitOAuthAccount(71)
	account.SessionWindowEnd = &resetAt
	harness := newUsageLimitGatewayHarness(t, resetAt, []*service.Account{account})

	c, recorder := harness.request("/v1/messages", usageLimitMessagesBody(true))
	harness.handler.Messages(c)

	require.Equal(t, []int64{71}, harness.upstream.ids)
	requireAnthropicFiveHourUsageLimit(t, recorder, resetAt)
}

func TestGatewayResponsesAllAccountsOutOfUsageReturnsUsageLimit(t *testing.T) {
	later := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	sooner := time.Now().Add(time.Hour).Truncate(time.Second)
	first, second := usageLimitOAuthAccount(71), usageLimitOAuthAccount(72)
	first.RateLimitResetAt = &sooner
	second.RateLimitResetAt = &later
	harness := newUsageLimitGatewayHarness(t, time.Time{}, []*service.Account{first, second})

	c, recorder := harness.request("/v1/responses", `{"model":"claude-sonnet-4-6","input":"hello"}`)
	harness.handler.Responses(c)

	require.Equal(t, http.StatusTooManyRequests, recorder.Code, recorder.Body.String())
	requireRetryAfterNear(t, recorder.Header(), time.Until(sooner))
	require.Empty(t, recorder.Header().Get("anthropic-ratelimit-unified-status"))
	body := recorder.Body.Bytes()
	require.Equal(t, "usage_limit_reached", gjson.GetBytes(body, "error.type").String())
	require.Equal(t, sooner.Unix(), gjson.GetBytes(body, "error.resets_at").Int())
	require.InDelta(t, time.Until(sooner).Seconds(), gjson.GetBytes(body, "error.resets_in_seconds").Int(), 5)
	require.Contains(t, gjson.GetBytes(body, "error.message").String(), sooner.UTC().Format(time.RFC3339))
	require.Empty(t, harness.upstream.ids)
}
