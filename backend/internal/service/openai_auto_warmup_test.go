package service

import (
	"context"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type autoWarmupSettingRepo struct {
	SettingRepository
	mu     sync.Mutex
	values map[string]string
}

func (r *autoWarmupSettingRepo) GetAll(context.Context) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	values := make(map[string]string, len(r.values))
	for key, value := range r.values {
		values[key] = value
	}
	return values, nil
}

func (r *autoWarmupSettingRepo) SetMultiple(_ context.Context, values map[string]string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.values == nil {
		r.values = make(map[string]string)
	}
	for key, value := range values {
		r.values[key] = value
	}
	return nil
}

type autoWarmupAccountRepo struct {
	AccountRepository
	mu       sync.Mutex
	accounts map[int64]*Account
}

func (r *autoWarmupAccountRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	account := r.accounts[id]
	if account == nil {
		return nil, ErrAccountNotFound
	}
	clone := *account
	clone.Credentials = cloneOpenAIAutoResetExtra(account.Credentials)
	clone.Extra = cloneOpenAIAutoResetExtra(account.Extra)
	return &clone, nil
}

func (r *autoWarmupAccountRepo) UpdateExtra(_ context.Context, id int64, updates map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	account := r.accounts[id]
	if account == nil {
		return ErrAccountNotFound
	}
	if account.Extra == nil {
		account.Extra = make(map[string]any)
	}
	for key, value := range updates {
		account.Extra[key] = value
	}
	return nil
}

type autoWarmupAttemptMemoryRepo struct {
	mu          sync.Mutex
	nextID      int64
	attempts    []*OpenAIAutoWarmupAttempt
	completions map[int64]OpenAIAutoWarmupCompletion
}

func (r *autoWarmupAttemptMemoryRepo) Claim(_ context.Context, accountID int64, windowType string, resetAt time.Time) (*OpenAIAutoWarmupAttempt, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, attempt := range r.attempts {
		if attempt.AccountID == accountID && attempt.WindowType == windowType && absDuration(attempt.ResetAt.Sub(resetAt)) <= time.Minute {
			return nil, false, nil
		}
	}
	r.nextID++
	attempt := &OpenAIAutoWarmupAttempt{
		ID: r.nextID, AccountID: accountID, WindowType: windowType,
		ResetAt: resetAt.UTC(), AttemptedAt: time.Now().UTC(), Status: OpenAIAutoWarmupStatusPending,
	}
	r.attempts = append(r.attempts, attempt)
	return attempt, true, nil
}

func (r *autoWarmupAttemptMemoryRepo) ClaimDormant(_ context.Context, accountID int64, windowType string, resetAt time.Time, retryAfter time.Duration) (*OpenAIAutoWarmupAttempt, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	for _, attempt := range r.attempts {
		if attempt.AccountID == accountID && attempt.WindowType == windowType && now.Sub(attempt.AttemptedAt) < retryAfter {
			return nil, false, nil
		}
	}
	r.nextID++
	attempt := &OpenAIAutoWarmupAttempt{
		ID: r.nextID, AccountID: accountID, WindowType: windowType,
		ResetAt: resetAt.UTC(), AttemptedAt: now, Status: OpenAIAutoWarmupStatusPending,
	}
	r.attempts = append(r.attempts, attempt)
	return attempt, true, nil
}

func (r *autoWarmupAttemptMemoryRepo) Complete(_ context.Context, attemptID int64, completion OpenAIAutoWarmupCompletion) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.completions == nil {
		r.completions = make(map[int64]OpenAIAutoWarmupCompletion)
	}
	r.completions[attemptID] = completion
	return nil
}

func (r *autoWarmupAttemptMemoryRepo) attemptCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.attempts)
}

type autoWarmupSenderStub struct {
	err     error
	result  *OpenAIAutoWarmupResult
	release <-chan struct{}
	started chan<- struct{}
	calls   atomic.Int32
	current atomic.Int32
	max     atomic.Int32
}

type autoWarmupHTTPUpstream struct {
	request  *http.Request
	body     []byte
	proxyURL string
	do       func(*http.Request, []byte) (*http.Response, error)
}

func (u *autoWarmupHTTPUpstream) Do(req *http.Request, proxyURL string, _ int64, _ int) (*http.Response, error) {
	u.request = req
	u.proxyURL = proxyURL
	if req != nil && req.Body != nil {
		u.body, _ = io.ReadAll(req.Body)
		_ = req.Body.Close()
		req.Body = io.NopCloser(strings.NewReader(string(u.body)))
	}
	return u.do(req, u.body)
}

