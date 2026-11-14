package service

import (
	"context"
	"testing"
	"time"

	gocache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/require"
)

type singleAntigravityAccountRepoStub struct {
	AccountRepository
	accounts []Account
	calls    int
}

func (r *singleAntigravityAccountRepoStub) ListSchedulableByGroupIDAndPlatform(_ context.Context, _ int64, _ string) ([]Account, error) {
	r.calls++
	return r.accounts, nil
}

func TestIsSingleAntigravityAccountGroupCachesEmptyResult(t *testing.T) {
	repo := &singleAntigravityAccountRepoStub{}
	svc := &GatewayService{accountRepo: repo, noAntigravityGroups: gocache.New(noAntigravityGroupTTL, time.Minute)}
	groupID := int64(7)
	otherGroupID := int64(8)

	require.False(t, svc.IsSingleAntigravityAccountGroup(context.Background(), &groupID))
	require.False(t, svc.IsSingleAntigravityAccountGroup(context.Background(), &groupID))
	require.Equal(t, 1, repo.calls, "empty result must be served from cache")

	require.False(t, svc.IsSingleAntigravityAccountGroup(context.Background(), &otherGroupID))
	require.Equal(t, 2, repo.calls, "cache is keyed by group")
}

func TestIsSingleAntigravityAccountGroupSeesNewAccountAfterExpiry(t *testing.T) {
	repo := &singleAntigravityAccountRepoStub{}
	svc := &GatewayService{accountRepo: repo, noAntigravityGroups: gocache.New(noAntigravityGroupTTL, time.Minute)}
	groupID := int64(7)

	require.False(t, svc.IsSingleAntigravityAccountGroup(context.Background(), &groupID))

	repo.accounts = []Account{{ID: 1, Platform: PlatformAntigravity}}
	require.False(t, svc.IsSingleAntigravityAccountGroup(context.Background(), &groupID), "still cached within TTL")

	// 等价于 TTL 到期：空结果条目被清除后必须重新查询。
	svc.noAntigravityGroups.Flush()
	require.True(t, svc.IsSingleAntigravityAccountGroup(context.Background(), &groupID))

	// 非空结果不缓存，继续由调度快照负责。
	require.True(t, svc.IsSingleAntigravityAccountGroup(context.Background(), &groupID))
	require.Equal(t, 3, repo.calls)
}
