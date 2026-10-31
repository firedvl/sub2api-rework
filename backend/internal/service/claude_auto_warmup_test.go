//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClaudeAutoWarmupQuotaReadiness(t *testing.T) {
	now := time.Now()
	zero, used, full, nan := 0.0, 12.0, 100.0, math.NaN()
	future, past := now.Add(time.Hour).Format(time.RFC3339), now.Add(-time.Minute).Format(time.RFC3339)
	for _, tc := range []struct {
		name   string
		usage  *ClaudeUsageResponse
		ready  bool
		reason string
	}{
		{"missing", nil, false, "quota_unavailable"},
		{"empty response", &ClaudeUsageResponse{}, false, "quota_unavailable"},
		{"idle", &ClaudeUsageResponse{FiveHour: ClaudeUsageWindow{Utilization: &zero}}, true, ""},
		{"active even at zero", &ClaudeUsageResponse{FiveHour: ClaudeUsageWindow{Utilization: &zero, ResetsAt: future}}, false, "waiting_new_window"},
		{"expired", &ClaudeUsageResponse{FiveHour: ClaudeUsageWindow{Utilization: &full, ResetsAt: past}}, true, ""},
		{"weekly blocked", &ClaudeUsageResponse{FiveHour: ClaudeUsageWindow{Utilization: &zero}, SevenDay: ClaudeUsageWindow{Utilization: &full, ResetsAt: future}}, false, "waiting_new_window"},
		{"invalid reset", &ClaudeUsageResponse{FiveHour: ClaudeUsageWindow{Utilization: &zero, ResetsAt: "invalid"}}, false, "quota_unavailable"},
		{"missing reset with usage", &ClaudeUsageResponse{FiveHour: ClaudeUsageWindow{Utilization: &used}}, false, "quota_unavailable"},
		{"NaN", &ClaudeUsageResponse{FiveHour: ClaudeUsageWindow{Utilization: &nan}}, false, "quota_unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ready, reason, _ := claudeAutoWarmupReady(tc.usage, now)
			require.Equal(t, tc.ready, ready)
			require.Equal(t, tc.reason, reason)
		})
	}
}

type claudeWarmupUsageStub struct {
	ClaudeUsageFetcher
	usage   *ClaudeUsageResponse
	err     error
	calls   int
	options *ClaudeUsageFetchOptions
	fetch   func(int) (*ClaudeUsageResponse, error)
}

func (s *claudeWarmupUsageStub) FetchUsageWithOptions(_ context.Context, opts *ClaudeUsageFetchOptions) (*ClaudeUsageResponse, error) {
	s.calls++
	s.options = opts
	if s.fetch != nil {
		return s.fetch(s.calls)
	}
	return s.usage, s.err
}

func TestClaudeAutoWarmupConnectionAndDurableDedup(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "uncertain_failure"}[failure], func(t *testing.T) {
			zero := 0.0
			account := &Account{ID: 321, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"access_token": "stale"}, Extra: map[string]any{OpenAIAutoWarmupEnabledExtraKey: true}}
			repo := &autoWarmupAccountRepo{accounts: map[int64]*Account{321: account}}
			claims := &autoWarmupAttemptMemoryRepo{}
			usage := &claudeWarmupUsageStub{usage: &ClaudeUsageResponse{FiveHour: ClaudeUsageWindow{Utilization: &zero}}}
			calls := 0
			upstream := &autoWarmupHTTPUpstream{do: func(req *http.Request, body []byte) (*http.Response, error) {
				calls++
				require.Equal(t, testClaudeAPIURL, req.URL.String())
				require.Equal(t, "Bearer synthetic-token", req.Header.Get("Authorization"))
				var payload map[string]any
				require.NoError(t, json.Unmarshal(body, &payload))
				require.Equal(t, float64(16), payload["max_tokens"])
				require.NotEmpty(t, payload["system"])
				if failure {
					return nil, errors.New("connection reset")
				}
				return newJSONResponse(200, claudeProbeStart+claudeProbeTerminal+claudeProbeStop), nil
			}}
			svc := NewOpenAIQuotaAutoResetService(repo, nil, nil, nil, nil, nil, nil)
			svc.warmupAttempts = claims
			svc.claudeWarmupTokens = resetTokenStub{}
			svc.claudeWarmupUsage = &AccountUsageService{accountRepo: repo, usageFetcher: usage, cache: NewUsageCache()}
			svc.claudeWarmupTest = &AccountTestService{accountRepo: repo, httpUpstream: upstream}
			require.NoError(t, svc.evaluateAccount(context.Background(), 321))
			require.Equal(t, 1, calls)
			require.Equal(t, "synthetic-token", usage.options.AccessToken)
			state := account.Extra[ClaudeAutoWarmupStateExtraKey].(*OpenAIAutoWarmupState)
			require.Equal(t, !failure, state.Status == OpenAIAutoWarmupStatusSucceeded)
			// Lose all cached status, as in a stale read or a second instance: the DB claim still wins.
			account.Extra = map[string]any{OpenAIAutoWarmupEnabledExtraKey: true}
			require.NoError(t, svc.evaluateAccount(context.Background(), 321))
			require.Equal(t, 1, calls)
			require.Equal(t, 1, claims.attemptCount())
		})
	}
}

func TestClaudeAutoWarmupDisabledAndUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name        string
		enabled     bool
		accountType string
		queryErr    error
		wantQueries int
	}{
		{"disabled", false, AccountTypeOAuth, nil, 0},
		{"API key", true, AccountTypeAPIKey, nil, 0},
		{"setup token", true, AccountTypeSetupToken, nil, 0},
		{"quota unavailable", true, AccountTypeOAuth, errors.New("usage unavailable"), 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &Account{ID: 1, Platform: PlatformAnthropic, Type: tc.accountType, Status: StatusActive, Schedulable: true, Extra: map[string]any{OpenAIAutoWarmupEnabledExtraKey: tc.enabled}}
			repo := &autoWarmupAccountRepo{accounts: map[int64]*Account{1: a}}
			usage := &claudeWarmupUsageStub{err: tc.queryErr}
			claims := &autoWarmupAttemptMemoryRepo{}
			svc := NewOpenAIQuotaAutoResetService(repo, nil, nil, nil, nil, nil, nil)
			svc.claudeWarmupTest = &AccountTestService{}
			svc.claudeWarmupUsage = &AccountUsageService{accountRepo: repo, usageFetcher: usage, cache: NewUsageCache()}
			svc.claudeWarmupTokens = resetTokenStub{}
			svc.warmupAttempts = claims
			require.NoError(t, svc.evaluateAccount(context.Background(), 1))
			require.NoError(t, svc.evaluateAccount(context.Background(), 1))
			require.Equal(t, tc.wantQueries, usage.calls)
			require.Zero(t, claims.attemptCount())
		})
	}
}

func TestClaudeAutoWarmupManagedConfiguration(t *testing.T) {
	extra, err := normalizeOpenAIAutoResetCreditExtra(PlatformAnthropic, AccountTypeOAuth, false, map[string]any{OpenAIAutoWarmupEnabledExtraKey: true, ClaudeAutoWarmupStateExtraKey: "forged", ClaudeAutoWarmupEvaluationExtraKey: "forged"})
	require.NoError(t, err)
	require.Equal(t, map[string]any{OpenAIAutoWarmupEnabledExtraKey: true}, extra)
	_, err = normalizeOpenAIAutoResetCreditExtra(PlatformAnthropic, AccountTypeOAuth, false, map[string]any{OpenAIAutoResetCreditEnabledExtraKey: true})
	require.Error(t, err)
	_, err = normalizeOpenAIAutoResetCreditExtra(PlatformAnthropic, AccountTypeOAuth, false, map[string]any{OpenAIAutoWarmupEnabledExtraKey: "true"})
	require.Error(t, err)
	for _, kind := range []string{AccountTypeAPIKey, AccountTypeSetupToken} {
		_, err = normalizeOpenAIAutoResetCreditExtra(PlatformAnthropic, kind, false, map[string]any{OpenAIAutoWarmupEnabledExtraKey: true})
		require.Error(t, err)
	}
}

