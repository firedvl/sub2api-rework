package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRedeemReductionUsesLockedCurrentSubscription(t *testing.T) {
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	stale := UserSubscription{ID: 7, ExpiresAt: now.AddDate(0, 0, 10), Status: SubscriptionStatusActive, Notes: "old"}
	current := stale
	current.ExpiresAt = now.AddDate(0, 0, 20)
	current.Notes = "renewed"
	repo := &lockingRenewalRepo{stale: stale, current: current}
	subscriptions := NewSubscriptionService(nil, repo, nil, nil, nil)
	subscriptions.now = func() time.Time { return now }
	svc := &RedeemService{subscriptionService: subscriptions}
	for range 2 {
		require.NoError(t, svc.reduceOrCancelSubscription(context.Background(), 11, 13, 1, "deduct"))
	}
	require.Equal(t, 2, repo.lockReads)
	require.Equal(t, current.ExpiresAt.AddDate(0, 0, -2), repo.current.ExpiresAt)
	require.Contains(t, repo.current.Notes, "renewed")
}

type failingReductionLockRepo struct {
	*lockingRenewalRepo
	err error
}

func (r *failingReductionLockRepo) GetByIDForUpdate(context.Context, int64) (*UserSubscription, error) {
	return nil, r.err
}

func TestRedeemReductionLockFailureDoesNotWrite(t *testing.T) {
	sub := UserSubscription{ID: 7, ExpiresAt: time.Now().AddDate(0, 0, 10), Status: SubscriptionStatusActive, Notes: "unchanged"}
	repo := &failingReductionLockRepo{lockingRenewalRepo: &lockingRenewalRepo{stale: sub, current: sub}, err: errors.New("lock failed")}
	svc := &RedeemService{subscriptionService: NewSubscriptionService(nil, repo, nil, nil, nil)}
	require.ErrorIs(t, svc.reduceOrCancelSubscription(context.Background(), 11, 13, 1, "deduct"), repo.err)
	require.Equal(t, sub, repo.current)
}

func TestRedeemReductionPreservesRemainingTime(t *testing.T) {
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name      string
		remaining time.Duration
		days      int
		expired   bool
	}{
		{"partial day", 36 * time.Hour, 1, false},
		{"one second left", 24*time.Hour + time.Second, 1, false},
		{"exact exhaustion", 24 * time.Hour, 1, true},
		{"excess deduction", 12 * time.Hour, 1, true},
		{"already expired", -time.Hour, 1, true},
		{"multiple days", 84 * time.Hour, 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sub := UserSubscription{ID: 7, ExpiresAt: now.Add(tc.remaining), Status: SubscriptionStatusActive, Notes: "original"}
			repo := &lockingRenewalRepo{stale: sub, current: sub}
			subscriptions := NewSubscriptionService(nil, repo, nil, nil, nil)
			subscriptions.now = func() time.Time { return now }
			svc := &RedeemService{subscriptionService: subscriptions}
			require.NoError(t, svc.reduceOrCancelSubscription(context.Background(), 11, 13, tc.days, "deduct"))
			want, status := sub.ExpiresAt.AddDate(0, 0, -tc.days), SubscriptionStatusActive
			if tc.expired {
				want, status = now, SubscriptionStatusExpired
			}
			require.Equal(t, want, repo.current.ExpiresAt)
			require.Equal(t, status, repo.current.Status)
			require.Contains(t, repo.current.Notes, "original")
		})
	}
}

func TestExtendSubscriptionUsesLockedCurrentExpiry(t *testing.T) {
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	stale := UserSubscription{ID: 7, UserID: 11, GroupID: 13, ExpiresAt: now.AddDate(0, 0, 10), Status: SubscriptionStatusActive}
	current := stale
	current.ExpiresAt = now.AddDate(0, 0, 9)
	repo := &lockingRenewalRepo{stale: stale, current: current}
	svc := NewSubscriptionService(nil, repo, nil, nil, nil)
	svc.now = func() time.Time { return now }

	updated, err := svc.ExtendSubscription(context.Background(), 7, 2)
	require.NoError(t, err)
	require.Equal(t, 1, repo.lockReads)
	require.Equal(t, current.ExpiresAt.AddDate(0, 0, 2), updated.ExpiresAt)
}
