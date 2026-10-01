//go:build unit

package service

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func ollama429Account(id int64, platform string) *Account {
	return &Account{
		ID:          id,
		Platform:    platform,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"base_url": "https://www.ollama.com",
			"api_key":  "ollama429-key",
		},
	}
}

type ollama429SchedulerStub struct {
	mu        sync.Mutex
	accept    bool
	scheduled []int64
	callbacks map[int64]OllamaCloudUsageRateLimitProbeCallback
}

func newOllama429SchedulerStub(accept bool) *ollama429SchedulerStub {
	return &ollama429SchedulerStub{accept: accept, callbacks: map[int64]OllamaCloudUsageRateLimitProbeCallback{}}
}

func (p *ollama429SchedulerStub) ScheduleOllamaCloudUsageRateLimitProbe(accountID int64, cb OllamaCloudUsageRateLimitProbeCallback) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.accept {
		return false
	}
	p.scheduled = append(p.scheduled, accountID)
	p.callbacks[accountID] = cb // latest wins, mirroring the coordinator coalescing
	return true
}

func (p *ollama429SchedulerStub) fire(accountID int64, resetAt time.Time) {
	p.mu.Lock()
	cb := p.callbacks[accountID]
	p.mu.Unlock()
	if cb != nil {
		cb(accountID, resetAt)
	}
}

func (p *ollama429SchedulerStub) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.scheduled)
}

type ollama429BlockRec struct {
	accountID int64
	until     time.Time
	reason    string
}

type ollama429BlockerStub struct {
	mu          sync.Mutex
	blocks      []ollama429BlockRec
	generations map[int64]uint64
}

func (b *ollama429BlockerStub) BlockAccountScheduling(account *Account, until time.Time, reason string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if account != nil {
		b.blocks = append(b.blocks, ollama429BlockRec{accountID: account.ID, until: until, reason: reason})
	}
}

func (blocker *ollama429BlockerStub) ClearAccountSchedulingBlock(id int64) {
	blocker.mu.Lock()
	defer blocker.mu.Unlock()
	if blocker.generations == nil {
		blocker.generations = make(map[int64]uint64)
	}
	blocker.generations[id]++
}

func (blocker *ollama429BlockerStub) AccountSchedulingClearGeneration(id int64) uint64 {
	blocker.mu.Lock()
	defer blocker.mu.Unlock()
	return blocker.generations[id]
}

func (blocker *ollama429BlockerStub) BlockAccountSchedulingIfGeneration(account *Account, until time.Time, reason string, expected uint64) (uint64, bool) {
	blocker.mu.Lock()
	defer blocker.mu.Unlock()
	if blocker.generations[account.ID] != expected {
		return blocker.generations[account.ID], false
	}
	if blocker.generations == nil {
		blocker.generations = make(map[int64]uint64)
	}
	blocker.blocks = append(blocker.blocks, ollama429BlockRec{accountID: account.ID, until: until, reason: reason})
	return blocker.generations[account.ID], true
}

func (b *ollama429BlockerStub) last() (ollama429BlockRec, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.blocks) == 0 {
		return ollama429BlockRec{}, false
	}
	return b.blocks[len(b.blocks)-1], true
}

type ollama429Repo struct {
	mockAccountRepoForGemini
	mu             sync.Mutex
	accounts       map[int64]*Account
	casUpdated     int
	casSkipped     int
	ifLaterWrites  []time.Time
	uncondWrites   []time.Time
	versionCounter int64
}

func newOllama429Repo(acct *Account) *ollama429Repo {
	r := &ollama429Repo{accounts: map[int64]*Account{}}
	r.accounts[acct.ID] = acct
	return r
}

func (r *ollama429Repo) bump(acct *Account) {
	r.versionCounter++
	acct.UpdatedAt = time.Unix(0, r.versionCounter).UTC()
}

