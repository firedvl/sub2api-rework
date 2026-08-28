package service

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
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
	release <-chan struct{}
	started chan<- struct{}
	calls   atomic.Int32
	current atomic.Int32
	max     atomic.Int32
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
	return &OpenAIAutoWarmupResult{
		Model: "gpt-test", RequestID: "req-test", ResponseID: "resp-test",
		Usage: OpenAIUsage{InputTokens: 5, OutputTokens: 1}, Latency: 10 * time.Millisecond,
	}, s.err
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
}

func TestOpenAIAutoWarmupConcurrencyIsBounded(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	release := make(chan struct{})
	started := make(chan struct{}, 8)
	sender := &autoWarmupSenderStub{release: release, started: started}
	attempts := &autoWarmupAttemptMemoryRepo{}
	accounts := make(map[int64]*Account, 8)
	for id := int64(1); id <= 8; id++ {
		accounts[id] = newAutoWarmupTestAccount(id, now)
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
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}, "X-Request-Id": []string{"req-warm"}},
		Body: io.NopCloser(gjsonReader(`{"id":"resp-warm","usage":{"input_tokens":5,"output_tokens":1}}`)),
	}}
	settingService, _ := newAutoWarmupTestSettingService(t, true)
	service := &OpenAIGatewayService{accountRepo: repo, httpUpstream: upstream, cfg: &config.Config{}, settingService: settingService}

	result, err := service.SendOpenAIAutoWarmup(context.Background(), account.ID)
	require.NoError(t, err)
	require.Equal(t, "gpt-test", result.Model)
	require.Equal(t, "req-warm", result.RequestID)
	require.Equal(t, "resp-warm", result.ResponseID)
	require.Equal(t, 5, result.Usage.InputTokens)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, account.Proxy.URL(), upstream.lastProxyURL)
	require.Equal(t, HTTPUpstreamProfileOpenAI, HTTPUpstreamProfileFromContext(upstream.lastReq.Context()))
	require.Equal(t, "Bearer synthetic-token", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, chatgptCodexURL, upstream.lastReq.URL.String())
	require.Equal(t, "gpt-test", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, openAIAutoWarmupPrompt, gjson.GetBytes(upstream.lastBody, "input").String())
	require.Equal(t, int64(openAIAutoWarmupMaxOutput), gjson.GetBytes(upstream.lastBody, "max_output_tokens").Int())
	require.False(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.False(t, gjson.GetBytes(upstream.lastBody, "store").Bool())
	require.False(t, gjson.GetBytes(upstream.lastBody, "parallel_tool_calls").Bool())
	require.Empty(t, gjson.GetBytes(upstream.lastBody, "tools").Array())
	require.False(t, gjson.GetBytes(upstream.lastBody, "web_search").Exists())
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

type stringReadCloser struct{ value string }

func (r *stringReadCloser) Read(buffer []byte) (int, error) {
	if r.value == "" {
		return 0, io.EOF
	}
	n := copy(buffer, r.value)
	r.value = r.value[n:]
	return n, nil
}

func (r *stringReadCloser) Close() error { return nil }

func gjsonReader(value string) *stringReadCloser { return &stringReadCloser{value: value} }
