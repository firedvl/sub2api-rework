package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/redis/go-redis/v9"
)

const (
	leaderLockKeyPrefix = "leader:lock:"
	scanCursorKeyPrefix = "scan:cursor:"
)

// leaderLockReleaseScript releases a leader lock only when the caller still owns
// it (compare-and-delete by owner token). This prevents a previous holder whose
// lock already expired — and was re-acquired by another instance — from deleting
// the new owner's lock.
var leaderLockReleaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`)

var scanCursorStoreScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  redis.call("SET", KEYS[2], ARGV[2])
  return 1
end
return 0
`)

type leaderLockCache struct {
	rdb *redis.Client
}

// NewLeaderLockCache returns a Redis-backed implementation of
// service.LeaderLockCache used by periodic background jobs to elect a single
// runner across instances.
func NewLeaderLockCache(rdb *redis.Client) service.LeaderLockCache {
	return &leaderLockCache{rdb: rdb}
}

func (c *leaderLockCache) TryAcquireLeaderLock(ctx context.Context, key, owner string, ttl time.Duration) (bool, error) {
	return c.rdb.SetNX(ctx, leaderLockKeyPrefix+key, owner, ttl).Result()
}

func (c *leaderLockCache) ReleaseLeaderLock(ctx context.Context, key, owner string) error {
	return leaderLockReleaseScript.Run(ctx, c.rdb, []string{leaderLockKeyPrefix + key}, owner).Err()
}

func (c *leaderLockCache) LoadScanCursor(ctx context.Context, key string) (int64, error) {
	cursor, err := c.rdb.Get(ctx, scanCursorKeyPrefix+key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return cursor, err
}

func (c *leaderLockCache) StoreScanCursorIfLeader(ctx context.Context, cursorKey, leaderKey, owner string, cursor int64) (bool, error) {
	result, err := scanCursorStoreScript.Run(ctx, c.rdb, []string{
		leaderLockKeyPrefix + leaderKey,
		scanCursorKeyPrefix + cursorKey,
	}, owner, cursor).Int()
	return result == 1, err
}
