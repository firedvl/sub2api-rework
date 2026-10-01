//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type bulkPostReadFailureRepo struct {
	service.UserSubscriptionRepository
	reads  int
	failAt int
}

func (repo *bulkPostReadFailureRepo) GetByID(ctx context.Context, id int64) (*service.UserSubscription, error) {
	repo.reads++
	if repo.reads == repo.failAt {
		return nil, errors.New("post-write refresh failed")
	}
	return repo.UserSubscriptionRepository.GetByID(ctx, id)
}

func TestSubscriptionBulkActionPostWriteFailureRollsBackBeforeRetry(t *testing.T) {
	for _, action := range []string{"extend", "reset_quota"} {
		t.Run(action, func(t *testing.T) {
			ctx := context.Background()
			user, err := integrationEntClient.User.Create().
				SetEmail(fmt.Sprintf("bulk-%s-%d@example.test", action, time.Now().UnixNano())).
				SetPasswordHash("fixture-hash").SetRole(service.RoleUser).SetStatus(service.StatusActive).Save(ctx)
			require.NoError(t, err)
			group, err := integrationEntClient.Group.Create().SetName("bulk subscription fixture").SetStatus(service.StatusActive).Save(ctx)
			require.NoError(t, err)
			now := time.Now().UTC().Truncate(time.Microsecond)
			subscription, err := integrationEntClient.UserSubscription.Create().
				SetUserID(user.ID).SetGroupID(group.ID).SetStartsAt(now).
				SetExpiresAt(now.AddDate(0, 0, 30)).SetStatus(service.SubscriptionStatusActive).
				SetAssignedAt(now).SetDailyUsageUsd(7).SetWeeklyUsageUsd(11).Save(ctx)
			require.NoError(t, err)
			t.Cleanup(func() {
				cleanupCtx := mixins.SkipSoftDelete(context.Background())
				require.NoError(t, integrationEntClient.UserSubscription.DeleteOneID(subscription.ID).Exec(cleanupCtx))
				require.NoError(t, integrationEntClient.User.DeleteOneID(user.ID).Exec(cleanupCtx))
				require.NoError(t, integrationEntClient.Group.DeleteOneID(group.ID).Exec(cleanupCtx))
			})
			repository := NewUserSubscriptionRepository(integrationEntClient)
			failingRepo := &bulkPostReadFailureRepo{UserSubscriptionRepository: repository, failAt: 1}
			if action == "reset_quota" {
				failingRepo.failAt = 2
			}
			subscriptions := service.NewSubscriptionService(nil, failingRepo, nil, integrationEntClient, nil)
			t.Cleanup(subscriptions.Stop)
			input := &service.BulkSubscriptionActionInput{SubscriptionIDs: []int64{subscription.ID}, Action: action, Days: 7, Daily: true}
			result, err := subscriptions.BulkSubscriptionAction(ctx, input)
			require.NoError(t, err)
			require.Equal(t, 1, result.FailedCount)
			unchanged, err := repository.GetByID(ctx, subscription.ID)
			require.NoError(t, err)
			require.Equal(t, subscription.ExpiresAt, unchanged.ExpiresAt)
			require.Equal(t, float64(7), unchanged.DailyUsageUSD)
			require.Equal(t, float64(11), unchanged.WeeklyUsageUSD)
			failingRepo.failAt, failingRepo.reads = 0, 0
			result, err = subscriptions.BulkSubscriptionAction(ctx, input)
			require.NoError(t, err)
			require.Equal(t, 1, result.SuccessCount)
			changed, err := repository.GetByID(ctx, subscription.ID)
			require.NoError(t, err)
			if action == "extend" {
				require.Equal(t, subscription.ExpiresAt.AddDate(0, 0, 7), changed.ExpiresAt)
			} else {
				require.Zero(t, changed.DailyUsageUSD)
				require.Equal(t, float64(11), changed.WeeklyUsageUSD)
			}
		})
	}
}