func TestClaudeAutoWarmupScannerQueuesOnlyEligibleAccounts(t *testing.T) {
	enabled := map[string]any{OpenAIAutoWarmupEnabledExtraKey: true}
	parentID := int64(1)
	repo := &autoWarmupPagedAccountRepo{autoWarmupAccountRepo: &autoWarmupAccountRepo{accounts: map[int64]*Account{
		1: {ID: 1, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Extra: enabled},
		2: {ID: 2, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true},
		3: {ID: 3, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: false, Extra: enabled},
		4: {ID: 4, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Extra: enabled},
		5: {ID: 5, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, ParentAccountID: &parentID, Extra: enabled},
	}}}
	svc := NewOpenAIQuotaAutoResetService(repo, nil, nil, nil, nil, nil, nil)
	svc.claudeWarmupTest = &AccountTestService{}
	svc.claudeWarmupUsage = &AccountUsageService{accountRepo: repo, usageFetcher: &claudeWarmupUsageStub{}, cache: NewUsageCache()}
	svc.claudeWarmupTokens = resetTokenStub{}
	svc.warmupAttempts = &autoWarmupAttemptMemoryRepo{}
	svc.scanClaudeAutoWarmupAccounts(context.Background())
	require.Len(t, svc.queue, 1)
	require.Equal(t, int64(1), <-svc.queue)
}

type claudeWarmupCountdownRepo struct {
	*autoWarmupPagedAccountRepo
	sessionErr error
}

func (r *claudeWarmupCountdownRepo) UpdateSessionWindow(_ context.Context, id int64, start, end *time.Time, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sessionErr != nil {
		return r.sessionErr
	}
	a := r.accounts[id]
	a.SessionWindowStart, a.SessionWindowEnd, a.SessionWindowStatus = start, end, status
	return nil
}

func (r *claudeWarmupCountdownRepo) UpdateSessionWindowEnd(_ context.Context, id int64, end time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.accounts[id].SessionWindowEnd = &end
	return nil
}

type claudeWarmupCountdownHarness struct {
	svc     *OpenAIQuotaAutoResetService
	repo    *claudeWarmupCountdownRepo
	usage   *claudeWarmupUsageStub
	account *Account
	sends   int
	headers http.Header
	stream  string
	sendErr error
}

func newClaudeWarmupCountdownHarness(t *testing.T) *claudeWarmupCountdownHarness {
	t.Helper()
	h := &claudeWarmupCountdownHarness{headers: make(http.Header), stream: claudeProbeStart + claudeProbeTerminal + claudeProbeStop}
	h.account = &Account{ID: 53, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"access_token": "old-token"}, Extra: map[string]any{OpenAIAutoWarmupEnabledExtraKey: true}}
	h.repo = &claudeWarmupCountdownRepo{autoWarmupPagedAccountRepo: &autoWarmupPagedAccountRepo{autoWarmupAccountRepo: &autoWarmupAccountRepo{accounts: map[int64]*Account{53: h.account}}}}
	zero := 0.0
	h.usage = &claudeWarmupUsageStub{usage: &ClaudeUsageResponse{FiveHour: ClaudeUsageWindow{Utilization: &zero}}}
	h.svc = NewOpenAIQuotaAutoResetService(h.repo, nil, nil, nil, nil, nil, nil)
	h.svc.warmupAttempts = &autoWarmupAttemptMemoryRepo{}
	h.svc.claudeWarmupTokens = resetTokenStub{}
	h.svc.claudeWarmupUsage = &AccountUsageService{accountRepo: h.repo, usageFetcher: h.usage, cache: NewUsageCache(), usageLogRepo: &usageBatchLogRepoStub{}}
	h.svc.claudeWarmupTest = &AccountTestService{accountRepo: h.repo, httpUpstream: &autoWarmupHTTPUpstream{do: func(req *http.Request, _ []byte) (*http.Response, error) {
		h.sends++
		deadline, ok := req.Context().Deadline()
		require.True(t, ok)
		require.LessOrEqual(t, time.Until(deadline), openAIAutoWarmupRequestTimeout)
		if h.sendErr != nil {
			return nil, h.sendErr
		}
		response := newJSONResponse(http.StatusOK, h.stream)
		response.Header = h.headers.Clone()
		return response, nil
	}}}
	return h
}

