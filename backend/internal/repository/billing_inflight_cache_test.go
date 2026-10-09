package repository

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newInflightTestEnv(t *testing.T, enabled bool, ttlSeconds int) (*miniredis.Miniredis, *billingCache, *service.BillingCacheService) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := &billingCache{rdb: rdb}
	cfg := &config.Config{}
	cfg.Billing.InflightReservation = config.InflightReservationConfig{
		Enabled:          enabled,
		TTLSeconds:       ttlSeconds,
		DefaultMaxTokens: 8192,
	}
	svc := service.NewBillingCacheService(cache, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(svc.Stop)
	return mr, cache, svc
}

func inflightCount(t *testing.T, cache *billingCache, userID int64) int64 {
	t.Helper()
	zkey, _ := billingInflightKeys(userID)
	n, err := cache.rdb.ZCard(context.Background(), zkey).Result()
	require.NoError(t, err)
	return n
}

func TestInflightReservation_ConcurrentAdmitsOnlyWhatBalanceCovers(t *testing.T) {
	_, cache, svc := newInflightTestEnv(t, true, 60)
	ctx := context.Background()
	user := &service.User{ID: 42}
	require.NoError(t, cache.SetUserBalance(ctx, user.ID, 1.0))

	const n = 20
	var admitted, rejected atomic.Int32
	releases := make(chan func(), n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			release, err := svc.ReserveInflightBalance(ctx, user, nil, nil, 0.3)
			if err != nil {
				if !errors.Is(err, service.ErrInsufficientBalance) {
					t.Errorf("unexpected error: %v", err)
				}
				rejected.Add(1)
				return
			}
			admitted.Add(1)
			releases <- release
		}()
	}
	close(start)
	wg.Wait()
	close(releases)

	// balance 1.0, estimate 0.3: first always admitted, then while 1.0 - inflight >= 0.3 → 3 total.
	require.Equal(t, int32(3), admitted.Load())
	require.Equal(t, int32(n-3), rejected.Load())
	require.Equal(t, int64(3), inflightCount(t, cache, user.ID))

	for release := range releases {
		release()
		release() // idempotent
	}
	require.Equal(t, int64(0), inflightCount(t, cache, user.ID))

	// After release, capacity is available again.
	release, err := svc.ReserveInflightBalance(ctx, user, nil, nil, 0.3)
	require.NoError(t, err)
	release()
}

func TestInflightReservation_UsesCurrentBalanceAfterStalledAdmission(t *testing.T) {
	_, cache, _ := newInflightTestEnv(t, true, 60)
	ctx := context.Background()
	require.NoError(t, cache.SetUserBalance(ctx, 90, 10))
	stale, err := cache.GetUserBalance(ctx, 90)
	require.NoError(t, err)
	allowed, _, err := cache.ReserveInflightBalance(ctx, 90, "a", 9, stale, time.Minute)
	require.NoError(t, err)
	require.True(t, allowed)
	require.NoError(t, cache.DeductUserBalance(ctx, 90, 9))
	require.NoError(t, cache.ReleaseInflightBalance(ctx, 90, "a"))
	allowed, _, err = cache.ReserveInflightBalance(ctx, 90, "c", 0.5, 1, time.Minute)
	require.NoError(t, err)
	require.True(t, allowed)
	// A second instance submits the old pre-read balance after settlement.
	other := &billingCache{rdb: cache.rdb}
	allowed, _, err = other.ReserveInflightBalance(ctx, 90, "b", 1, stale, time.Minute)
	require.NoError(t, err)
	require.False(t, allowed, "admission must atomically read the current $1 balance, not the old $10 argument")
}