func (r *ollama429Repo) GetByID(_ context.Context, id int64) (*Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	account := r.accounts[id]
	if account == nil {
		return nil, nil
	}
	copy := cloneOllamaUsageTestAccount(*account)
	return &copy, nil
}

func (repository *ollama429Repo) RecordOllamaCloudUsage429(_ context.Context, observed *Account, resetAt *time.Time) (*AccountRateLimitGeneration, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	account := repository.accounts[observed.ID]
	if account == nil || !account.UpdatedAt.Equal(observed.UpdatedAt) {
		return nil, nil
	}
	if resetAt != nil && (account.RateLimitResetAt == nil || resetAt.After(*account.RateLimitResetAt)) {
		account.RateLimitResetAt = cloneTimePtr(resetAt)
		repository.ifLaterWrites = append(repository.ifLaterWrites, *resetAt)
	}
	limitedAt := time.Now()
	if account.RateLimitedAt != nil && !limitedAt.After(*account.RateLimitedAt) {
		limitedAt = account.RateLimitedAt.Add(time.Microsecond)
	}
	account.RateLimitedAt = &limitedAt
	repository.bump(account)
	return &AccountRateLimitGeneration{LimitedAt: limitedAt, ResetAt: cloneTimePtr(account.RateLimitResetAt)}, nil
}

func (r *ollama429Repo) currentReset(id int64) *time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	a := r.accounts[id]
	if a == nil || a.RateLimitResetAt == nil {
		return nil
	}
	v := *a.RateLimitResetAt
	return &v
}

func (r *ollama429Repo) mutate(id int64, fn func(*Account)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a, ok := r.accounts[id]; ok {
		fn(a)
		r.bump(a)
	}
}

func (r *ollama429Repo) bumpVersion(id int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a, ok := r.accounts[id]; ok {
		r.bump(a)
	}
}

func (r *ollama429Repo) SetRateLimitedIfLater(_ context.Context, id int64, resetAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	a := r.accounts[id]
	if a == nil {
		return nil
	}
	cur := a.RateLimitResetAt
	if cur == nil || resetAt.After(*cur) {
		a.RateLimitResetAt = ollama429TimePtr(resetAt)
		now := time.Now()
		a.RateLimitedAt = &now
		r.bump(a)
		r.ifLaterWrites = append(r.ifLaterWrites, resetAt)
	}
	return nil
}

func (r *ollama429Repo) SetRateLimited(_ context.Context, id int64, resetAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	a := r.accounts[id]
	if a == nil {
		return nil
	}
	a.RateLimitResetAt = ollama429TimePtr(resetAt)
	now := time.Now()
	a.RateLimitedAt = &now
	r.bump(a)
	r.uncondWrites = append(r.uncondWrites, resetAt)
	return nil
}

func (r *ollama429Repo) SetRateLimitedIfUnchanged(
	_ context.Context, id int64,
	expectedUpdatedAt time.Time,
	expectedLimitedAt, expectedResetAt *time.Time,
	newResetAt time.Time,
) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a := r.accounts[id]
	if a == nil {
		r.casSkipped++
		return false, nil
	}
	limitedMatch := timePtrEqual(expectedLimitedAt, a.RateLimitedAt)
	resetMatch := timePtrEqual(expectedResetAt, a.RateLimitResetAt)
	if !a.UpdatedAt.Equal(expectedUpdatedAt) || !limitedMatch || !resetMatch {
		r.casSkipped++
		return false, nil
	}
	a.RateLimitedAt = ollama429TimePtr(time.Now())
	a.RateLimitResetAt = ollama429TimePtr(newResetAt)
	r.bump(a)
	r.casUpdated++
	return true, nil
}

func ollama429TimePtr(t time.Time) *time.Time { return &t }