func TestClaudeAutoWarmupResetsStartAndPersistSuccessiveCountdowns(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newClaudeWarmupCountdownHarness(t)
		expired := time.Now().Add(-time.Minute)
		h.account.RateLimitResetAt = &expired
		h.account.TempUnschedulableUntil = &expired
		h.account.SessionWindowEnd = &expired
		h.account.SessionWindowStatus = "rejected"
		zero, full := 0.0, 100.0
		reset := time.Now().Add(5 * time.Hour).Truncate(time.Second)
		h.usage.fetch = func(call int) (*ClaudeUsageResponse, error) {
			if call%2 == 1 {
				return &ClaudeUsageResponse{FiveHour: ClaudeUsageWindow{Utilization: &full, ResetsAt: expired.Format(time.RFC3339)}}, nil
			}
			return &ClaudeUsageResponse{FiveHour: ClaudeUsageWindow{Utilization: &zero, ResetsAt: reset.Format(time.RFC3339)}}, nil
		}
		evaluateScan := func() {
			h.svc.scanClaudeAutoWarmupAccounts(t.Context())
			require.Len(t, h.svc.queue, 1)
			id := <-h.svc.queue
			require.NoError(t, h.svc.evaluateAccount(t.Context(), id))
			h.svc.pending.Delete(id)
		}
		evaluateScan()
		require.Equal(t, 1, h.sends)
		state := claudeAutoWarmupStateFromExtra(h.account.Extra)
		require.True(t, state.WindowStarted)
		require.Equal(t, reset.Format(time.RFC3339), state.Observed5hResetAt)
		require.Equal(t, "allowed", h.account.SessionWindowStatus)
		require.True(t, reset.Equal(*h.account.SessionWindowEnd))
		require.Equal(t, reset.Unix(), h.account.Extra["session_window_usage_reset"])
		passive, err := h.svc.claudeWarmupUsage.GetPassiveUsage(t.Context(), 53)
		require.NoError(t, err)
		require.True(t, reset.Equal(*passive.FiveHour.ResetsAt))
		require.Equal(t, 5*60*60, passive.FiveHour.RemainingSeconds)
		cached, _ := h.svc.claudeWarmupUsage.cache.apiCache.Load(int64(53))
		require.Equal(t, reset.Format(time.RFC3339), cached.(*apiUsageCache).response.FiveHour.ResetsAt)
		evaluateScan()
		require.Equal(t, 1, h.sends)
		require.Equal(t, 2, h.usage.calls)
		time.Sleep(5*time.Hour + time.Minute)
		expired = reset
		reset = time.Now().Add(5 * time.Hour).Truncate(time.Second)
		evaluateScan()
		require.Equal(t, 2, h.sends)
		require.True(t, reset.Equal(*h.account.SessionWindowEnd))
		require.True(t, claudeAutoWarmupStateFromExtra(h.account.Extra).WindowStarted)
	})
}

