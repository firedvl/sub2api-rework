//go:build unit

package service

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestEstimateInflightReservationCost(t *testing.T) {
	billing := NewBillingService(&config.Config{}, nil)
	pricing, err := billing.GetModelPricing("claude-sonnet-4-5")
	require.NoError(t, err)

	cfg := config.InflightReservationConfig{DefaultMaxTokens: 8192, MaxOutputTokens: 64000, MaxInputTokens: 1000}

	// explicit max_tokens; body 400 bytes → 100 input tokens
	cost, ok := EstimateInflightReservationCost(billing, cfg, "claude-sonnet-4-5", 400, 1000, 1)
	require.True(t, ok)
	require.InDelta(t, 100*pricing.InputPricePerToken+1000*pricing.OutputPricePerToken, cost, 1e-12)

	// missing max_tokens → default; rate multiplier applied
	cost, ok = EstimateInflightReservationCost(billing, cfg, "claude-sonnet-4-5", 0, 0, 2)
	require.True(t, ok)
	require.InDelta(t, 2*8192*pricing.OutputPricePerToken, cost, 1e-12)

	// caps: input and output clamped
	cost, ok = EstimateInflightReservationCost(billing, cfg, "claude-sonnet-4-5", 1_000_000, 1_000_000, 1)
	require.True(t, ok)
	require.InDelta(t, 1000*pricing.InputPricePerToken+64000*pricing.OutputPricePerToken, cost, 1e-12)

	// free group / missing billing → fail open
	_, ok = EstimateInflightReservationCost(billing, cfg, "claude-sonnet-4-5", 10, 10, 0)
	require.False(t, ok)
	_, ok = EstimateInflightReservationCost(nil, cfg, "claude-sonnet-4-5", 10, 10, 1)
	require.False(t, ok)
	_, ok = EstimateInflightReservationCost(billing, cfg, "", 10, 10, 1)
	require.False(t, ok)
}

func TestReserveInflightBalance_NoReservationCacheFailsOpen(t *testing.T) {
	cfg := &config.Config{}
	cfg.Billing.InflightReservation.Enabled = true
	svc := NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(svc.Stop)
	release, err := svc.ReserveInflightBalance(context.Background(), &User{ID: 1}, nil, nil, 100)
	require.NoError(t, err)
	release()
}

func TestReserveInflightBalance_SimpleModeDisabled(t *testing.T) {
	cfg := &config.Config{RunMode: config.RunModeSimple}
	cfg.Billing.InflightReservation.Enabled = true
	svc := NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(svc.Stop)
	require.False(t, svc.InflightReservationEnabled())
}

// ---------------------------------------------------------------------------
// estimate: same model / resolver as billing
// ---------------------------------------------------------------------------

func newInflightEstimateGateway(t *testing.T, channelService *ChannelService) *GatewayService {
	t.Helper()
	cfg := &config.Config{}
	cfg.Billing.InflightReservation = config.InflightReservationConfig{Enabled: true, DefaultMaxTokens: 1000, MaxInputTokens: 200000, MaxOutputTokens: 128000}
	billing := NewBillingService(cfg, nil)
	return &GatewayService{
		cfg:            cfg,
		billingService: billing,
		resolver:       NewModelPricingResolver(channelService, billing),
		channelService: channelService,
	}
}

func TestInflightEstimate_ChannelAliasUsesMappedModel(t *testing.T) {
	groupID := int64(10)
	ch := Channel{
		ID:       1,
		Status:   StatusActive,
		GroupIDs: []int64{groupID},
		ModelMapping: map[string]map[string]string{
			"anthropic": {"my-alias": "claude-sonnet-4-5"},
		},
	}
	cs := newTestChannelService(makeStandardRepo(ch, map[int64]string{groupID: "anthropic"}))
	svc := newInflightEstimateGateway(t, cs)
	apiKey := &APIKey{User: &User{ID: 1}, GroupID: &groupID, Group: &Group{ID: groupID, Platform: PlatformAnthropic, RateMultiplier: 1}}

	// Old estimate (client model, base pricing only) → 0 → reservation bypassed.
	_, ok := EstimateInflightReservationCost(svc.billingService, svc.cfg.Billing.InflightReservation, "my-alias", 4000, 1000, 1)
	require.False(t, ok, "precondition: alias has no base pricing")

	est, priced := svc.EstimateInflightReservation(context.Background(), apiKey, InflightEstimateRequest{Model: "my-alias", BodyBytes: 4000, MaxTokens: 1000})
	require.True(t, priced)
	require.Greater(t, est, 0.0)

	direct, _ := svc.EstimateInflightReservation(context.Background(), apiKey, InflightEstimateRequest{Model: "claude-sonnet-4-5", BodyBytes: 4000, MaxTokens: 1000})
	require.InDelta(t, direct, est, 1e-12, "alias must be estimated as the channel-mapped billing model")
}