func timePtrEqual(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

func ollama429Fixture(t *testing.T, repo *ollama429Repo, scheduler *ollama429SchedulerStub) (*RateLimitService, *ollama429BlockerStub) {
	t.Helper()
	svc := NewRateLimitService(repo, nil, nil, nil, nil)
	blocker := &ollama429BlockerStub{}
	svc.SetAccountRuntimeBlocker(blocker)
	svc.SetOllamaCloudUsageProbeScheduler(scheduler)
	return svc, blocker
}

func TestHandle429_OllamaEarlyBranchAcrossPlatforms(t *testing.T) {
	platforms := []string{PlatformOpenAI, PlatformAnthropic, PlatformKimi, PlatformZhipu, PlatformDeepseek}
	for _, platform := range platforms {
		t.Run(platform, func(t *testing.T) {
			acct := ollama429Account(101, platform)
			repo := newOllama429Repo(acct)
			scheduler := newOllama429SchedulerStub(true)
			svc, _ := ollama429Fixture(t, repo, scheduler)

			svc.handle429(context.Background(), acct, http.Header{}, nil)

			require.Equal(t, 1, scheduler.count(), "real-Ollama %s account must schedule a probe", platform)
			require.Greater(t, len(repo.ifLaterWrites), 0, "immediate never-shrink cooldown must be applied")
		})
	}
}

func TestHandle429_NonOllamaUnchanged(t *testing.T) {
	acct := &Account{ID: 202, Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"base_url": "https://api.anthropic.com", "api_key": "k"}}
	repo := newOllama429Repo(acct)
	scheduler := newOllama429SchedulerStub(true)
	svc, _ := ollama429Fixture(t, repo, scheduler)

	svc.handle429(context.Background(), acct, http.Header{}, nil)

	require.Zero(t, scheduler.count(), "non-Ollama account must not schedule an Ollama probe")
	require.Zero(t, repo.casUpdated)
}

func TestHandle429_OllamaUsesValidRetryAfter(t *testing.T) {
	acct := ollama429Account(301, PlatformOpenAI)
	repo := newOllama429Repo(acct)
	scheduler := newOllama429SchedulerStub(true)
	svc, _ := ollama429Fixture(t, repo, scheduler)

	before := time.Now()
	svc.handle429(context.Background(), acct, http.Header{"Retry-After": []string{"120"}}, nil)
	after := time.Now()

	require.Equal(t, 1, scheduler.count())
	reset := repo.currentReset(acct.ID)
	require.NotNil(t, reset)
	require.False(t, reset.Before(before.Add(120*time.Second)), "reset %v < now+120s", reset)
	require.False(t, reset.After(after.Add(120*time.Second)), "reset %v > now+120s", reset)
}

func TestHandle429_OllamaFallsBackToSecondsCooldown(t *testing.T) {
	acct := ollama429Account(302, PlatformAnthropic)
	repo := newOllama429Repo(acct)
	scheduler := newOllama429SchedulerStub(true)
	svc, _ := ollama429Fixture(t, repo, scheduler)

	before := time.Now()
	svc.handle429(context.Background(), acct, http.Header{}, nil) // no Retry-After
	after := time.Now()

	require.Equal(t, 1, scheduler.count())
	reset := repo.currentReset(acct.ID)
	require.NotNil(t, reset)
	def := defaultRateLimit429CooldownSeconds
	require.False(t, reset.Before(before.Add(time.Duration(def)*time.Second)))
	require.False(t, reset.After(after.Add(time.Duration(def)*time.Second)))
}

func TestHandle429_OllamaScheduleRejectedStillCooled(t *testing.T) {
	acct := ollama429Account(303, PlatformOpenAI)
	repo := newOllama429Repo(acct)
	scheduler := newOllama429SchedulerStub(false) // queue full / service stopped
	svc, _ := ollama429Fixture(t, repo, scheduler)

	svc.handle429(context.Background(), acct, http.Header{}, nil)

	require.Zero(t, scheduler.count())
	require.Greater(t, len(repo.ifLaterWrites), 0, "immediate cooldown must still be applied when scheduling is rejected")
}