func TestClaudeAutoWarmupConfirmsResponseHeadersEvenWithInterruptedStream(t *testing.T) {
	for _, interrupted := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete", true: "interrupted"}[interrupted], func(t *testing.T) {
			h := newClaudeWarmupCountdownHarness(t)
			// An upstream boundary can be rounded to the hour; do not substitute now+5h.
			reset := time.Now().Truncate(time.Hour).Add(5 * time.Hour)
			h.headers.Set("anthropic-ratelimit-unified-5h-reset", strconv.FormatInt(reset.Unix(), 10))
			h.headers.Set("anthropic-ratelimit-unified-5h-utilization", "0.001")
			h.headers.Set("anthropic-ratelimit-unified-5h-status", "allowed")
			if interrupted {
				h.stream = claudeProbeStart
			}
			require.NoError(t, h.svc.evaluateAccount(t.Context(), 53))
			state := claudeAutoWarmupStateFromExtra(h.account.Extra)
			require.True(t, state.WindowStarted)
			require.True(t, reset.Equal(*h.account.SessionWindowEnd))
			require.True(t, reset.Add(-5*time.Hour).Equal(*h.account.SessionWindowStart))
			require.Equal(t, 0.001, h.account.Extra["session_window_utilization"])
			require.Equal(t, "passive", h.account.Extra["session_window_utilization_source"])
			require.Equal(t, 1, h.usage.calls, "headers avoid an unnecessary usage request")
			_, cached := h.svc.claudeWarmupUsage.cache.apiCache.Load(int64(53))
			require.False(t, cached, "the idle preflight cache must not hide the new window")
			if interrupted {
				require.Equal(t, OpenAIAutoWarmupStatusWindowStartedWithWarning, state.Status)
			} else {
				require.Equal(t, OpenAIAutoWarmupStatusSucceeded, state.Status)
			}
		})
	}
}

func TestClaudeAutoWarmupDelayedConfirmationDoesNotResend(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newClaudeWarmupCountdownHarness(t)
		reset := time.Now().Add(5 * time.Hour)
		zero := 0.0
		h.usage.fetch = func(call int) (*ClaudeUsageResponse, error) {
			if call == 1 {
				return &ClaudeUsageResponse{FiveHour: ClaudeUsageWindow{Utilization: &zero}}, nil
			}
			if call == 2 {
				return nil, errors.New("usage endpoint temporarily unavailable")
			}
			return &ClaudeUsageResponse{FiveHour: ClaudeUsageWindow{Utilization: &zero, ResetsAt: reset.Format(time.RFC3339)}}, nil
		}
		require.NoError(t, h.svc.evaluateAccount(t.Context(), 53))
		require.False(t, claudeAutoWarmupStateFromExtra(h.account.Extra).WindowStarted)
		require.Nil(t, h.account.SessionWindowEnd, "never invent a local countdown")
		require.NoError(t, h.svc.evaluateAccount(t.Context(), 53))
		require.Equal(t, 2, h.usage.calls)
		time.Sleep(time.Minute + time.Second)
		require.NoError(t, h.svc.evaluateAccount(t.Context(), 53))
		require.Equal(t, 1, h.sends)
		require.True(t, claudeAutoWarmupStateFromExtra(h.account.Extra).WindowStarted)
		require.True(t, reset.Equal(*h.account.SessionWindowEnd))
	})
}

func TestClaudeAutoWarmupRetriesOnlyAfterFreshIdleEvidenceAndBackoff(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newClaudeWarmupCountdownHarness(t)
		h.sendErr = errors.New("upstream connection failed")
		require.NoError(t, h.svc.evaluateAccount(t.Context(), 53))
		require.Equal(t, 1, h.sends)
		require.Nil(t, h.account.SessionWindowEnd)
		time.Sleep(2 * time.Minute)
		require.NoError(t, h.svc.evaluateAccount(t.Context(), 53))
		require.Equal(t, 1, h.sends, "a failed send cannot retry rapidly")
		time.Sleep(9 * time.Minute)
		h.sendErr = nil
		zero := 0.0
		reset := time.Now().Add(5 * time.Hour)
		h.usage.fetch = func(_ int) (*ClaudeUsageResponse, error) {
			usage := &ClaudeUsageResponse{FiveHour: ClaudeUsageWindow{Utilization: &zero}}
			if h.sends >= 2 {
				usage.FiveHour.ResetsAt = reset.Format(time.RFC3339)
			}
			return usage, nil
		}
		require.NoError(t, h.svc.evaluateAccount(t.Context(), 53))
		require.Equal(t, 2, h.sends)
		require.True(t, claudeAutoWarmupStateFromExtra(h.account.Extra).WindowStarted)
		require.True(t, reset.Equal(*h.account.SessionWindowEnd))
	})
}