func (u *autoWarmupHTTPUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func (s *autoWarmupSenderStub) SendOpenAIAutoWarmup(context.Context, int64) (*OpenAIAutoWarmupResult, error) {
	s.calls.Add(1)
	current := s.current.Add(1)
	defer s.current.Add(-1)
	for {
		observed := s.max.Load()
		if current <= observed || s.max.CompareAndSwap(observed, current) {
			break
		}
	}
	if s.started != nil {
		s.started <- struct{}{}
	}
	if s.release != nil {
		<-s.release
	}
	if s.result != nil {
		return s.result, s.err
	}
	return &OpenAIAutoWarmupResult{
		Model: "gpt-test", RequestID: "req-test", ResponseID: "resp-test",
		Usage: OpenAIUsage{InputTokens: 5, OutputTokens: 1}, Latency: 10 * time.Millisecond,
	}, s.err
}

func TestOpenAIAutoWarmupPersistsWindowStartedWarningWithoutSchemaChange(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	used := 1.0
	account := newAutoWarmupTestAccount(90, now)
	repo := &autoWarmupAccountRepo{accounts: map[int64]*Account{account.ID: account}}
	attempts := &autoWarmupAttemptMemoryRepo{}
	sender := &autoWarmupSenderStub{
		err: infraerrors.New(http.StatusBadGateway, "OPENAI_AUTO_WARMUP_RESPONSE_UNEXPECTED", "missing terminal response"),
		result: &OpenAIAutoWarmupResult{
			Model: "gpt-5.4-mini", RequestID: "req-warning", UpstreamStatus: http.StatusOK,
			WindowStarted: true, Observed5hResetAt: now.Add(5 * time.Hour).Format(time.RFC3339), Observed5hUsedPercent: &used,
		},
	}
	newAutoWarmupTestService(t, repo, attempts, sender, true).
		maybeWarmFreshOpenAIWindow(context.Background(), account, newAutoWarmupTestUsage(5*time.Hour, 0), false, now)

	require.Equal(t, OpenAIAutoWarmupStatusFailed, attempts.completions[1].Status)
	require.Equal(t, "OPENAI_AUTO_WARMUP_WINDOW_STARTED_WARNING", attempts.completions[1].ErrorCode)
	stored, err := repo.GetByID(context.Background(), account.ID)
	require.NoError(t, err)
	state, ok := stored.Extra[OpenAIAutoWarmupStateExtraKey].(*OpenAIAutoWarmupState)
	require.True(t, ok)
	require.Equal(t, OpenAIAutoWarmupStatusWindowStartedWithWarning, state.Status)
	require.True(t, state.WindowStarted)
	require.Equal(t, http.StatusOK, state.UpstreamStatus)
	require.Equal(t, "req-warning", state.RequestID)
}

func absDuration(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}

func newAutoWarmupTestAccount(id int64, now time.Time) *Account {
	return &Account{
		ID: id, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Status: StatusActive, Schedulable: true,
		Credentials: map[string]any{"access_token": "synthetic-token", "chatgpt_account_id": "synthetic-account"},
		Extra: map[string]any{
			OpenAIAutoWarmupEnabledExtraKey: true,
			"codex_5h_used_percent":         20.0,
			"codex_5h_window_minutes":       300,
			"codex_5h_reset_at":             now.Add(-time.Minute).UTC().Format(time.RFC3339),
			"codex_usage_updated_at":        now.Add(-time.Minute).UTC().Format(time.RFC3339),
		},
	}
}

func newAutoWarmupTestUsage(resetAfter time.Duration, used float64) *OpenAIQuotaUsage {
	return &OpenAIQuotaUsage{RateLimit: &OpenAIRateLimit{
		Allowed: true, LimitReached: false, allowedPresent: true, limitReachedPresent: true,
		PrimaryWindow: &OpenAIRateLimitWindow{
			UsedPercent: used, usedPercentPresent: true,
			LimitWindowSeconds: int64((5 * time.Hour).Seconds()), ResetAfterSeconds: int64(resetAfter.Seconds()),
		},
	}}
}

func setDormantOpenAIObservation(account *Account, observedAt time.Time) {
	account.Extra["codex_5h_used_percent"] = 0.0
	account.Extra["codex_5h_window_minutes"] = 300
	account.Extra["codex_5h_reset_at"] = observedAt.Add(5 * time.Hour).UTC().Format(time.RFC3339)
	account.Extra["codex_usage_updated_at"] = observedAt.UTC().Format(time.RFC3339)
}

func newAutoWarmupTestService(t testing.TB, accountRepo AccountRepository, attempts OpenAIAutoWarmupAttemptRepository, sender openAIAutoWarmupSender, global bool) *OpenAIQuotaAutoResetService {
	t.Helper()
	settingService, _ := newAutoWarmupTestSettingService(t, global)
	service := NewOpenAIQuotaAutoResetService(accountRepo, nil, nil, nil, nil, settingService, nil)
	service.SetAutoWarmup(attempts, sender)
	return service
}

func newAutoWarmupTestSettingService(t testing.TB, enabled bool) (*SettingService, *autoWarmupSettingRepo) {
	t.Helper()
	original := xai.RuntimeModelMappingOptions()
	t.Cleanup(func() { xai.SetRuntimeModelMappingOptions(original) })
	settingRepo := &autoWarmupSettingRepo{values: map[string]string{SettingKeyOpenAIAutoWarmupEnabled: "false"}}
	if enabled {
		settingRepo.values[SettingKeyOpenAIAutoWarmupEnabled] = "true"
	}
	return NewSettingService(settingRepo, &config.Config{}), settingRepo
}

func TestAssessOpenAIAutoWarmupWindowRequiresFreshPrimaryWindow(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	account := newAutoWarmupTestAccount(1, now)
	usage := newAutoWarmupTestUsage(5*time.Hour, 0)

	window, ok := assessOpenAIAutoWarmupWindow(account, usage, false, now)
	require.True(t, ok)
	require.Equal(t, now.Add(5*time.Hour), window.resetAt)

	tests := []struct {
		name      string
		mutate    func(*Account, *OpenAIQuotaUsage)
		recovered bool
	}{
		{name: "timestamp jitter", mutate: func(a *Account, u *OpenAIQuotaUsage) {
			a.Extra["codex_5h_reset_at"] = now.Add(5*time.Hour - 30*time.Second).Format(time.RFC3339)
		}},
		{name: "old window not crossed", mutate: func(a *Account, u *OpenAIQuotaUsage) {
			a.Extra["codex_5h_reset_at"] = now.Add(time.Hour).Format(time.RFC3339)
		}},
		{name: "provider denied", mutate: func(a *Account, u *OpenAIQuotaUsage) { u.RateLimit.Allowed = false }},
		{name: "quota exhausted", mutate: func(a *Account, u *OpenAIQuotaUsage) { u.RateLimit.LimitReached = true }},
		{name: "wrong window", mutate: func(a *Account, u *OpenAIQuotaUsage) {
			u.RateLimit.PrimaryWindow.LimitWindowSeconds = int64(time.Hour.Seconds())
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := newAutoWarmupTestAccount(1, now)
			fresh := newAutoWarmupTestUsage(5*time.Hour, 0)
			test.mutate(candidate, fresh)
			_, eligible := assessOpenAIAutoWarmupWindow(candidate, fresh, test.recovered, now)
			require.False(t, eligible)
		})
	}

	account.Extra["codex_5h_reset_at"] = now.Add(time.Hour).Format(time.RFC3339)
	_, ok = assessOpenAIAutoWarmupWindow(account, usage, true, now)
	require.True(t, ok, "confirmed quota recovery may prove the old window ended before its stored reset")
}

func TestOpenAIAutoWarmupClassifiesDormantButNotAnchoredWindow(t *testing.T) {
	t0 := time.Now().UTC().Truncate(time.Second)
	t1 := t0.Add(time.Minute)

	t.Run("sliding observation after minimum gap", func(t *testing.T) {
		account := newAutoWarmupTestAccount(1, t0)
		setDormantOpenAIObservation(account, t0)
		window, ok := assessOpenAIAutoWarmupWindow(account, newAutoWarmupTestUsage(5*time.Hour, 0), false, t0.Add(45*time.Second))
		require.True(t, ok)
		require.True(t, window.dormant)
	})

	t.Run("sliding idle window is warmed once then anchored window is left alone", func(t *testing.T) {
		account := newAutoWarmupTestAccount(1, t0)
		setDormantOpenAIObservation(account, t0)
		repo := &autoWarmupAccountRepo{accounts: map[int64]*Account{account.ID: account}}
		attempts := &autoWarmupAttemptMemoryRepo{}
		sender := &autoWarmupSenderStub{}
		service := newAutoWarmupTestService(t, repo, attempts, sender, true)

		window, ok := assessOpenAIAutoWarmupWindow(account, newAutoWarmupTestUsage(5*time.Hour, 0), false, t1)
		require.True(t, ok)
		require.True(t, window.dormant)
		service.maybeWarmFreshOpenAIWindow(context.Background(), account, newAutoWarmupTestUsage(5*time.Hour, 0), false, t1)
		require.Equal(t, int32(1), sender.calls.Load())

		anchoredReset := t1.Add(5 * time.Hour)
		setDormantOpenAIObservation(account, t1)
		account.Extra["codex_5h_reset_at"] = anchoredReset.Format(time.RFC3339)
		t2 := t1.Add(time.Minute)
		service.maybeWarmFreshOpenAIWindow(context.Background(), account, newAutoWarmupTestUsage(anchoredReset.Sub(t2), 0), false, t2)
		require.Equal(t, int32(1), sender.calls.Load())
		require.Equal(t, 1, attempts.attemptCount())
	})

	for _, jitter := range []time.Duration{0, -5 * time.Second, 5 * time.Second, 30 * time.Second} {
		t.Run("anchored reset jitter "+jitter.String(), func(t *testing.T) {
			account := newAutoWarmupTestAccount(2, t0)
			setDormantOpenAIObservation(account, t0)
			fixedReset := t0.Add(5 * time.Hour)
			usage := newAutoWarmupTestUsage(fixedReset.Add(jitter).Sub(t1), 0)
			window, ok := assessOpenAIAutoWarmupWindow(account, usage, false, t1)
			require.False(t, ok)
			require.False(t, window.dormant)
		})
	}
}

func TestOpenAIAutoWarmupDormantWindowGates(t *testing.T) {
	t0 := time.Now().UTC().Truncate(time.Second)
	t1 := t0.Add(time.Minute)
	tests := []struct {
		name   string
		global bool
		mutate func(*Account, *OpenAIQuotaUsage)
	}{
		{name: "global off", global: false},
		{name: "account off", global: true, mutate: func(a *Account, _ *OpenAIQuotaUsage) { a.Extra[OpenAIAutoWarmupEnabledExtraKey] = false }},
		{name: "shadow", global: true, mutate: func(a *Account, _ *OpenAIQuotaUsage) { parent := int64(99); a.ParentAccountID = &parent }},
		{name: "inactive", global: true, mutate: func(a *Account, _ *OpenAIQuotaUsage) { a.Status = StatusDisabled }},
		{name: "unschedulable", global: true, mutate: func(a *Account, _ *OpenAIQuotaUsage) { a.Schedulable = false }},
		{name: "temporary cooldown", global: true, mutate: func(a *Account, _ *OpenAIQuotaUsage) {
			until := time.Now().Add(time.Hour)
			a.TempUnschedulableUntil = &until
		}},
		{name: "rate limited", global: true, mutate: func(a *Account, _ *OpenAIQuotaUsage) { reset := t1.Add(time.Hour); a.RateLimitResetAt = &reset }},
		{name: "exhausted", global: true, mutate: func(_ *Account, u *OpenAIQuotaUsage) { u.RateLimit.LimitReached = true }},
		{name: "malformed", global: true, mutate: func(_ *Account, u *OpenAIQuotaUsage) { u.RateLimit.PrimaryWindow.UsedPercent = math.NaN() }},
		{name: "missing utilization", global: true, mutate: func(_ *Account, u *OpenAIQuotaUsage) { u.RateLimit.PrimaryWindow.usedPercentPresent = false }},
		{name: "wrong primary window", global: true, mutate: func(_ *Account, u *OpenAIQuotaUsage) {
			u.RateLimit.PrimaryWindow.LimitWindowSeconds = int64(time.Hour.Seconds())
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			account := newAutoWarmupTestAccount(1, t0)
			setDormantOpenAIObservation(account, t0)
			usage := newAutoWarmupTestUsage(5*time.Hour, 0)
			if test.mutate != nil {
				test.mutate(account, usage)
			}
			sender := &autoWarmupSenderStub{}
			service := newAutoWarmupTestService(t, &autoWarmupAccountRepo{accounts: map[int64]*Account{account.ID: account}}, &autoWarmupAttemptMemoryRepo{}, sender, test.global)
			service.maybeWarmFreshOpenAIWindow(context.Background(), account, usage, false, t1)
			require.Zero(t, sender.calls.Load())
		})
	}
}

func TestOpenAIAutoWarmupFlagsDedupFailureAndRestart(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	usage := newAutoWarmupTestUsage(5*time.Hour, 0)

	t.Run("both flags are required", func(t *testing.T) {
		account := newAutoWarmupTestAccount(1, now)
		attempts := &autoWarmupAttemptMemoryRepo{}
		sender := &autoWarmupSenderStub{}
		newAutoWarmupTestService(t, &autoWarmupAccountRepo{accounts: map[int64]*Account{1: account}}, attempts, sender, false).
			maybeWarmFreshOpenAIWindow(context.Background(), account, usage, false, now)
		require.Zero(t, sender.calls.Load())

		delete(account.Extra, OpenAIAutoWarmupEnabledExtraKey)
		newAutoWarmupTestService(t, &autoWarmupAccountRepo{accounts: map[int64]*Account{1: account}}, attempts, sender, true).
			maybeWarmFreshOpenAIWindow(context.Background(), account, usage, false, now)
		require.Zero(t, sender.calls.Load())
	})

	t.Run("same window, restart, and jitter stay deduplicated", func(t *testing.T) {
		account := newAutoWarmupTestAccount(2, now)
		repo := &autoWarmupAccountRepo{accounts: map[int64]*Account{2: account}}
		attempts := &autoWarmupAttemptMemoryRepo{}
		sender := &autoWarmupSenderStub{}
		service := newAutoWarmupTestService(t, repo, attempts, sender, true)
		service.maybeWarmFreshOpenAIWindow(context.Background(), account, usage, false, now)
		service.maybeWarmFreshOpenAIWindow(context.Background(), account, usage, false, now)
		newAutoWarmupTestService(t, repo, attempts, sender, true).
			maybeWarmFreshOpenAIWindow(context.Background(), account, usage, false, now.Add(30*time.Second))
		require.Equal(t, int32(1), sender.calls.Load())
		require.Equal(t, 1, attempts.attemptCount())

		newAutoWarmupTestService(t, repo, attempts, sender, true).
			maybeWarmFreshOpenAIWindow(context.Background(), account, usage, false, now.Add(5*time.Hour))
		require.Equal(t, int32(2), sender.calls.Load())
		require.Equal(t, 2, attempts.attemptCount())
	})

	t.Run("failure is persisted without a tight retry", func(t *testing.T) {
		account := newAutoWarmupTestAccount(3, now)
		repo := &autoWarmupAccountRepo{accounts: map[int64]*Account{3: account}}
		attempts := &autoWarmupAttemptMemoryRepo{}
		sender := &autoWarmupSenderStub{err: errors.New("synthetic failure")}
		service := newAutoWarmupTestService(t, repo, attempts, sender, true)
		service.maybeWarmFreshOpenAIWindow(context.Background(), account, usage, false, now)
		service.maybeWarmFreshOpenAIWindow(context.Background(), account, usage, false, now)

		require.Equal(t, int32(1), sender.calls.Load())
		require.Equal(t, OpenAIAutoWarmupStatusFailed, attempts.completions[1].Status)
		stored, err := repo.GetByID(context.Background(), account.ID)
		require.NoError(t, err)
		state, ok := stored.Extra[OpenAIAutoWarmupStateExtraKey].(*OpenAIAutoWarmupState)
		require.True(t, ok)
		require.Equal(t, OpenAIAutoWarmupStatusFailed, state.Status)
	})

	t.Run("sliding reset drift stays deduplicated across scanner and service restarts", func(t *testing.T) {
		t0 := now
		account := newAutoWarmupTestAccount(4, t0)
		setDormantOpenAIObservation(account, t0)
		repo := &autoWarmupAccountRepo{accounts: map[int64]*Account{account.ID: account}}
		attempts := &autoWarmupAttemptMemoryRepo{}
		sender := &autoWarmupSenderStub{err: errors.New("synthetic failure")}
		newAutoWarmupTestService(t, repo, attempts, sender, true).
			maybeWarmFreshOpenAIWindow(context.Background(), account, newAutoWarmupTestUsage(5*time.Hour, 0), false, t0.Add(time.Minute))
		newAutoWarmupTestService(t, repo, attempts, sender, true).
			maybeWarmFreshOpenAIWindow(context.Background(), account, newAutoWarmupTestUsage(5*time.Hour, 0), false, t0.Add(2*time.Minute))

		require.Equal(t, int32(1), sender.calls.Load())
		require.Equal(t, 1, attempts.attemptCount())
		require.Equal(t, OpenAIAutoWarmupStatusFailed, attempts.completions[1].Status)

		attempts.mu.Lock()
		attempts.attempts[0].AttemptedAt = time.Now().UTC().Add(-openAIAutoWarmupDormantRetry - time.Minute)
		attempts.mu.Unlock()
		newAutoWarmupTestService(t, repo, attempts, sender, true).
			maybeWarmFreshOpenAIWindow(context.Background(), account, newAutoWarmupTestUsage(5*time.Hour, 0), false, t0.Add(3*time.Minute))
		require.Equal(t, int32(2), sender.calls.Load())
		require.Equal(t, 2, attempts.attemptCount())
	})
}

func TestOpenAIAutoWarmupConcurrencyIsBounded(t *testing.T) {
	t0 := time.Now().UTC().Truncate(time.Second)
	now := t0.Add(time.Minute)
	release := make(chan struct{})
	started := make(chan struct{}, 8)
	sender := &autoWarmupSenderStub{release: release, started: started}
	attempts := &autoWarmupAttemptMemoryRepo{}
	accounts := make(map[int64]*Account, 8)
	for id := int64(1); id <= 8; id++ {
		accounts[id] = newAutoWarmupTestAccount(id, t0)
		setDormantOpenAIObservation(accounts[id], t0)
	}
	service := newAutoWarmupTestService(t, &autoWarmupAccountRepo{accounts: accounts}, attempts, sender, true)

	var wg sync.WaitGroup
	for _, account := range accounts {
		wg.Add(1)
		go func(account *Account) {
			defer wg.Done()
			service.maybeWarmFreshOpenAIWindow(context.Background(), account, newAutoWarmupTestUsage(5*time.Hour, 0), false, now)
		}(account)
	}
	for range 4 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("four warm-ups did not enter the sender")
		}
	}
	select {
	case <-started:
		t.Fatal("more than four warm-ups entered concurrently")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	wg.Wait()
	require.Equal(t, int32(8), sender.calls.Load())
	require.LessOrEqual(t, sender.max.Load(), int32(4))
}