func TestInflightEstimate_GroupPerRequestPricing(t *testing.T) {
	groupID := int64(20)
	price := 0.5
	group := &Group{ID: groupID, Platform: PlatformAnthropic, RateMultiplier: 2, ModelPricing: []ChannelModelPricing{
		{Models: []string{"custom-per-request"}, BillingMode: BillingModePerRequest, PerRequestPrice: &price},
	}}
	svc := newInflightEstimateGateway(t, nil)
	apiKey := &APIKey{User: &User{ID: 1}, GroupID: &groupID, Group: group}

	est, priced := svc.EstimateInflightReservation(context.Background(), apiKey, InflightEstimateRequest{Model: "custom-per-request", BodyBytes: 100})
	require.True(t, priced)
	require.InDelta(t, 1.0, est, 1e-12, "per_request 0.5 × group rate 2")

	// Wildcard group token pricing is honored as well.
	in, out := 1e-6, 2e-6
	group.ModelPricing = []ChannelModelPricing{{Models: []string{"wild-*"}, BillingMode: BillingModeToken, InputPrice: &in, OutputPrice: &out}}
	est, priced = svc.EstimateInflightReservation(context.Background(), apiKey, InflightEstimateRequest{Model: "wild-x", BodyBytes: 4000, MaxTokens: 500})
	require.True(t, priced)
	require.InDelta(t, (1000*in+500*out)*2, est, 1e-12)
}

func TestInflightEstimate_UnpricedIsReportedAndNonMeteredIsNot(t *testing.T) {
	svc := newInflightEstimateGateway(t, nil)
	apiKey := &APIKey{User: &User{ID: 1}}
	est, priced := svc.EstimateInflightReservation(context.Background(), apiKey, InflightEstimateRequest{Model: "totally-unknown-model-xyz"})
	require.False(t, priced)
	require.Zero(t, est)

	est, priced = svc.EstimateInflightReservation(context.Background(), apiKey, InflightEstimateRequest{})
	require.True(t, priced, "non-metered requests (e.g. media status lookups) are not 'unpriced'")
	require.Zero(t, est)
}

func TestInflightEstimate_MediaKinds(t *testing.T) {
	groupID := int64(30)
	p4k := 0.3
	search := 1000.0
	group := &Group{ID: groupID, Platform: PlatformOpenAI, RateMultiplier: 1, ImagePrice4K: &p4k, SearchPricePer1k: &search}
	svc := newInflightEstimateGateway(t, nil)
	apiKey := &APIKey{User: &User{ID: 1}, GroupID: &groupID, Group: group}

	est, priced := svc.EstimateInflightReservation(context.Background(), apiKey, InflightEstimateRequest{Model: "gpt-image-1", Kind: InflightEstimateImage, Units: 2})
	require.True(t, priced)
	require.GreaterOrEqual(t, est, 0.6, "image estimate uses the highest size tier × n")

	est, priced = svc.EstimateInflightReservation(context.Background(), apiKey, InflightEstimateRequest{Model: "grok-web-search", Kind: InflightEstimatePerRequest, SearchCalls: 1})
	require.True(t, priced)
	require.InDelta(t, 1.0, est, 1e-12)

	est, priced = svc.EstimateInflightReservation(context.Background(), apiKey, InflightEstimateRequest{Model: "realtime", Kind: InflightEstimateAudio, AudioMode: "realtime", AudioUnits: 1})
	require.True(t, priced)
	require.Greater(t, est, 0.0)
}