func TestHandle429_OllamaDoesNotShrinkConfirmedLongCooldown(t *testing.T) {
	acct := ollama429Account(304, PlatformOpenAI)
	repo := newOllama429Repo(acct)
	long := time.Now().Add(time.Hour)
	repo.mutate(acct.ID, func(a *Account) { a.RateLimitResetAt = &long })
	scheduler := newOllama429SchedulerStub(true)
	svc, _ := ollama429Fixture(t, repo, scheduler)

	svc.handle429(context.Background(), acct, http.Header{}, nil)

	reset := repo.currentReset(acct.ID)
	require.NotNil(t, reset)
	require.GreaterOrEqual(t, reset.Unix(), long.Unix(), "already-confirmed long cooldown must not be shrunk to fallback seconds")
	require.Equal(t, 1, scheduler.count())
}

func TestOllamaProbeCallback_ValidUpdatesDBAndMemory(t *testing.T) {
	acct := ollama429Account(401, PlatformOpenAI)
	repo := newOllama429Repo(acct)
	scheduler := newOllama429SchedulerStub(true)
	svc, blocker := ollama429Fixture(t, repo, scheduler)

	svc.handle429(context.Background(), acct, http.Header{}, nil) // schedules; immediate 5s
	require.Equal(t, 1, scheduler.count())

	probeReset := time.Now().Add(2 * time.Hour)
	scheduler.fire(acct.ID, probeReset)

	require.Equal(t, 1, repo.casUpdated, "valid callback must update DB once")
	require.Zero(t, repo.casSkipped)
	reset := repo.currentReset(acct.ID)
	require.NotNil(t, reset)
	require.True(t, reset.Equal(probeReset), "DB reset = %v, want %v", reset, probeReset)
	rec, ok := blocker.last()
	require.True(t, ok)
	require.Equal(t, "ollama_cloud_usage_429_probe", rec.reason)
	require.True(t, rec.until.Equal(probeReset))
}

func TestOllamaProbeCallback_DuplicateNoDoubleUpdate(t *testing.T) {
	acct := ollama429Account(402, PlatformOpenAI)
	repo := newOllama429Repo(acct)
	scheduler := newOllama429SchedulerStub(true)
	svc, blocker := ollama429Fixture(t, repo, scheduler)

	svc.handle429(context.Background(), acct, http.Header{}, nil)
	probeReset := time.Now().Add(2 * time.Hour)
	scheduler.fire(acct.ID, probeReset)
	require.Equal(t, 1, repo.casUpdated)

	scheduler.fire(acct.ID, probeReset.Add(time.Hour))
	require.Equal(t, 1, repo.casUpdated)
	rec, _ := blocker.last()
	require.Equal(t, "ollama_cloud_usage_429_probe", rec.reason)
	require.True(t, rec.until.Equal(probeReset))
}

func TestOllamaProbeCallback_StaleLongDoesNotOverrideNewShort(t *testing.T) {
	acct := ollama429Account(403, PlatformOpenAI)
	repo := newOllama429Repo(acct)
	scheduler := newOllama429SchedulerStub(true)
	svc, blocker := ollama429Fixture(t, repo, scheduler)

	svc.handle429(context.Background(), acct, http.Header{}, nil) // schedules gen @5s
	require.Equal(t, 1, scheduler.count())

	newShort := time.Now().Add(5 * time.Second)
	repo.mutate(acct.ID, func(a *Account) { a.RateLimitResetAt = ollama429TimePtr(newShort) })

	oldLong := time.Now().Add(7 * 24 * time.Hour)
	scheduler.fire(acct.ID, oldLong)

	require.Zero(t, repo.casUpdated, "stale long callback must not pass the CAS")
	require.Greater(t, repo.casSkipped, 0)
	reset := repo.currentReset(acct.ID)
	require.NotNil(t, reset)
	require.True(t, reset.Equal(newShort), "current reset must remain the new short %v, got %v", newShort, reset)
	rec, ok := blocker.last()
	if ok && rec.reason == "ollama_cloud_usage_429_probe" {
		t.Fatalf("stale long callback must not announce scheduling; got block to %v", rec.until)
	}
}