func TestOpenAIAutoWarmupRechecksFlagsAfterClaimBeforeSend(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	for _, flag := range []string{"global", "account"} {
		t.Run(flag, func(t *testing.T) {
			account := newAutoWarmupTestAccount(1, now)
			accountRepo := &autoWarmupAccountRepo{accounts: map[int64]*Account{account.ID: account}}
			attempts := &autoWarmupAttemptMemoryRepo{}
			sender := &autoWarmupSenderStub{}
			settingService, settingRepo := newAutoWarmupTestSettingService(t, true)
			service := NewOpenAIQuotaAutoResetService(accountRepo, nil, nil, nil, nil, settingService, nil)
			service.SetAutoWarmup(attempts, sender)
			for range cap(service.warmupSlots) {
				service.warmupSlots <- struct{}{}
			}

			done := make(chan struct{})
			go func() {
				defer close(done)
				service.maybeWarmFreshOpenAIWindow(context.Background(), account, newAutoWarmupTestUsage(5*time.Hour, 0), false, now)
			}()
			require.Eventually(t, func() bool { return attempts.attemptCount() == 1 }, time.Second, time.Millisecond)
			if flag == "global" {
				require.NoError(t, settingRepo.SetMultiple(context.Background(), map[string]string{SettingKeyOpenAIAutoWarmupEnabled: "false"}))
			} else {
				require.NoError(t, accountRepo.UpdateExtra(context.Background(), account.ID, map[string]any{OpenAIAutoWarmupEnabledExtraKey: false}))
			}
			<-service.warmupSlots
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("queued warm-up did not stop after its switch was disabled")
			}

			require.Zero(t, sender.calls.Load())
			require.Equal(t, OpenAIAutoWarmupStatusFailed, attempts.completions[1].Status)
			if flag == "global" {
				require.Equal(t, "OPENAI_AUTO_WARMUP_DISABLED", attempts.completions[1].ErrorCode)
			} else {
				require.Equal(t, "OPENAI_AUTO_WARMUP_ACCOUNT_UNSAFE", attempts.completions[1].ErrorCode)
			}
		})
	}
}