func TestInflightEstimate_SelectedPricingMultiplierOnce(t *testing.T) {
	for _, kind := range []InflightEstimateKind{InflightEstimateToken, InflightEstimateImage, InflightEstimateVideo, InflightEstimateAudio} {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			groupID := int64(44)
			price, in, out := 0.25, 0.001, 0.002
			mode := BillingModeImage
			want := 3.0 // 2 units * 0.25 * rate 2 * selected effort 3
			switch kind {
			case InflightEstimateToken:
				mode, want = BillingModeToken, 36 // (1000*0.001+1000*0.002)*priority2*rate2*effort3
			case InflightEstimateVideo:
				mode, want = BillingModeVideo, 15 // 2 videos * 5 seconds * 0.25 * rate2 * effort3
			case InflightEstimateAudio:
				mode = BillingModePerRequest
			}
			card := ChannelModelPricing{Models: []string{"custom-metered"}, BillingMode: mode,
				PerRequestPrice: &price, InputPrice: &in, OutputPrice: &out,
				ReasoningEffortMultipliers: map[string]float64{"high": 3}}
			channelCard := card.Clone()
			channelCard.ReasoningEffortMultipliers = map[string]float64{"high": 7}
			cs := newTestChannelService(makeStandardRepo(Channel{ID: 4, Status: StatusActive,
				GroupIDs: []int64{groupID}, ModelPricing: []ChannelModelPricing{channelCard}}, map[int64]string{groupID: PlatformOpenAI}))
			svc := newInflightEstimateGateway(t, cs)
			key := &APIKey{User: &User{ID: 1}, GroupID: &groupID, Group: &Group{ID: groupID,
				Platform: PlatformOpenAI, RateMultiplier: 2, ModelPricing: []ChannelModelPricing{card}}}
			req := InflightEstimateRequest{Model: "custom-metered", Kind: kind, Units: 2, BodyBytes: 4000, MaxTokens: 1000,
				ReasoningEffort: "high", ServiceTier: "priority", VideoDurationSeconds: 5, AudioMode: "tts", AudioUnits: 2}
			estimate, priced := svc.EstimateInflightReservation(context.Background(), key, req)
			require.True(t, priced)
			require.InDelta(t, want, estimate, 1e-12, "group card owns the effort multiplier; channel must not stack")
			if kind == InflightEstimateToken {
				req.ReasoningEffort, req.ServiceTier = "low", ""
				estimate, priced = svc.EstimateInflightReservation(context.Background(), key, req)
				require.True(t, priced)
				require.InDelta(t, 6, estimate, 1e-12)
			}
		})
	}
}

func TestInflightEstimate_ExplicitZeroPriceIsPriced(t *testing.T) {
	svc := newInflightEstimateGateway(t, nil)
	zero := 0.0
	for _, mode := range []BillingMode{BillingModeToken, BillingModePerRequest, BillingModeImage} {
		key := &APIKey{User: &User{ID: 1}, GroupID: i64p(4), Group: &Group{ID: 4, RateMultiplier: 1,
			ModelPricing: []ChannelModelPricing{{Models: []string{"custom-free"}, BillingMode: mode, InputPrice: &zero, OutputPrice: &zero, PerRequestPrice: &zero}}}}
		kind := InflightEstimateToken
		if mode == BillingModeImage {
			kind = InflightEstimateImage
		}
		estimate, priced := svc.EstimateInflightReservation(context.Background(), key, InflightEstimateRequest{Model: "custom-free", Kind: kind})
		require.True(t, priced, "explicit free cards must remain allowed in fail-closed-on-unpriced mode")
		require.Zero(t, estimate)
	}
}