func TestOllamaProbeCallback_AdminClearSkips(t *testing.T) {
	acct := ollama429Account(404, PlatformOpenAI)
	repo := newOllama429Repo(acct)
	scheduler := newOllama429SchedulerStub(true)
	svc, blocker := ollama429Fixture(t, repo, scheduler)

	svc.handle429(context.Background(), acct, http.Header{}, nil)
	repo.mutate(acct.ID, func(a *Account) {
		a.RateLimitedAt = nil
		a.RateLimitResetAt = nil
	})

	scheduler.fire(acct.ID, time.Now().Add(2*time.Hour))
	require.Zero(t, repo.casUpdated, "cleared account must not be re-limited by a stale callback")
	require.Nil(t, repo.currentReset(acct.ID))
	rec, ok := blocker.last()
	if ok && rec.reason == "ollama_cloud_usage_429_probe" {
		t.Fatalf("cleared account must not be re-blocked by a stale callback")
	}
}

func TestOllamaProbeCallback_KeySwapSkips(t *testing.T) {
	acct := ollama429Account(405, PlatformOpenAI)
	repo := newOllama429Repo(acct)
	scheduler := newOllama429SchedulerStub(true)
	svc, _ := ollama429Fixture(t, repo, scheduler)

	svc.handle429(context.Background(), acct, http.Header{}, nil) // captures fingerprint of k1
	repo.mutate(acct.ID, func(a *Account) { a.Credentials["api_key"] = "ollama429-key-2" })

	scheduler.fire(acct.ID, time.Now().Add(2*time.Hour))
	require.Zero(t, repo.casUpdated, "key-swapped account must not be limited by the old key's probe")
	reset := repo.currentReset(acct.ID)
	require.NotNil(t, reset)
	require.False(t, reset.After(time.Now().Add(time.Hour)), "key-swapped account must keep only its immediate cooldown, got %v", reset)
}

func TestOllamaProbeCallback_ResetAlreadyPastSkips(t *testing.T) {
	acct := ollama429Account(406, PlatformOpenAI)
	repo := newOllama429Repo(acct)
	scheduler := newOllama429SchedulerStub(true)
	svc, _ := ollama429Fixture(t, repo, scheduler)

	svc.handle429(context.Background(), acct, http.Header{}, nil)
	repo.mutate(acct.ID, func(a *Account) {
	})
	scheduler.fire(acct.ID, time.Now().Add(-time.Minute))

	require.Zero(t, repo.casUpdated)
	require.Zero(t, repo.casSkipped, "an already-passed reset is dropped before any CAS is attempted")
}

func TestOllamaProbeCallback_SnapshotPersistBumpStillApplies(t *testing.T) {
	acct := ollama429Account(501, PlatformOpenAI)
	repo := newOllama429Repo(acct)
	scheduler := newOllama429SchedulerStub(true)
	svc, blocker := ollama429Fixture(t, repo, scheduler)

	svc.handle429(context.Background(), acct, http.Header{}, nil)
	require.Equal(t, 1, scheduler.count())

	repo.bumpVersion(acct.ID) // simulate updateSnapshot persisting the snapshot

	probeReset := time.Now().Add(2 * time.Hour)
	scheduler.fire(acct.ID, probeReset)

	require.Equal(t, 1, repo.casUpdated, "snapshot persist bumping UpdatedAt must not fail the exhaustion CAS")
	require.Zero(t, repo.casSkipped)
	reset := repo.currentReset(acct.ID)
	require.NotNil(t, reset)
	require.True(t, reset.Equal(probeReset), "DB reset = %v, want %v", reset, probeReset)
	rec, ok := blocker.last()
	require.True(t, ok)
	require.Equal(t, "ollama_cloud_usage_429_probe", rec.reason)
}