func TestOpenAIAutoWarmupRunsAfterQuotaRecovery(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	account, _ := quotaRecoveryTestAccount(t, now, now.Add(4*time.Hour), "5h")
	account.Extra[OpenAIAutoWarmupEnabledExtraKey] = true
	account.Extra["codex_5h_used_percent"] = 100.0
	account.Extra["codex_5h_window_minutes"] = 300
	account.Extra["codex_5h_reset_at"] = now.Add(time.Hour).Format(time.RFC3339)
	repo := &quotaRecoveryAccountRepoStub{account: account, clearResult: true}
	usage := quotaRecoveryTestUsage(20, 10)
	usage.RateLimit.PrimaryWindow.ResetAfterSeconds = int64((5 * time.Hour).Seconds())
	attempts := &autoWarmupAttemptMemoryRepo{}
	sender := &autoWarmupSenderStub{}
	service := newAutoWarmupTestService(t, repo, attempts, sender, true)
	service.quota = &quotaRecoveryQuotaStub{usage: usage}
	service.recoverer = &quotaRecoveryRuntimeStub{}
	service.SetAutoWarmup(attempts, sender)

	require.NoError(t, service.evaluateAccount(context.Background(), account.ID))
	require.Equal(t, 1, repo.clearCalls)
	require.Equal(t, int32(1), sender.calls.Load())
}