func TestInflightEstimate_ImageTokenPricingUsesInheritedCatalogPrices(t *testing.T) {
	svc := newInflightEstimateGateway(t, nil)
	svc.billingService = NewBillingService(svc.cfg, newStubPricingServiceFromMap(map[string]*LiteLLMModelPricing{
		"custom-image-token": {InputCostPerToken: 5e-6, OutputCostPerToken: 10e-6, InputCostPerImageToken: 8e-6, OutputCostPerImageToken: 30e-6},
	}))
	svc.resolver = NewModelPricingResolver(nil, svc.billingService)
	key := &APIKey{User: &User{ID: 1}, GroupID: i64p(4), Group: &Group{ID: 4, RateMultiplier: 1,
		ModelPricing: []ChannelModelPricing{{Models: []string{"custom-image-token"}, BillingMode: BillingModeToken}}}}
	estimate, priced := svc.EstimateInflightReservation(context.Background(), key, InflightEstimateRequest{
		Model: "custom-image-token", Kind: InflightEstimateImage, BodyBytes: 4000, MaxTokens: 1000,
	})
	require.True(t, priced)
	require.InDelta(t, 0.038, estimate, 1e-12)
}

func TestInflightEstimate_UsesIndependentVideoAndBaseAudioSearchRates(t *testing.T) {
	svc := newInflightEstimateGateway(t, nil)
	key := &APIKey{User: &User{ID: 1}, GroupID: i64p(4), Group: &Group{ID: 4, Platform: PlatformOpenAI,
		RateMultiplier: 2, VideoRateIndependent: true, VideoRateMultiplier: 7}}
	d := svc.inflightEstimateDeps()
	d.openAI = true
	// Supply a distinct peak token rate so the test does not depend on the clock.
	video, priced := d.estimateOne(context.Background(), key, "grok-imagine-video", InflightEstimateRequest{
		Kind: InflightEstimateVideo, Units: 1, VideoDurationSeconds: 5, VideoResolution: "480p",
	}, 20, 2, 2)
	require.True(t, priced)
	require.InDelta(t, 0.05*5*7, video, 1e-12)
	audio, priced := d.estimateOne(context.Background(), key, "realtime", InflightEstimateRequest{
		Kind: InflightEstimateAudio, AudioMode: "realtime", AudioUnits: 1,
	}, 20, 2, 2)
	require.True(t, priced)
	require.InDelta(t, defaultAudioRealtimePricePerMin*2, audio, 1e-12)
	search, priced := d.estimateOne(context.Background(), key, "alpha-search", InflightEstimateRequest{
		Kind: InflightEstimatePerRequest, WebSearchCalls: 1,
	}, 20, 2, 2)
	require.True(t, priced)
	require.InDelta(t, defaultWebSearchPricePerCall*2, search, 1e-12)
}

type failingInflightDeductCache struct {
	*balanceEligibilityCacheStub
}

func (c *failingInflightDeductCache) DeductUserBalance(context.Context, int64, float64) error {
	c.deductCalls.Add(1)
	return fmt.Errorf("cache deduction failed")
}

func TestSyncBalanceCacheAfterDeduction_InflightFailureInvalidatesStaleBalance(t *testing.T) {
	cache := &failingInflightDeductCache{&balanceEligibilityCacheStub{balance: 10}}
	svc := newInflightSvc(t, cache, 60)
	syncBalanceCacheAfterDeduction(context.Background(), &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: 9}, User: &User{ID: 1},
	}, &billingDeps{billingCacheService: svc}, nil)
	require.Equal(t, int64(1), cache.deductCalls.Load())
	require.Equal(t, int64(1), cache.invalidateCalls.Load(), "invalidate synchronously before the billing task releases its reservation")
}

type inflightRefillCache struct {
	*memInflightCache
	filled atomic.Bool
}

func (c *inflightRefillCache) GetUserBalance(ctx context.Context, id int64) (float64, error) {
	if !c.filled.Load() {
		return 0, fmt.Errorf("cache miss")
	}
	return c.memInflightCache.GetUserBalance(ctx, id)
}

func (c *inflightRefillCache) SetUserBalance(_ context.Context, _ int64, balance float64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.balance = balance
	c.filled.Store(true)
	return nil
}