func TestOllamaProbeCallback_NeverShortensCooldownFloor(t *testing.T) {
	acct := ollama429Account(502, PlatformOpenAI)
	repo := newOllama429Repo(acct)
	scheduler := newOllama429SchedulerStub(true)
	svc, _ := ollama429Fixture(t, repo, scheduler)

	before := time.Now()
	svc.handle429(context.Background(), acct, http.Header{"Retry-After": []string{"120"}}, nil)
	after := time.Now()
	floor := repo.currentReset(acct.ID)
	require.NotNil(t, floor)
	require.False(t, floor.Before(before.Add(120*time.Second)))
	require.False(t, floor.After(after.Add(120*time.Second)))

	scheduler.fire(acct.ID, time.Now().Add(30*time.Second))

	require.Zero(t, repo.casUpdated, "a probe reset sooner than the explicit floor must be dropped")
	still := repo.currentReset(acct.ID)
	require.True(t, still.Equal(*floor), "the explicit Retry-After floor must be preserved")
}

func TestOllamaProbeCallback_DisabledSkips(t *testing.T) {
	acct := ollama429Account(503, PlatformOpenAI)
	repo := newOllama429Repo(acct)
	scheduler := newOllama429SchedulerStub(true)
	svc, _ := ollama429Fixture(t, repo, scheduler)

	svc.handle429(context.Background(), acct, http.Header{}, nil)
	repo.mutate(acct.ID, func(a *Account) { a.Status = StatusDisabled })

	scheduler.fire(acct.ID, time.Now().Add(2*time.Hour))
	require.Zero(t, repo.casUpdated, "a disabled account must not be re-limited by a stale callback")
}

func TestHandle429_OllamaSchedulerAbsentStillNotifiesRuntime(t *testing.T) {
	acct := ollama429Account(504, PlatformOpenAI)
	repo := newOllama429Repo(acct)
	svc := NewRateLimitService(repo, nil, nil, nil, nil)
	blocker := &ollama429BlockerStub{}
	svc.SetAccountRuntimeBlocker(blocker)

	before := time.Now()
	svc.handle429(context.Background(), acct, http.Header{}, nil)
	after := time.Now()

	require.Greater(t, len(repo.ifLaterWrites), 0)
	rec, ok := blocker.last()
	require.True(t, ok)
	require.Equal(t, "ollama_429", rec.reason)
	require.False(t, rec.until.Before(before.Add(defaultRateLimit429CooldownSeconds*time.Second)))
	require.False(t, rec.until.After(after.Add(defaultRateLimit429CooldownSeconds*time.Second)))
}

func TestOllamaProbeCallback_FallbackOffStillRejectsAdminClear(t *testing.T) {
	account := ollama429Account(891, PlatformOpenAI)
	repository := newOllama429Repo(account)
	scheduler := newOllama429SchedulerStub(true)
	service, _ := ollama429Fixture(t, repository, scheduler)
	settings := newMockSettingRepo()
	settings.data[SettingKeyRateLimit429CooldownSettings] = `{"enabled":false,"cooldown_seconds":5}`
	service.SetSettingService(NewSettingService(settings, nil))
	service.handle429(context.Background(), account, http.Header{}, nil)
	require.Nil(t, repository.currentReset(account.ID), "disabled fallback must not invent a cooldown")
	repository.mutate(account.ID, func(current *Account) { current.RateLimitedAt, current.RateLimitResetAt = nil, nil })
	scheduler.fire(account.ID, time.Now().Add(time.Hour))
	require.Zero(t, repository.casUpdated, "an old callback must not re-arm an administratively cleared generation")
}

