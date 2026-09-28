//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type visibilityUserRepo struct {
	UserRepository
	user *User
	err  error
}

func (r *visibilityUserRepo) GetByID(context.Context, int64) (*User, error) { return r.user, r.err }

type visibilitySubRepo struct {
	UserSubscriptionRepository
	subs []UserSubscription
	err  error
}

func (r *visibilitySubRepo) ListActiveByUserID(context.Context, int64) ([]UserSubscription, error) {
	return r.subs, r.err
}

func TestGetUserGroupVisibilityIncludesActiveSubscriptions(t *testing.T) {
	svc := &APIKeyService{
		userRepo:    &visibilityUserRepo{user: &User{ID: 1, AllowedGroups: []int64{7}, RestrictPublicGroups: true}},
		userSubRepo: &visibilitySubRepo{subs: []UserSubscription{{UserID: 1, GroupID: 42, Status: SubscriptionStatusActive}}},
	}
	visible, restricted, err := svc.GetUserGroupVisibility(context.Background(), 1)
	require.NoError(t, err)
	require.True(t, restricted)
	require.Equal(t, map[int64]struct{}{7: {}, 42: {}}, visible)
}

func TestGetUserGroupVisibilityFailsClosedOnSubscriptionError(t *testing.T) {
	failure := errors.New("repository unavailable")
	svc := &APIKeyService{
		userRepo:    &visibilityUserRepo{user: &User{ID: 1}},
		userSubRepo: &visibilitySubRepo{err: failure},
	}
	visible, _, err := svc.GetUserGroupVisibility(context.Background(), 1)
	require.ErrorIs(t, err, failure)
	require.Nil(t, visible)
}
