package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newMiniredisEmailCache(t *testing.T) (service.EmailCache, *miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewEmailCache(rdb), mr, rdb
}

func TestEmailCache_ConcurrentWrongCodesCannotExceedAttemptCap(t *testing.T) {
	cache, _, _ := newMiniredisEmailCache(t)
	ctx := context.Background()
	email := "user@example.com"
	svc := service.NewEmailService(nil, cache)
	require.NoError(t, cache.SetVerificationCode(ctx, email, &service.VerificationCodeData{
		Code: "123456", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Minute),
	}, time.Minute))
	var invalid, maxed atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			switch err := svc.VerifyCode(ctx, email, "000000"); {
			case errors.Is(err, service.ErrInvalidVerifyCode):
				invalid.Add(1)
			case errors.Is(err, service.ErrVerifyCodeMaxAttempts):
				maxed.Add(1)
			default:
				t.Errorf("unexpected result: %v", err)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(4), invalid.Load())
	require.Equal(t, int32(46), maxed.Load())
	require.ErrorIs(t, svc.VerifyCode(ctx, email, "123456"), service.ErrVerifyCodeMaxAttempts)
}

func TestEmailCache_CodeGenerationLifecycle(t *testing.T) {
	for _, notify := range []bool{false, true} {
		t.Run(map[bool]string{false: "auth", true: "notify"}[notify], func(t *testing.T) {
			cache, mr, _ := newMiniredisEmailCache(t)
			set, get, incr, del, key := cache.SetVerificationCode, cache.GetVerificationCode,
				cache.IncrVerificationCodeAttempts, cache.DeleteVerificationCode, verifyCodeKey
			if notify {
				set, get, incr, del, key = cache.SetNotifyVerifyCode, cache.GetNotifyVerifyCode,
					cache.IncrNotifyVerifyCodeAttempts, cache.DeleteNotifyVerifyCode, notifyVerifyKey
			}
			ctx := context.Background()
			email := "User@Example.com"
			old := &service.VerificationCodeData{Code: "123456", CreatedAt: time.Now()}
			require.NoError(t, set(ctx, email, old, time.Minute))
			n, err := incr(ctx, strings.ToLower(email), old, false)
			require.NoError(t, err)
			require.Equal(t, 1, n)
			mr.FastForward(10 * time.Second)
			n, err = incr(ctx, email, old, false)
			require.NoError(t, err)
			require.Equal(t, 2, n)
			require.Equal(t, mr.TTL(key(email)), mr.TTL(key(email)+attemptsKeySuffix))
			// Same code value, different generation: stale attempts and successes cannot mutate it.
			fresh := &service.VerificationCodeData{Code: old.Code, CreatedAt: old.CreatedAt.Add(time.Second)}
			require.NoError(t, set(ctx, email, fresh, time.Minute))
			for _, consume := range []bool{false, true} {
				_, err = incr(ctx, email, old, consume)
				require.ErrorIs(t, err, redis.Nil)
			}
			data, err := get(ctx, email)
			require.NoError(t, err)
			require.Zero(t, data.Attempts)
			n, err = incr(ctx, email, data, true)
			require.NoError(t, err)
			require.Equal(t, 1, n)
			require.False(t, mr.Exists(key(email)))
			require.False(t, mr.Exists(key(email)+attemptsKeySuffix))
			_, err = incr(ctx, email, data, true)
			require.ErrorIs(t, err, redis.Nil)
			require.NoError(t, set(ctx, email, fresh, time.Minute))
			_, err = incr(ctx, email, fresh, false)
			require.NoError(t, err)
			require.NoError(t, del(ctx, email))
			require.False(t, mr.Exists(key(email)+attemptsKeySuffix))
			require.NoError(t, set(ctx, email, fresh, time.Minute))
			mr.FastForward(time.Minute)
			_, err = incr(ctx, email, fresh, true)
			require.ErrorIs(t, err, redis.Nil)
		})
	}
}

func TestEmailCache_CodeSuccessfulConsumptionSingleUse(t *testing.T) {
	cache, _, _ := newMiniredisEmailCache(t)
	ctx := context.Background()
	email := "success@example.com"
	require.NoError(t, cache.SetVerificationCode(ctx, email,
		&service.VerificationCodeData{Code: "123456", CreatedAt: time.Now()}, time.Minute))
	var wins atomic.Int32
	var wg sync.WaitGroup
	svc := service.NewEmailService(nil, cache)
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if svc.VerifyCode(ctx, email, "123456") == nil {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(1), wins.Load())
}