func TestOllamaProbeCallback_Newer429WithSameLongFloorRejectsOldCallback(t *testing.T) {
	account := ollama429Account(892, PlatformOpenAI)
	account.RateLimitedAt = ollama429TimePtr(time.Now())
	account.RateLimitResetAt = ollama429TimePtr(time.Now().Add(time.Hour))
	repository := newOllama429Repo(account)
	scheduler := newOllama429SchedulerStub(true)
	service, _ := ollama429Fixture(t, repository, scheduler)
	service.handle429(context.Background(), account, http.Header{}, nil)
	oldCallback := scheduler.callbacks[account.ID]
	service.handle429(context.Background(), account, http.Header{}, nil)
	oldCallback(account.ID, time.Now().Add(2*time.Hour))
	require.Zero(t, repository.casUpdated, "every new 429 must supersede its older callback even when the floor stays unchanged")
}

func TestOllamaProbeCallback_ChangedProxyOrSessionRejectsOldResult(t *testing.T) {
	for _, change := range []string{"proxy", "session"} {
		t.Run(change, func(t *testing.T) {
			account := ollama429Account(893, PlatformOpenAI)
			account.Extra = map[string]any{OllamaCloudUsageSessionExtraKey: "cipher:wos-session=original"}
			repository := newOllama429Repo(account)
			scheduler := newOllama429SchedulerStub(true)
			service, _ := ollama429Fixture(t, repository, scheduler)
			service.handle429(context.Background(), account, http.Header{}, nil)
			repository.mutate(account.ID, func(current *Account) {
				if change == "proxy" {
					proxyID := int64(77)
					current.ProxyID = &proxyID
				} else {
					current.Extra[OllamaCloudUsageSessionExtraKey] = "cipher:wos-session=replaced"
				}
			})
			scheduler.fire(account.ID, time.Now().Add(time.Hour))
			require.Zero(t, repository.casUpdated)
		})
	}
}

type ollama429LinkRepo struct {
	*ollamaUsageTestRepo
	versionCounter int64
	casUpdated     int
	linkIfLater    []time.Time
}

func (repository *ollama429LinkRepo) RecordOllamaCloudUsage429(_ context.Context, observed *Account, resetAt *time.Time) (*AccountRateLimitGeneration, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	account := repository.accounts[observed.ID]
	if account == nil || !account.UpdatedAt.Equal(observed.UpdatedAt) {
		return nil, nil
	}
	if resetAt != nil && (account.RateLimitResetAt == nil || resetAt.After(*account.RateLimitResetAt)) {
		account.RateLimitResetAt = cloneTimePtr(resetAt)
		repository.linkIfLater = append(repository.linkIfLater, *resetAt)
	}
	limitedAt := time.Now()
	if account.RateLimitedAt != nil && !limitedAt.After(*account.RateLimitedAt) {
		limitedAt = account.RateLimitedAt.Add(time.Microsecond)
	}
	account.RateLimitedAt = &limitedAt
	repository.bump(account)
	return &AccountRateLimitGeneration{LimitedAt: limitedAt, ResetAt: cloneTimePtr(account.RateLimitResetAt)}, nil
}

func (r *ollama429LinkRepo) bump(a *Account) {
	r.versionCounter++
	a.UpdatedAt = time.Unix(0, r.versionCounter).UTC()
}

func (r *ollama429LinkRepo) UpdateOllamaCloudUsageSnapshot(ctx context.Context, expected *Account, snapshot *OllamaCloudUsageSnapshot) error {
	if err := r.ollamaUsageTestRepo.UpdateOllamaCloudUsageSnapshot(ctx, expected, snapshot); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if a, ok := r.accounts[expected.ID]; ok {
		r.bump(a)
	}
	return nil
}

func (r *ollama429LinkRepo) SetRateLimitedIfLater(_ context.Context, id int64, resetAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	a := r.accounts[id]
	if a == nil {
		return nil
	}
	cur := a.RateLimitResetAt
	if cur == nil || resetAt.After(*cur) {
		a.RateLimitResetAt = ollama429TimePtr(resetAt)
		now := time.Now()
		a.RateLimitedAt = &now
		r.bump(a)
		r.linkIfLater = append(r.linkIfLater, resetAt)
	}
	return nil
}

