//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newLeaderLockTestCache(t *testing.T) (*leaderLockCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return &leaderLockCache{rdb: rdb}, mr
}

func TestLeaderLockCache_AcquireContendedRelease(t *testing.T) {
	cache, _ := newLeaderLockTestCache(t)
	ctx := context.Background()
	const key = "dashboard:aggregation:leader"

	ok, err := cache.TryAcquireLeaderLock(ctx, key, "A", time.Minute)
	require.NoError(t, err)
	require.True(t, ok, "first owner should acquire")

	ok, err = cache.TryAcquireLeaderLock(ctx, key, "B", time.Minute)
	require.NoError(t, err)
	require.False(t, ok, "peer must be locked out while held")

	require.NoError(t, cache.ReleaseLeaderLock(ctx, key, "A"))

	ok, err = cache.TryAcquireLeaderLock(ctx, key, "B", time.Minute)
	require.NoError(t, err)
	require.True(t, ok, "peer should acquire after release")
}

// A stale owner whose lock expired and was re-acquired by a peer must not delete
// the peer's lock when its late release fires (compare-and-delete by owner).
func TestLeaderLockCache_ReleaseIsCompareAndDelete(t *testing.T) {
	cache, _ := newLeaderLockTestCache(t)
	ctx := context.Background()
	const key = "payment:order:expiry:leader"

	ok, err := cache.TryAcquireLeaderLock(ctx, key, "A", time.Minute)
	require.NoError(t, err)
	require.True(t, ok)

	// Simulate A's lock expiring and peer B taking ownership.
	require.NoError(t, cache.rdb.Set(ctx, leaderLockKeyPrefix+key, "B", time.Minute).Err())

	// A's stale release must be a no-op against B's lock.
	require.NoError(t, cache.ReleaseLeaderLock(ctx, key, "A"))

	val, err := cache.rdb.Get(ctx, leaderLockKeyPrefix+key).Result()
	require.NoError(t, err)
	require.Equal(t, "B", val, "stale owner must not delete the new owner's lock")
}

func TestLeaderLockCache_TTLExpires(t *testing.T) {
	cache, mr := newLeaderLockTestCache(t)
	ctx := context.Background()
	const key = "subscription:expiry:reminder:leader"

	ok, err := cache.TryAcquireLeaderLock(ctx, key, "A", time.Minute)
	require.NoError(t, err)
	require.True(t, ok)

	mr.FastForward(2 * time.Minute)

	ok, err = cache.TryAcquireLeaderLock(ctx, key, "B", time.Minute)
	require.NoError(t, err)
	require.True(t, ok, "lock should be re-acquirable after the TTL expires")
}

func TestLeaderLockCache_ScanCursorPersistsWithoutTTL(t *testing.T) {
	cache, mr := newLeaderLockTestCache(t)
	ctx := context.Background()
	const key = "jobs:openai-quota-recovery"

	cursor, err := cache.LoadScanCursor(ctx, key)
	require.NoError(t, err)
	require.Zero(t, cursor)

	ok, err := cache.TryAcquireLeaderLock(ctx, key, "A", time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	stored, err := cache.StoreScanCursorIfLeader(ctx, key, key, "A", 100)
	require.NoError(t, err)
	require.True(t, stored)
	mr.FastForward(24 * time.Hour)

	cursor, err = cache.LoadScanCursor(ctx, key)
	require.NoError(t, err)
	require.Equal(t, int64(100), cursor)
	ttl, err := cache.rdb.TTL(ctx, scanCursorKeyPrefix+key).Result()
	require.NoError(t, err)
	require.Less(t, ttl, time.Duration(0), "scan cursor must not expire")
}

func TestLeaderLockCache_StaleLeaderCannotRegressScanCursor(t *testing.T) {
	cache, _ := newLeaderLockTestCache(t)
	ctx := context.Background()
	const key = "jobs:openai-quota-recovery"

	require.NoError(t, cache.rdb.Set(ctx, leaderLockKeyPrefix+key, "B", time.Minute).Err())
	stored, err := cache.StoreScanCursorIfLeader(ctx, key, key, "B", 200)
	require.NoError(t, err)
	require.True(t, stored)

	stored, err = cache.StoreScanCursorIfLeader(ctx, key, key, "A", 100)
	require.NoError(t, err)
	require.False(t, stored)
	cursor, err := cache.LoadScanCursor(ctx, key)
	require.NoError(t, err)
	require.Equal(t, int64(200), cursor)
}