type pausedBalanceSnapshotRepo struct {
	service.UserRepository
	mu      sync.Mutex
	balance float64
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (r *pausedBalanceSnapshotRepo) GetByID(ctx context.Context, id int64) (*service.User, error) {
	r.mu.Lock()
	balance := r.balance
	r.mu.Unlock()
	r.once.Do(func() { close(r.started); <-r.release })
	return &service.User{ID: id, Balance: balance}, nil
}

func TestInflightReservation_FencesSnapshotAcrossMissingCacheDeduction(t *testing.T) {
	_, cache, _ := newInflightTestEnv(t, true, 60)
	repo := &pausedBalanceSnapshotRepo{balance: 10, started: make(chan struct{}), release: make(chan struct{})}
	cfg := &config.Config{}
	cfg.Billing.InflightReservation = config.InflightReservationConfig{Enabled: true, TTLSeconds: 60}
	billing := service.NewBillingCacheService(cache, repo, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billing.Stop)
	result := make(chan float64, 1)
	errs := make(chan error, 1)
	go func() {
		balance, err := billing.GetUserBalance(context.Background(), 91)
		result <- balance
		errs <- err
	}()
	<-repo.started
	// Commit on another instance while the old DB snapshot is paused.
	repo.mu.Lock()
	repo.balance = 1
	repo.mu.Unlock()
	other := &billingCache{rdb: cache.rdb}
	require.NoError(t, other.DeductUserBalance(context.Background(), 91, 9))
	close(repo.release)
	require.NoError(t, <-errs)
	require.Equal(t, 1.0, <-result, "a fenced-out DB snapshot must be reloaded before returning")
	current, err := cache.GetUserBalance(context.Background(), 91)
	require.NoError(t, err)
	require.Equal(t, 1.0, current, "old snapshot cannot restore pre-settlement balance")
}

func TestInflightReservation_FirstRequestAlwaysAdmitted(t *testing.T) {
	_, cache, svc := newInflightTestEnv(t, true, 60)
	ctx := context.Background()
	user := &service.User{ID: 7}
	require.NoError(t, cache.SetUserBalance(ctx, user.ID, 0.01))

	release, err := svc.ReserveInflightBalance(ctx, user, nil, nil, 5)
	require.NoError(t, err, "a lone request keeps legacy behavior even if estimate exceeds balance")
	_, err = svc.ReserveInflightBalance(ctx, user, nil, nil, 0.001)
	require.ErrorIs(t, err, service.ErrInsufficientBalance)
	release()
	release2, err := svc.ReserveInflightBalance(ctx, user, nil, nil, 0.001)
	require.NoError(t, err)
	release2()
}

func TestInflightReservation_TTLExpiryFreesLeakedReservation(t *testing.T) {
	_, cache, svc := newInflightTestEnv(t, true, 1)
	ctx := context.Background()
	user := &service.User{ID: 9}
	require.NoError(t, cache.SetUserBalance(ctx, user.ID, 1.0))

	_, err := svc.ReserveInflightBalance(ctx, user, nil, nil, 0.8) // leaked: never released
	require.NoError(t, err)
	_, err = svc.ReserveInflightBalance(ctx, user, nil, nil, 0.8)
	require.ErrorIs(t, err, service.ErrInsufficientBalance)

	time.Sleep(1100 * time.Millisecond)
	release, err := svc.ReserveInflightBalance(ctx, user, nil, nil, 0.8)
	require.NoError(t, err)
	require.Equal(t, int64(1), inflightCount(t, cache, user.ID))
	release()
}

func TestInflightReservation_DisabledKeepsLegacyBehavior(t *testing.T) {
	_, cache, svc := newInflightTestEnv(t, false, 60)
	ctx := context.Background()
	user := &service.User{ID: 11}
	require.NoError(t, cache.SetUserBalance(ctx, user.ID, 0.01))
	require.False(t, svc.InflightReservationEnabled())
	for i := 0; i < 5; i++ {
		_, err := svc.ReserveInflightBalance(ctx, user, nil, nil, 10)
		require.NoError(t, err)
	}
	require.Equal(t, int64(0), inflightCount(t, cache, user.ID))
}

func TestInflightReservation_SubscriptionUnaffected(t *testing.T) {
	_, cache, svc := newInflightTestEnv(t, true, 60)
	ctx := context.Background()
	user := &service.User{ID: 12}
	require.NoError(t, cache.SetUserBalance(ctx, user.ID, 0))
	group := &service.Group{ID: 1, SubscriptionType: service.SubscriptionTypeSubscription}
	sub := &service.UserSubscription{ID: 1}
	for i := 0; i < 5; i++ {
		_, err := svc.ReserveInflightBalance(ctx, user, group, sub, 10)
		require.NoError(t, err)
	}
	require.Equal(t, int64(0), inflightCount(t, cache, user.ID))
}

func TestInflightReservation_ZeroEstimateSkips(t *testing.T) {
	_, cache, svc := newInflightTestEnv(t, true, 60)
	ctx := context.Background()
	user := &service.User{ID: 13}
	require.NoError(t, cache.SetUserBalance(ctx, user.ID, 1))
	_, err := svc.ReserveInflightBalance(ctx, user, nil, nil, 0)
	require.NoError(t, err)
	require.Equal(t, int64(0), inflightCount(t, cache, user.ID))
}

// downReserveCache 余额读取正常，但预留走一个已关闭的 Redis。
type downReserveCache struct {
	service.BillingCache
	down *billingCache
}

func (d *downReserveCache) ReserveInflightBalance(ctx context.Context, userID int64, requestID string, amount, balance float64, ttl time.Duration) (bool, float64, error) {
	return d.down.ReserveInflightBalance(ctx, userID, requestID, amount, balance, ttl)
}

func (d *downReserveCache) ReleaseInflightBalance(ctx context.Context, userID int64, requestID string) error {
	return d.down.ReleaseInflightBalance(ctx, userID, requestID)
}

func TestInflightReservation_RedisDownFailsOpen(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	up := &billingCache{rdb: rdb}
	require.NoError(t, up.SetUserBalance(context.Background(), 5, 0.01))

	downMR := miniredis.RunT(t)
	downRDB := redis.NewClient(&redis.Options{Addr: downMR.Addr(), MaxRetries: -1, DialTimeout: 100 * time.Millisecond})
	t.Cleanup(func() { _ = downRDB.Close() })
	downMR.Close()

	cfg := &config.Config{}
	cfg.Billing.InflightReservation = config.InflightReservationConfig{Enabled: true, TTLSeconds: 60}
	svc := service.NewBillingCacheService(&downReserveCache{BillingCache: up, down: &billingCache{rdb: downRDB}}, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(svc.Stop)

	user := &service.User{ID: 5}
	for i := 0; i < 3; i++ {
		release, err := svc.ReserveInflightBalance(context.Background(), user, nil, nil, 100)
		require.NoError(t, err)
		release()
	}
}

func TestInflightReservation_ScriptErrorSurfaces(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	cache := &billingCache{rdb: rdb}
	mr.Close()
	_, _, err := cache.ReserveInflightBalance(context.Background(), 1, "r", 1, 1, time.Second)
	require.Error(t, err)
	require.False(t, errors.Is(err, context.Canceled))
	_ = rdb.Close()
}

// 回归：原实现在 handler 返回时即释放预留，而计费（RecordUsage → 余额缓存扣减）是异步的，
// 窗口内的顺序请求看到「在途=0 且余额未扣」全部放行（余额 $1、5×$0.9 → -4.40）。
// 现在预留由计费任务在余额缓存扣减之后才释放。
func TestInflightReservation_SequentialInBillingWindowAdmitsOnlyWhatBalanceCovers(t *testing.T) {
	_, cache, svc := newInflightTestEnv(t, true, 60)
	ctx := context.Background()
	user := &service.User{ID: 77}
	require.NoError(t, cache.SetUserBalance(ctx, user.ID, 1.0))

	const cost = 0.9
	var pendingBilling []func()
	admitted := 0
	for i := 0; i < 5; i++ {
		res, err := svc.ReserveInflight(ctx, user, nil, nil, cost)
		if err != nil {
			require.ErrorIs(t, err, service.ErrInsufficientBalance)
			continue
		}
		admitted++
		taskDone := res.Acquire() // handler submits the async billing task
		res.HandlerDone()         // handler returns before billing lands
		pendingBilling = append(pendingBilling, func() {
			require.NoError(t, cache.DeductUserBalance(ctx, user.ID, cost))
			taskDone()
		})
	}
	require.Equal(t, 1, admitted, "requests arriving before billing lands must not be admitted")

	for _, bill := range pendingBilling {
		bill()
	}
	bal, err := cache.GetUserBalance(ctx, user.ID)
	require.NoError(t, err)
	require.InDelta(t, 0.1, bal, 1e-9)
	require.Equal(t, int64(0), inflightCount(t, cache, user.ID))
}

func TestInflightReservation_RenewalKeepsStreamingReservationAlive(t *testing.T) {
	_, cache, svc := newInflightTestEnv(t, true, 1)
	ctx := context.Background()
	user := &service.User{ID: 78}
	require.NoError(t, cache.SetUserBalance(ctx, user.ID, 1.0))

	res, err := svc.ReserveInflight(ctx, user, nil, nil, 0.8)
	require.NoError(t, err)
	time.Sleep(2500 * time.Millisecond) // streaming well past the 1s TTL
	_, err = svc.ReserveInflight(ctx, user, nil, nil, 0.8)
	require.ErrorIs(t, err, service.ErrInsufficientBalance, "renewed reservation must still count")
	require.Equal(t, int64(1), inflightCount(t, cache, user.ID))

	res.HandlerDone()
	require.Equal(t, int64(0), inflightCount(t, cache, user.ID))

	ok, err := cache.RenewInflightBalance(ctx, user.ID, "missing", time.Second)
	require.NoError(t, err)
	require.False(t, ok, "renew must not resurrect a released reservation")
}

func TestInflightReservation_RenewDoesNotResurrectExpiredMember(t *testing.T) {
	_, cache, _ := newInflightTestEnv(t, true, 60)
	ctx := context.Background()
	userID := int64(79)
	zkey, hkey := billingInflightKeys(userID)
	// 已过期但尚未被惰性清理的成员（score 在过去）。
	require.NoError(t, cache.rdb.ZAdd(ctx, zkey, redis.Z{Score: float64(time.Now().UnixMilli() - 1000), Member: "stale"}).Err())
	require.NoError(t, cache.rdb.HSet(ctx, hkey, "stale", "0.5").Err())

	ok, err := cache.RenewInflightBalance(ctx, userID, "stale", time.Minute)
	require.NoError(t, err)
	require.False(t, ok, "renew must not resurrect an expired member")
	require.Equal(t, int64(0), inflightCount(t, cache, userID))
	exists, err := cache.rdb.HExists(ctx, hkey, "stale").Result()
	require.NoError(t, err)
	require.False(t, exists)
}

func TestInflightReservation_ShortTTLDoesNotExpireLongerReservation(t *testing.T) {
	mr, cache, _ := newInflightTestEnv(t, true, 60)
	ctx := context.Background()
	require.NoError(t, cache.SetUserBalance(ctx, 80, 1))
	allowed, _, err := cache.ReserveInflightBalance(ctx, 80, "long", 0.7, 1, time.Minute)
	require.NoError(t, err)
	require.True(t, allowed)
	allowed, _, err = cache.ReserveInflightBalance(ctx, 80, "short", 0.1, 1, time.Second)
	require.NoError(t, err)
	require.True(t, allowed)
	mr.FastForward(2 * time.Second)
	zkey, hkey := billingInflightKeys(80)
	require.True(t, mr.Exists(zkey))
	require.True(t, mr.Exists(hkey))
	allowed, sum, err := cache.ReserveInflightBalance(ctx, 80, "third", 0.3, 1, time.Minute)
	require.NoError(t, err)
	require.False(t, allowed, "short request TTL cannot erase a still-live longer hold")
	require.GreaterOrEqual(t, sum, 0.7)
}

func TestBalanceEpochFencesEveryMutationAndExpiresOnlyBalance(t *testing.T) {
	mr, cache, _ := newInflightTestEnv(t, true, 60)
	ctx := context.Background()
	other := &billingCache{rdb: cache.rdb}
	for _, mutation := range []string{"set", "deduct_missing", "invalidate", "publish"} {
		t.Run(mutation, func(t *testing.T) {
			epoch, err := cache.GetUserBalanceEpoch(ctx, 92)
			require.NoError(t, err)
			switch mutation {
			case "set":
				require.NoError(t, other.SetUserBalance(ctx, 92, 1))
			case "deduct_missing":
				require.NoError(t, cache.rdb.Del(ctx, billingBalanceKey(92)).Err())
				require.NoError(t, other.DeductUserBalance(ctx, 92, 9))
			case "invalidate":
				require.NoError(t, other.InvalidateUserBalance(ctx, 92))
			case "publish":
				ok, err := other.SetUserBalanceIfEpoch(ctx, 92, 1, epoch)
				require.NoError(t, err)
				require.True(t, ok)
			}
			ok, err := cache.SetUserBalanceIfEpoch(ctx, 92, 10, epoch)
			require.NoError(t, err)
			require.False(t, ok, "old snapshots cannot survive any financial mutation")
		})
	}
	epoch, err := cache.GetUserBalanceEpoch(ctx, 92)
	require.NoError(t, err)
	mr.FastForward(10 * time.Minute)
	current, err := other.GetUserBalanceEpoch(ctx, 92)
	require.NoError(t, err)
	require.Equal(t, epoch, current, "epoch cannot expire and allow an ABA snapshot to publish")
	zkey, hkey := billingInflightKeys(92)
	tag := "{" + billingBalanceKey(92) + "}"
	require.Contains(t, zkey, tag)
	require.Contains(t, hkey, tag)
	require.Contains(t, billingBalanceEpochKey(92), tag, "Lua keys must hash to the existing balance key's Redis Cluster slot")
}