func (r *ollama429LinkRepo) SetRateLimitedIfUnchanged(
	_ context.Context, id int64,
	expectedUpdatedAt time.Time,
	expectedLimitedAt, expectedResetAt *time.Time,
	newResetAt time.Time,
) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a := r.accounts[id]
	if a == nil {
		return false, nil
	}
	if !a.UpdatedAt.Equal(expectedUpdatedAt) ||
		!timePtrEqual(expectedLimitedAt, a.RateLimitedAt) ||
		!timePtrEqual(expectedResetAt, a.RateLimitResetAt) {
		return false, nil
	}
	a.RateLimitedAt = ollama429TimePtr(time.Now())
	a.RateLimitResetAt = ollama429TimePtr(newResetAt)
	r.bump(a)
	r.casUpdated++
	return true, nil
}

func (r *ollama429LinkRepo) currentReset(id int64) *time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	a := r.accounts[id]
	if a == nil || a.RateLimitResetAt == nil {
		return nil
	}
	v := *a.RateLimitResetAt
	return &v
}

func (r *ollama429LinkRepo) casUpdatedCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.casUpdated
}

func TestOllama429RealProbeLinkage_SnapshotPersistThenWriteBack(t *testing.T) {
	account := ollamaUsageAccount(701)
	account.Extra[OllamaCloudUsageSessionExtraKey] = "cipher:wos-session=secret"
	account.Extra[OllamaCloudUsageAutoRefreshExtraKey] = false

	reset := time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)
	body := []byte(`
		<section>
			<div><span>5 hour usage</span><span>100% used</span></div>
			<time datetime="` + reset + `"></time>
		</section>`)

	repo := &ollama429LinkRepo{ollamaUsageTestRepo: &ollamaUsageTestRepo{
		upstreamBillingProbeAccountRepo: &upstreamBillingProbeAccountRepo{
			accounts: map[int64]*Account{account.ID: account},
		},
	}}
	upstream := &ollamaUsageHTTPStub{body: body}

	usageSvc := NewOllamaCloudUsageService(repo, upstream, NewSettingService(&upstreamBillingProbeSettingRepo{}, nil), ollamaUsageTestEncryptor{}, true)

	noop, noopErr := usageSvc.refreshAccount(context.Background(), account.ID, defaultOllamaCloudUsageSettings(), true)
	require.NoError(t, noopErr)
	require.Nil(t, noop, "auto_refresh-disabled refresh is a nil,nil no-op")

	usageSvc.Start()
	t.Cleanup(usageSvc.Stop)

	blocker := &ollama429BlockerStub{}
	rlSvc := NewRateLimitService(repo, nil, nil, nil, nil)
	rlSvc.SetAccountRuntimeBlocker(blocker)
	rlSvc.SetOllamaCloudUsageProbeScheduler(usageSvc)

	rlSvc.handle429(context.Background(), account, http.Header{}, nil)
	require.Greater(t, len(repo.linkIfLater), 0, "immediate cooldown must be applied")

	require.Eventually(t, func() bool {
		if repo.casUpdatedCount() != 1 {
			return false
		}
		rec, ok := blocker.last()
		return ok && rec.reason == "ollama_cloud_usage_429_probe"
	}, 10*time.Second, 5*time.Millisecond, "exhaustion write-back through the real probe must update once and notify")

	require.Greater(t, upstream.calls.Load(), int64(0), "probe must query usage even with auto_refresh disabled")

	rec, ok := blocker.last()
	require.True(t, ok)
	require.Equal(t, "ollama_cloud_usage_429_probe", rec.reason)
	require.True(t, rec.until.After(time.Now()), "probe reset must be in the future")
	resetAfter := repo.currentReset(account.ID)
	require.NotNil(t, resetAfter)
	require.True(t, resetAfter.After(time.Now()), "DB reset must be extended by the real probe write-back")
}