func TestEmailCache_AttemptCounterEvictionPreservesBudget(t *testing.T) {
	cache, mr, _ := newMiniredisEmailCache(t)
	ctx := context.Background()
	email := "evicted@example.com"
	data := &service.VerificationCodeData{Code: "123456", Attempts: 3, CreatedAt: time.Now()}
	require.NoError(t, cache.SetVerificationCode(ctx, email, data, time.Minute))
	// A pre-upgrade record or evicted counter must retain the record's failed attempts.
	mr.Del(verifyCodeKey(email) + attemptsKeySuffix)
	n, err := cache.IncrVerificationCodeAttempts(ctx, email, data, false)
	require.NoError(t, err)
	require.Equal(t, 4, n)
	mr.FastForward(10 * time.Second)
	mr.Del(verifyCodeKey(email) + attemptsKeySuffix)
	svc := service.NewEmailService(nil, cache)
	require.ErrorIs(t, svc.VerifyCode(ctx, email, "wrong"), service.ErrVerifyCodeMaxAttempts)
	require.Equal(t, 50*time.Second, mr.TTL(verifyCodeKey(email)))
	mr.Del(verifyCodeKey(email) + attemptsKeySuffix)
	require.ErrorIs(t, svc.VerifyCode(ctx, email, "123456"), service.ErrVerifyCodeMaxAttempts)
}

func TestEmailCache_PasswordResetTokenHashedAndSingleUse(t *testing.T) {
	cache, mr, _ := newMiniredisEmailCache(t)
	ctx := context.Background()
	email := "reset@example.com"
	svc := service.NewEmailService(nil, cache)
	token, err := svc.GeneratePasswordResetToken()
	require.NoError(t, err)
	sum := sha256.Sum256([]byte(token))
	require.NoError(t, cache.SetPasswordResetToken(ctx, email, &service.PasswordResetTokenData{
		Token: hex.EncodeToString(sum[:]), CreatedAt: time.Now(),
	}, time.Minute))
	raw, err := mr.Get(passwordResetKey(email))
	require.NoError(t, err)
	require.NotContains(t, raw, token)
	require.NoError(t, svc.VerifyPasswordResetToken(ctx, email, token))
	require.ErrorIs(t, svc.ConsumePasswordResetToken(ctx, email, "wrong"), service.ErrInvalidResetToken)
	require.True(t, mr.Exists(passwordResetKey(email)))
	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := svc.ConsumePasswordResetToken(ctx, email, token)
			if err == nil {
				wins.Add(1)
			} else if !errors.Is(err, service.ErrInvalidResetToken) {
				t.Errorf("unexpected result: %v", err)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(1), wins.Load())
	require.False(t, mr.Exists(passwordResetKey(email)))
}

func TestEmailCache_ResetTokenMismatchExpiryAndCorruption(t *testing.T) {
	cache, mr, rdb := newMiniredisEmailCache(t)
	ctx := context.Background()
	email := "keep@example.com"
	require.NoError(t, cache.SetPasswordResetToken(ctx, email, &service.PasswordResetTokenData{Token: "abc"}, time.Minute))
	ok, err := cache.ConsumePasswordResetToken(ctx, email, "xyz")
	require.NoError(t, err)
	require.False(t, ok)
	require.True(t, mr.Exists(passwordResetKey(email)))
	ok, err = cache.ConsumePasswordResetToken(ctx, email, "abc")
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, cache.SetPasswordResetToken(ctx, email, &service.PasswordResetTokenData{Token: "abc"}, time.Minute))
	mr.FastForward(time.Minute)
	ok, err = cache.ConsumePasswordResetToken(ctx, email, "abc")
	require.NoError(t, err)
	require.False(t, ok)
	for _, raw := range []string{"invalid-json", "null", "123", `{"Token":123}`} {
		require.NoError(t, rdb.Set(ctx, passwordResetKey(email), raw, time.Minute).Err())
		ok, err = cache.ConsumePasswordResetToken(ctx, email, "abc")
		require.NoError(t, err)
		require.False(t, ok)
	}
	require.NoError(t, rdb.Close())
	ok, err = cache.ConsumePasswordResetToken(ctx, email, "abc")
	require.Error(t, err)
	require.False(t, ok)
}