func TestOpenAIGatewayServiceSendsMinimalAutoWarmupThroughAccountRoute(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	manifestProxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"models":[{"slug":"gpt-test"}]}`)
	}))
	defer manifestProxy.Close()
	proxyAddress, ok := manifestProxy.Listener.Addr().(*net.TCPAddr)
	require.True(t, ok)
	originalModelsURL := chatgptCodexModelsURL
	chatgptCodexModelsURL = "http://models.invalid/codex/models"
	t.Cleanup(func() { chatgptCodexModelsURL = originalModelsURL })

	proxyID := int64(7)
	account := newAutoWarmupTestAccount(11, now)
	account.ProxyID = &proxyID
	account.Proxy = &Proxy{ID: proxyID, Protocol: "http", Host: proxyAddress.IP.String(), Port: proxyAddress.Port}
	repo := &autoWarmupAccountRepo{accounts: map[int64]*Account{account.ID: account}}
	upstream := &autoWarmupHTTPUpstream{do: func(req *http.Request, body []byte) (*http.Response, error) {
		if !gjson.GetBytes(body, "stream").Bool() || gjson.GetBytes(body, "max_output_tokens").Exists() ||
			gjson.GetBytes(body, "tools").Exists() || gjson.GetBytes(body, "parallel_tool_calls").Exists() ||
			req.Header.Get("Accept") != "text/event-stream" || req.Header.Get("originator") == "" {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}, "X-Request-Id": []string{"req-rejected"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"code":"invalid_request","message":"unsupported warm-up request contract"}}`)),
			}, nil
		}
		headers := http.Header{"Content-Type": []string{"text/event-stream"}}
		headers.Set("X-Request-Id", "req-warm")
		headers.Set("x-codex-primary-used-percent", "12")
		headers.Set("x-codex-primary-reset-after-seconds", "18000")
		headers.Set("x-codex-primary-window-minutes", "300")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     headers,
			Body:       io.NopCloser(strings.NewReader("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp-warm\",\"usage\":{\"input_tokens\":5,\"output_tokens\":1}}}\n\n")),
		}, nil
	}}
	settingService, _ := newAutoWarmupTestSettingService(t, true)
	service := &OpenAIGatewayService{
		accountRepo: repo, httpUpstream: upstream, cfg: &config.Config{}, settingService: settingService,
		codexSnapshotThrottle: newAccountWriteThrottle(time.Nanosecond),
	}

	result, err := service.SendOpenAIAutoWarmup(context.Background(), account.ID)
	require.NoError(t, err)
	require.Equal(t, "gpt-test", result.Model)
	require.Equal(t, "req-warm", result.RequestID)
	require.Equal(t, "resp-warm", result.ResponseID)
	require.Equal(t, 5, result.Usage.InputTokens)
	require.True(t, result.WindowStarted)
	require.NotEmpty(t, result.Observed5hResetAt)
	require.NotNil(t, result.Observed5hUsedPercent)
	require.NotNil(t, upstream.request)
	require.Equal(t, account.Proxy.URL(), upstream.proxyURL)
	require.Equal(t, HTTPUpstreamProfileOpenAI, HTTPUpstreamProfileFromContext(upstream.request.Context()))
	require.Equal(t, "Bearer synthetic-token", upstream.request.Header.Get("Authorization"))
	require.Equal(t, "synthetic-account", upstream.request.Header.Get("chatgpt-account-id"))
	require.Equal(t, codexCLIUserAgent, upstream.request.Header.Get("User-Agent"))
	require.Equal(t, openai.CodexDefaultOriginator, upstream.request.Header.Get("originator"))
	require.Equal(t, codexCLIVersion, upstream.request.Header.Get("version"))
	require.Equal(t, "responses=experimental", upstream.request.Header.Get("OpenAI-Beta"))
	require.Equal(t, chatgptCodexURL, upstream.request.URL.String())
	require.Equal(t, "gpt-test", gjson.GetBytes(upstream.body, "model").String())
	require.Equal(t, openAIAutoWarmupPrompt, gjson.GetBytes(upstream.body, "instructions").String())
	require.Equal(t, openAIAutoWarmupInput, gjson.GetBytes(upstream.body, "input.0.content").String())
	require.True(t, gjson.GetBytes(upstream.body, "stream").Bool())
	require.False(t, gjson.GetBytes(upstream.body, "store").Bool())
	require.False(t, gjson.GetBytes(upstream.body, "max_output_tokens").Exists())
	require.False(t, gjson.GetBytes(upstream.body, "parallel_tool_calls").Exists())
	require.False(t, gjson.GetBytes(upstream.body, "tools").Exists())
	require.Eventually(t, func() bool {
		stored, getErr := repo.GetByID(context.Background(), account.ID)
		if getErr != nil {
			return false
		}
		used, ok := resolveAccountExtraNumber(stored.Extra, "codex_5h_used_percent")
		return ok && used == 12
	}, time.Second, 10*time.Millisecond)
}