func TestInflightBalanceRefillCannotOverwriteSettlement(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			cache := &inflightRefillCache{memInflightCache: newMemInflightCache(0)}
			cfg := &config.Config{}
			cfg.Billing.InflightReservation.Enabled = enabled
			// Leave the existing cache-write queue paused until after settlement.
			svc := &BillingCacheService{cache: cache, userRepo: &balanceLoadUserRepoStub{balance: 10}, cfg: cfg, cacheWriteChan: make(chan cacheWriteTask, 1)}
			balance, err := svc.GetUserBalance(context.Background(), 1)
			require.NoError(t, err)
			require.Equal(t, 10.0, balance)
			if enabled {
				syncBalanceCacheAfterDeduction(context.Background(), &postUsageBillingParams{
					Cost: &CostBreakdown{ActualCost: 9}, User: &User{ID: 1},
				}, &billingDeps{billingCacheService: svc}, nil)
			} else {
				require.False(t, cache.filled.Load(), "disabled mode preserves asynchronous cache fills")
			}
			close(svc.cacheWriteChan)
			svc.cacheWriteWg.Add(1)
			svc.cacheWriteWorker(svc.cacheWriteChan)
			balance, err = cache.GetUserBalance(context.Background(), 1)
			require.NoError(t, err)
			want := 10.0
			if enabled {
				want = 1
			}
			require.Equal(t, want, balance, "draining old cache work must not restore pre-settlement balance")
		})
	}
}

// ---------------------------------------------------------------------------
// reservation handle: hand-off to billing task, renewal
// ---------------------------------------------------------------------------

type memInflightCache struct {
	BillingCache
	mu        sync.Mutex
	balance   float64
	res       map[string]float64
	exp       map[string]time.Time
	renews    atomic.Int32
	deducts   atomic.Int32
	releaseCt atomic.Int32
}

func newMemInflightCache(balance float64) *memInflightCache {
	return &memInflightCache{balance: balance, res: map[string]float64{}, exp: map[string]time.Time{}}
}

func (m *memInflightCache) GetUserBalance(context.Context, int64) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.balance, nil
}

func (m *memInflightCache) DeductUserBalance(_ context.Context, _ int64, amount float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.balance -= amount
	m.deducts.Add(1)
	return nil
}

func (m *memInflightCache) gc() {
	now := time.Now()
	for id, e := range m.exp {
		if !e.After(now) {
			delete(m.exp, id)
			delete(m.res, id)
		}
	}
}

func (m *memInflightCache) ReserveInflightBalance(_ context.Context, _ int64, id string, amount, balance float64, ttl time.Duration) (bool, float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gc()
	sum := 0.0
	for _, v := range m.res {
		sum += v
	}
	if len(m.res) > 0 && balance-sum < amount {
		return false, sum, nil
	}
	m.res[id] = amount
	m.exp[id] = time.Now().Add(ttl)
	return true, sum, nil
}

func (m *memInflightCache) ReleaseInflightBalance(_ context.Context, _ int64, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.res, id)
	delete(m.exp, id)
	m.releaseCt.Add(1)
	return nil
}

func (m *memInflightCache) RenewInflightBalance(_ context.Context, _ int64, id string, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.renews.Add(1)
	if _, ok := m.res[id]; !ok {
		return false, nil
	}
	m.exp[id] = time.Now().Add(ttl)
	return true, nil
}

func (m *memInflightCache) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gc()
	return len(m.res)
}

func newInflightSvc(t *testing.T, cache BillingCache, ttlSeconds int) *BillingCacheService {
	t.Helper()
	cfg := &config.Config{}
	cfg.Billing.InflightReservation = config.InflightReservationConfig{Enabled: true, TTLSeconds: ttlSeconds}
	svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(svc.Stop)
	return svc
}

func TestInflightReservation_HeldUntilBillingTaskDone(t *testing.T) {
	cache := newMemInflightCache(1)
	svc := newInflightSvc(t, cache, 60)
	user := &User{ID: 1}

	res, err := svc.ReserveInflight(context.Background(), user, nil, nil, 0.9)
	require.NoError(t, err)
	require.NotNil(t, res)
	taskDone := res.Acquire() // billing task submitted
	res.HandlerDone()         // handler returned
	require.Equal(t, 1, cache.count(), "reservation must survive handler return while billing is pending")

	_, err = svc.ReserveInflight(context.Background(), user, nil, nil, 0.9)
	require.ErrorIs(t, err, ErrInsufficientBalance)

	taskDone()
	taskDone() // idempotent
	require.Equal(t, 0, cache.count())
	require.Equal(t, int32(1), cache.releaseCt.Load(), "released exactly once")

	// No billing task → HandlerDone releases immediately; Acquire after release is a no-op.
	res2, err := svc.ReserveInflight(context.Background(), user, nil, nil, 0.1)
	require.NoError(t, err)
	res2.HandlerDone()
	res2.HandlerDone()
	require.Equal(t, 0, cache.count())
	res2.Acquire()()
	require.Equal(t, int32(2), cache.releaseCt.Load())

	// Nil handle is safe.
	var nilRes *InflightReservation
	nilRes.Acquire()()
	nilRes.HandlerDone()
	nilRes.Release()
}