func TestClaudeAutoWarmupCountdownWriteFailureCannotReportConfirmation(t *testing.T) {
	h := newClaudeWarmupCountdownHarness(t)
	reset := time.Now().Truncate(time.Second).Add(5 * time.Hour)
	h.headers.Set("anthropic-ratelimit-unified-5h-reset", strconv.FormatInt(reset.Unix(), 10))
	h.repo.sessionErr = errors.New("session write failed")
	require.ErrorContains(t, h.svc.evaluateAccount(t.Context(), 53), "session write failed")
	require.False(t, claudeAutoWarmupStateFromExtra(h.account.Extra).WindowStarted)
	require.Nil(t, h.account.SessionWindowEnd)
	h.repo.sessionErr = nil
	zero := 0.0
	h.usage.usage = &ClaudeUsageResponse{FiveHour: ClaudeUsageWindow{Utilization: &zero, ResetsAt: reset.Format(time.RFC3339)}}
	require.NoError(t, h.svc.evaluateAccount(t.Context(), 53))
	require.Equal(t, 1, h.sends)
	require.True(t, claudeAutoWarmupStateFromExtra(h.account.Extra).WindowStarted)
	require.True(t, reset.Equal(*h.account.SessionWindowEnd))
}

func TestClaudeAutoWarmupHonorsUsageRetryAfter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newClaudeWarmupCountdownHarness(t)
		retryAt := time.Now().Add(time.Hour)
		h.usage.err = &ClaudeUsageRateLimitError{RetryAt: retryAt}
		require.NoError(t, h.svc.evaluateAccount(t.Context(), 53))
		time.Sleep(11 * time.Minute)
		require.NoError(t, h.svc.evaluateAccount(t.Context(), 53))
		require.Equal(t, 1, h.usage.calls, "the provider's later retry boundary overrides the local ten-minute backoff")
		require.Zero(t, h.sends)
		time.Sleep(50 * time.Minute)
		h.usage.err = nil
		reset := time.Now().Add(5 * time.Hour)
		h.headers.Set("anthropic-ratelimit-unified-5h-reset", strconv.FormatInt(reset.Unix(), 10))
		require.NoError(t, h.svc.evaluateAccount(t.Context(), 53))
		require.Equal(t, 1, h.sends)
		require.True(t, claudeAutoWarmupStateFromExtra(h.account.Extra).WindowStarted)
	})
}

func TestClaudeAutoWarmupUsesAvailableConnectionTestModel(t *testing.T) {
	h := newClaudeWarmupCountdownHarness(t)
	reset := time.Now().Truncate(time.Second).Add(5 * time.Hour)
	h.svc.claudeWarmupTest.httpUpstream = &autoWarmupHTTPUpstream{do: func(_ *http.Request, body []byte) (*http.Response, error) {
		var payload struct {
			Model string `json:"model"`
		}
		require.NoError(t, json.Unmarshal(body, &payload))
		if payload.Model == "claude-sonnet-4-5-20250929" {
			return newJSONResponse(http.StatusNotFound, `{"type":"error","error":{"type":"not_found_error","message":"model: claude-sonnet-4-5-20250929"}}`), nil
		}
		require.Equal(t, "claude-sonnet-4-6", payload.Model)
		response := newJSONResponse(http.StatusOK, claudeProbeStart+claudeProbeTerminal+claudeProbeStop)
		response.Header.Set("anthropic-ratelimit-unified-5h-reset", strconv.FormatInt(reset.Unix(), 10))
		return response, nil
	}}
	require.NoError(t, h.svc.evaluateAccount(t.Context(), h.account.ID))
	state := claudeAutoWarmupStateFromExtra(h.account.Extra)
	require.Equal(t, http.StatusOK, state.UpstreamStatus, "the dated model produces the live upstream 404")
	require.True(t, state.WindowStarted)
	require.Equal(t, "claude-sonnet-4-6", state.Model)
}