func TestOpenAIGatewayServiceCapturesSanitizedAutoWarmupRejection(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	account := newAutoWarmupTestAccount(12, now)
	manifestProxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"models":[{"slug":"gpt-test"}]}`)
	}))
	defer manifestProxy.Close()
	proxyAddress, ok := manifestProxy.Listener.Addr().(*net.TCPAddr)
	require.True(t, ok)
	proxyID := int64(8)
	account.ProxyID = &proxyID
	account.Proxy = &Proxy{ID: proxyID, Protocol: "http", Host: proxyAddress.IP.String(), Port: proxyAddress.Port}
	originalModelsURL := chatgptCodexModelsURL
	chatgptCodexModelsURL = "http://models.invalid/codex/models"
	t.Cleanup(func() { chatgptCodexModelsURL = originalModelsURL })

	upstream := &autoWarmupHTTPUpstream{do: func(_ *http.Request, _ []byte) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"Content-Type": []string{"application/json"}, "X-Request-Id": []string{"req-rejected"}},
			Body: io.NopCloser(strings.NewReader(
				`{"error":{"code":"invalid_request","message":"unsupported request; access_token=secret-value"}}`,
			)),
		}, nil
	}}
	repo := &autoWarmupAccountRepo{accounts: map[int64]*Account{account.ID: account}}
	settingService, _ := newAutoWarmupTestSettingService(t, true)
	service := &OpenAIGatewayService{accountRepo: repo, httpUpstream: upstream, cfg: &config.Config{}, settingService: settingService}

	result, err := service.SendOpenAIAutoWarmup(context.Background(), account.ID)

	require.Error(t, err)
	require.Equal(t, "OPENAI_AUTO_WARMUP_UPSTREAM_REJECTED", infraerrors.Reason(err))
	require.Equal(t, http.StatusBadRequest, result.UpstreamStatus)
	require.Equal(t, "invalid_request", result.UpstreamErrorCode)
	require.Equal(t, "unsupported request; access_token=***", result.UpstreamErrorMessage)
	require.Equal(t, "req-rejected", result.RequestID)
	require.NotContains(t, err.Error(), "secret-value")
	stored, getErr := repo.GetByID(context.Background(), account.ID)
	require.NoError(t, getErr)
	require.Equal(t, StatusActive, stored.Status)
}

func TestOpenAIGatewayServiceAutoWarmupHonorsCallerTimeout(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	account := newAutoWarmupTestAccount(13, now)
	manifestProxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"models":[{"slug":"gpt-test"}]}`)
	}))
	defer manifestProxy.Close()
	proxyAddress, ok := manifestProxy.Listener.Addr().(*net.TCPAddr)
	require.True(t, ok)
	proxyID := int64(9)
	account.ProxyID = &proxyID
	account.Proxy = &Proxy{ID: proxyID, Protocol: "http", Host: proxyAddress.IP.String(), Port: proxyAddress.Port}
	originalModelsURL := chatgptCodexModelsURL
	chatgptCodexModelsURL = "http://models.invalid/codex/models"
	t.Cleanup(func() { chatgptCodexModelsURL = originalModelsURL })

	upstream := &autoWarmupHTTPUpstream{do: func(req *http.Request, _ []byte) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	}}
	settingService, _ := newAutoWarmupTestSettingService(t, true)
	service := &OpenAIGatewayService{
		accountRepo:  &autoWarmupAccountRepo{accounts: map[int64]*Account{account.ID: account}},
		httpUpstream: upstream, cfg: &config.Config{}, settingService: settingService,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := service.SendOpenAIAutoWarmup(ctx, account.ID)

	require.Error(t, err)
	require.Equal(t, "OPENAI_AUTO_WARMUP_UPSTREAM_FAILED", infraerrors.Reason(err))
	require.ErrorIs(t, context.Cause(ctx), context.DeadlineExceeded)
}