func TestInflightReservation_RenewedWhileHandlerActive(t *testing.T) {
	cache := newMemInflightCache(1)
	svc := newInflightSvc(t, cache, 1)
	user := &User{ID: 2}

	res, err := svc.ReserveInflight(context.Background(), user, nil, nil, 0.9)
	require.NoError(t, err)
	time.Sleep(2500 * time.Millisecond) // > 2 × TTL
	require.Equal(t, 1, cache.count(), "renewal keeps the reservation alive past its TTL")
	require.Greater(t, cache.renews.Load(), int32(3))
	_, err = svc.ReserveInflight(context.Background(), user, nil, nil, 0.9)
	require.ErrorIs(t, err, ErrInsufficientBalance)

	done := res.Acquire()
	res.HandlerDone()
	renewsAtStop := cache.renews.Load()
	time.Sleep(1200 * time.Millisecond)
	require.Equal(t, renewsAtStop, cache.renews.Load(), "no renewal after handler end")
	require.Equal(t, 0, cache.count(), "post-handler hold is bounded by TTL")
	done()
}

func TestSyncBalanceCacheAfterDeduction_SynchronousWhenInflightEnabled(t *testing.T) {
	cache := newMemInflightCache(1)
	svc := newInflightSvc(t, cache, 60)
	p := &postUsageBillingParams{Cost: &CostBreakdown{ActualCost: 0.25}, User: &User{ID: 3}}
	syncBalanceCacheAfterDeduction(context.Background(), p, &billingDeps{billingCacheService: svc}, nil)
	require.Equal(t, int32(1), cache.deducts.Load(), "cache deduction must land before the billing task returns")
	bal, _ := cache.GetUserBalance(context.Background(), 3)
	require.InDelta(t, 0.75, bal, 1e-12)
}

// 计费侧：requested 来源下别名本身无价 → billableModelWithFallback 回退到映射模型。
// 准入估算必须同口径地 > 0，且等于按回退模型的估算。
func TestInflightEstimate_RequestedSourceUnpricedAliasFallsBackLikeBilling(t *testing.T) {
	groupID := int64(30)
	ch := Channel{
		ID:                 3,
		Status:             StatusActive,
		GroupIDs:           []int64{groupID},
		BillingModelSource: BillingModelSourceRequested,
		ModelMapping: map[string]map[string]string{
			"anthropic": {"req-alias": "claude-sonnet-4-5"},
		},
	}
	cs := newTestChannelService(makeStandardRepo(ch, map[int64]string{groupID: "anthropic"}))
	svc := newInflightEstimateGateway(t, cs)
	apiKey := &APIKey{User: &User{ID: 1}, GroupID: &groupID, Group: &Group{ID: groupID, Platform: PlatformAnthropic, RateMultiplier: 1}}
	ctx := context.Background()

	billed := svc.billableModelWithFallback(ctx, apiKey, "req-alias", "claude-sonnet-4-5", "req-alias")
	require.Equal(t, "claude-sonnet-4-5", billed, "precondition: billing falls back to the mapped model")

	est, priced := svc.EstimateInflightReservation(ctx, apiKey, InflightEstimateRequest{Model: "req-alias", BodyBytes: 4000, MaxTokens: 1000})
	require.True(t, priced)
	require.Greater(t, est, 0.0)
	direct, _ := svc.EstimateInflightReservation(ctx, apiKey, InflightEstimateRequest{Model: billed, BodyBytes: 4000, MaxTokens: 1000})
	require.InDelta(t, direct, est, 1e-12)
}

// 计费侧：别名仅在账号级映射（无渠道映射）→ UpstreamModel 为账号映射模型，计费回退到它。
// 准入时账号未选定：按分组内候选账号映射模型的最高估算。
func TestInflightEstimate_AccountLevelMappingFallsBackLikeBilling(t *testing.T) {
	groupID := int64(31)
	svc := newInflightEstimateGateway(t, nil)
	snap := &inflightSnapshotCacheStub{byBucket: map[string][]Account{inflightBucketKey(groupID, PlatformAnthropic): {
		{ID: 1, Platform: PlatformAnthropic, Credentials: map[string]any{"model_mapping": map[string]any{"acct-alias-31": "claude-sonnet-4-5"}}},
		{ID: 2, Platform: PlatformAnthropic, Credentials: map[string]any{"model_mapping": map[string]any{"acct-alias-31": "claude-opus-4-1"}}},
	}}}
	attachInflightSnapshot(svc, snap)
	apiKey := &APIKey{User: &User{ID: 1}, GroupID: &groupID, Group: &Group{ID: groupID, Platform: PlatformAnthropic, RateMultiplier: 1}}
	ctx := context.Background()

	_, ok := EstimateInflightReservationCost(svc.billingService, svc.cfg.Billing.InflightReservation, "acct-alias-31", 4000, 1000, 1)
	require.False(t, ok, "precondition: alias has no pricing")

	est, priced := svc.EstimateInflightReservation(ctx, apiKey, InflightEstimateRequest{Model: "acct-alias-31", BodyBytes: 4000, MaxTokens: 1000})
	require.True(t, priced)
	req := InflightEstimateRequest{BodyBytes: 4000, MaxTokens: 1000}
	best := 0.0
	for _, upstream := range []string{"claude-sonnet-4-5", "claude-opus-4-1"} {
		billed := svc.billableModelWithFallback(ctx, apiKey, "acct-alias-31", upstream, "acct-alias-31")
		require.Equal(t, upstream, billed, "precondition: billing charges the account-mapped model")
		req.Model = billed
		c, _ := svc.EstimateInflightReservation(ctx, apiKey, req)
		require.Greater(t, c, 0.0)
		best = math.Max(best, c)
	}
	require.InDelta(t, best, est, 1e-12, "estimate = max over candidate account-mapped billing models")
}

type inflightSnapshotCacheStub struct {
	SchedulerCache
	mu       sync.Mutex
	byBucket map[string][]Account
	reads    atomic.Int64
}

func inflightBucketKey(groupID int64, platform string) string {
	return fmt.Sprintf("%d|%s", groupID, platform)
}

func (c *inflightSnapshotCacheStub) set(groupID int64, platform string, accounts []Account) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.byBucket[inflightBucketKey(groupID, platform)] = accounts
}

func (c *inflightSnapshotCacheStub) GetSnapshot(ctx context.Context, bucket SchedulerBucket) ([]*Account, bool, error) {
	c.reads.Add(1)
	c.mu.Lock()
	defer c.mu.Unlock()
	src := c.byBucket[inflightBucketKey(bucket.GroupID, bucket.Platform)]
	out := make([]*Account, 0, len(src))
	for i := range src {
		a := src[i]
		out = append(out, &a)
	}
	return out, true, nil
}

// inflightCountingAccountRepo 统计请求路径上的任何直接查库。
type inflightCountingAccountRepo struct {
	AccountRepository
	dbCalls atomic.Int64
}

func (r *inflightCountingAccountRepo) ListSchedulableByGroupID(ctx context.Context, groupID int64) ([]Account, error) {
	r.dbCalls.Add(1)
	return nil, nil
}

func attachInflightSnapshot(svc *GatewayService, snap *inflightSnapshotCacheStub) *inflightCountingAccountRepo {
	repo := &inflightCountingAccountRepo{}
	svc.accountRepo = repo
	svc.schedulerSnapshot = NewSchedulerSnapshotService(snap, nil, repo, nil, svc.cfg)
	return repo
}