func TestResolveOpenAIAutoWarmupModelPrefersEligibleLightweightModelAcrossManifestOrder(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		want     string
	}{
		{name: "preferred model second", manifest: `{"models":[{"slug":"gpt-5.6-sol"},{"slug":"gpt-5.4-mini"}]}`, want: "gpt-5.4-mini"},
		{name: "preferred model first", manifest: `{"models":[{"slug":"gpt-5.4-mini"},{"slug":"gpt-5.6-sol"}]}`, want: "gpt-5.4-mini"},
		{name: "unsupported preferred model skipped", manifest: `{"models":[{"slug":"gpt-5.4-mini","supported_in_api":false},{"slug":"gpt-5.6-terra"}]}`, want: "gpt-5.6-terra"},
		{name: "deterministic fallback", manifest: `{"models":[{"slug":"gpt-test"},{"slug":"gpt-other"}]}`, want: "gpt-other"},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			account := newAutoWarmupTestAccount(int64(100+index), time.Now())
			manifestProxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, test.manifest)
			}))
			defer manifestProxy.Close()
			proxyAddress, ok := manifestProxy.Listener.Addr().(*net.TCPAddr)
			require.True(t, ok)
			proxyID := int64(20 + index)
			account.ProxyID = &proxyID
			account.Proxy = &Proxy{ID: proxyID, Protocol: "http", Host: proxyAddress.IP.String(), Port: proxyAddress.Port}
			originalModelsURL := chatgptCodexModelsURL
			chatgptCodexModelsURL = "http://models.invalid/codex/models"
			t.Cleanup(func() { chatgptCodexModelsURL = originalModelsURL })
			service := &OpenAIGatewayService{accountRepo: &autoWarmupAccountRepo{accounts: map[int64]*Account{account.ID: account}}, cfg: &config.Config{}}

			got, err := service.resolveOpenAIAutoWarmupModel(context.Background(), account, "synthetic-token")

			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestOpenAIGatewayServiceRejectsUnsafeAutoWarmupBeforeNetwork(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	settingService, _ := newAutoWarmupTestSettingService(t, true)
	tests := []struct {
		name   string
		mutate func(*Account)
	}{
		{name: "disabled", mutate: func(a *Account) { a.Status = StatusDisabled }},
		{name: "manual unschedulable", mutate: func(a *Account) { a.Schedulable = false }},
		{name: "rate limited", mutate: func(a *Account) { value := now.Add(time.Hour); a.RateLimitResetAt = &value }},
		{name: "quota exhausted", mutate: func(a *Account) { a.Extra["codex_5h_used_percent"] = 100.0 }},
		{name: "malformed quota", mutate: func(a *Account) { a.Extra["codex_5h_used_percent"] = -1.0 }},
		{name: "missing quota evidence", mutate: func(a *Account) { delete(a.Extra, "codex_5h_used_percent") }},
		{name: "API key", mutate: func(a *Account) { a.Type = AccountTypeAPIKey }},
		{name: "setup token", mutate: func(a *Account) { a.Type = AccountTypeSetupToken }},
		{name: "shadow", mutate: func(a *Account) { parent := int64(99); a.ParentAccountID = &parent }},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			account := newAutoWarmupTestAccount(int64(index+1), now)
			test.mutate(account)
			upstream := &httpUpstreamRecorder{}
			service := &OpenAIGatewayService{
				accountRepo:  &autoWarmupAccountRepo{accounts: map[int64]*Account{account.ID: account}},
				httpUpstream: upstream, cfg: &config.Config{}, settingService: settingService,
			}
			_, err := service.SendOpenAIAutoWarmup(context.Background(), account.ID)
			require.Error(t, err)
			require.Equal(t, "OPENAI_AUTO_WARMUP_ACCOUNT_UNSAFE", infraerrors.Reason(err))
			require.Empty(t, upstream.requests)
		})
	}

	account := newAutoWarmupTestAccount(100, now)
	delete(account.Credentials, "access_token")
	upstream := &httpUpstreamRecorder{}
	service := &OpenAIGatewayService{
		accountRepo:  &autoWarmupAccountRepo{accounts: map[int64]*Account{account.ID: account}},
		httpUpstream: upstream, cfg: &config.Config{}, settingService: settingService,
	}
	_, err := service.SendOpenAIAutoWarmup(context.Background(), account.ID)
	require.Error(t, err)
	require.Equal(t, "OPENAI_AUTO_WARMUP_AUTH_FAILED", infraerrors.Reason(err))
	require.Empty(t, upstream.requests)
}

func TestOpenAIGatewayServiceRechecksFlagsImmediatelyBeforeWarmupRequest(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	for _, flag := range []string{"global", "account"} {
		t.Run(flag, func(t *testing.T) {
			account := newAutoWarmupTestAccount(1, now)
			accountRepo := &autoWarmupAccountRepo{accounts: map[int64]*Account{account.ID: account}}
			settingService, settingRepo := newAutoWarmupTestSettingService(t, true)
			manifestProxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if flag == "global" {
					_ = settingRepo.SetMultiple(context.Background(), map[string]string{SettingKeyOpenAIAutoWarmupEnabled: "false"})
				} else {
					_ = accountRepo.UpdateExtra(context.Background(), account.ID, map[string]any{OpenAIAutoWarmupEnabledExtraKey: false})
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"models":[{"slug":"gpt-test"}]}`)
			}))
			defer manifestProxy.Close()
			proxyAddress, ok := manifestProxy.Listener.Addr().(*net.TCPAddr)
			require.True(t, ok)
			proxyID := int64(7)
			account.ProxyID = &proxyID
			account.Proxy = &Proxy{ID: proxyID, Protocol: "http", Host: proxyAddress.IP.String(), Port: proxyAddress.Port}
			originalModelsURL := chatgptCodexModelsURL
			chatgptCodexModelsURL = "http://models.invalid/codex/models"
			defer func() { chatgptCodexModelsURL = originalModelsURL }()

			upstream := &httpUpstreamRecorder{}
			service := &OpenAIGatewayService{accountRepo: accountRepo, httpUpstream: upstream, cfg: &config.Config{}, settingService: settingService}
			_, err := service.SendOpenAIAutoWarmup(context.Background(), account.ID)
			require.Error(t, err)
			if flag == "global" {
				require.Equal(t, "OPENAI_AUTO_WARMUP_DISABLED", infraerrors.Reason(err))
			} else {
				require.Equal(t, "OPENAI_AUTO_WARMUP_ACCOUNT_UNSAFE", infraerrors.Reason(err))
			}
			require.Empty(t, upstream.requests)
		})
	}
}

func TestOpenAIAutoWarmupSettingsDefaultOffAndPersist(t *testing.T) {
	original := xai.RuntimeModelMappingOptions()
	t.Cleanup(func() { xai.SetRuntimeModelMappingOptions(original) })
	repo := &autoWarmupSettingRepo{values: map[string]string{}}
	service := NewSettingService(repo, &config.Config{})
	settings, err := service.GetAllSettings(context.Background())
	require.NoError(t, err)
	require.False(t, settings.OpenAIAutoWarmupEnabled)

	settings.OpenAIAutoWarmupEnabled = true
	require.NoError(t, service.UpdateSettings(context.Background(), settings))
	repo.mu.Lock()
	persisted := repo.values[SettingKeyOpenAIAutoWarmupEnabled]
	repo.mu.Unlock()
	require.Equal(t, "true", persisted)
}

func TestOpenAIAutoWarmupAccountSettingRejectsUnsupportedAccounts(t *testing.T) {
	for _, test := range []struct {
		name        string
		platform    string
		accountType string
		shadow      bool
	}{
		{name: "OpenAI API key", platform: PlatformOpenAI, accountType: AccountTypeAPIKey},
		{name: "OpenAI setup token", platform: PlatformOpenAI, accountType: AccountTypeSetupToken},
		{name: "Gemini OAuth", platform: PlatformGemini, accountType: AccountTypeOAuth},
		{name: "OpenAI OAuth shadow", platform: PlatformOpenAI, accountType: AccountTypeOAuth, shadow: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := normalizeOpenAIAutoResetCreditExtra(test.platform, test.accountType, test.shadow, map[string]any{
				OpenAIAutoWarmupEnabledExtraKey: true,
			})
			require.Error(t, err)
		})
	}
}