// 已定价模型永不查账号映射；随机未定价模型名不直接查库、不产生按模型名的缓存（内存有界）。
func TestInflightEstimate_AccountMappingNoDBAndBoundedMemory(t *testing.T) {
	groupID := int64(40)
	svc := newInflightEstimateGateway(t, nil)
	snap := &inflightSnapshotCacheStub{byBucket: map[string][]Account{inflightBucketKey(groupID, PlatformAnthropic): {
		{ID: 1, Platform: PlatformAnthropic, Credentials: map[string]any{"model_mapping": map[string]any{"acct-alias-40": "claude-sonnet-4-5"}}},
	}}}
	repo := attachInflightSnapshot(svc, snap)
	apiKey := &APIKey{User: &User{ID: 1}, GroupID: &groupID, Group: &Group{ID: groupID, Platform: PlatformAnthropic, RateMultiplier: 1}}
	ctx := context.Background()

	for i := 0; i < 100; i++ {
		_, priced := svc.EstimateInflightReservation(ctx, apiKey, InflightEstimateRequest{Model: "claude-sonnet-4-5", BodyBytes: 4000, MaxTokens: 1000})
		require.True(t, priced)
	}
	require.Zero(t, snap.reads.Load(), "priced models must never trigger account-mapping lookup")

	const n = 20000
	for i := 0; i < n; i++ {
		_, priced := svc.EstimateInflightReservation(ctx, apiKey, InflightEstimateRequest{Model: fmt.Sprintf("rand-%d-%d", i, time.Now().UnixNano()), BodyBytes: 4000, MaxTokens: 1000})
		require.False(t, priced)
	}
	require.Zero(t, repo.dbCalls.Load(), "no direct DB query on the request path")
	require.Equal(t, int64(n), snap.reads.Load(), "unpriced lookups read the scheduler snapshot only")
	require.Len(t, snap.byBucket, 1, "random model names must not create scheduler cache buckets")
	require.Len(t, snap.byBucket[inflightBucketKey(groupID, PlatformAnthropic)], 1)
}

// 无分组 API Key：使用调度器的未分组账号池，估算与计费回退的账号映射模型同口径。
func TestInflightEstimate_NoGroupKeyUsesUngroupedAccountMapping(t *testing.T) {
	svc := newInflightEstimateGateway(t, nil)
	snap := &inflightSnapshotCacheStub{byBucket: map[string][]Account{inflightBucketKey(0, PlatformAnthropic): {
		{ID: 1, Platform: PlatformAnthropic, Credentials: map[string]any{"model_mapping": map[string]any{"acct-alias-nog": "claude-opus-4-1"}}},
	}}}
	attachInflightSnapshot(svc, snap)
	apiKey := &APIKey{User: &User{ID: 1}}
	ctx := context.Background()

	est, priced := svc.EstimateInflightReservation(ctx, apiKey, InflightEstimateRequest{Model: "acct-alias-nog", BodyBytes: 4000, MaxTokens: 1000})
	require.True(t, priced)
	direct, ok := svc.EstimateInflightReservation(ctx, apiKey, InflightEstimateRequest{Model: "claude-opus-4-1", BodyBytes: 4000, MaxTokens: 1000})
	require.True(t, ok)
	require.InDelta(t, direct, est, 1e-12)
}

// 管理员新增账号映射后立即生效（不缓存负结果）。
func TestInflightEstimate_AccountMappingNoNegativeCaching(t *testing.T) {
	groupID := int64(41)
	svc := newInflightEstimateGateway(t, nil)
	snap := &inflightSnapshotCacheStub{byBucket: map[string][]Account{}}
	attachInflightSnapshot(svc, snap)
	apiKey := &APIKey{User: &User{ID: 1}, GroupID: &groupID, Group: &Group{ID: groupID, Platform: PlatformAnthropic, RateMultiplier: 1}}
	ctx := context.Background()
	req := InflightEstimateRequest{Model: "acct-alias-41", BodyBytes: 4000, MaxTokens: 1000}

	_, priced := svc.EstimateInflightReservation(ctx, apiKey, req)
	require.False(t, priced)
	snap.set(groupID, PlatformAnthropic, []Account{{ID: 9, Platform: PlatformAnthropic, Credentials: map[string]any{"model_mapping": map[string]any{"acct-*": "claude-sonnet-4-5"}}}})
	est, priced := svc.EstimateInflightReservation(ctx, apiKey, req)
	require.True(t, priced, "new mapping (incl. wildcard) visible on next request")
	require.Greater(t, est, 0.0)
}
