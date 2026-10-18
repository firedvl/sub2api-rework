//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAnthropicAPIOnlyScheduling(t *testing.T) {
	for _, loadAware := range []bool{false, true} {
		for _, sticky := range []bool{false, true} {
			f := newLoadAwareRestrictionFixture(t, false, map[string]int64{}, map[string][]int64{"claude-fable-5-1": {1, 2}})
			repo := f.svc.accountRepo.(*mockAccountRepoForPlatform)
			repo.accounts[0].Type = AccountTypeOAuth
			repo.accounts[1].Type = AccountTypeAPIKey
			repo.accounts[1].Credentials["api_key"] = "synthetic"
			for i := range repo.accounts {
				repo.accounts[i].AccountGroups = []AccountGroup{{AccountID: repo.accounts[i].ID, GroupID: f.groupID}}
			}
			if sticky {
				f.cache.sessionBindings["api-session"] = 1
			}
			ctx := WithAnthropicAPIOnly(f.ctx)
			selectAccount := func(excluded map[int64]struct{}) (*Account, error) {
				if loadAware {
					result, err := f.svc.SelectAccountWithLoadAwareness(ctx, &f.groupID, "api-session", "claude-fable-5-1", excluded, "", 0)
					if err != nil {
						return nil, err
					}
					if result.ReleaseFunc != nil {
						result.ReleaseFunc()
					}
					return result.Account, nil
				}
				return f.svc.SelectAccountForModelWithExclusions(ctx, &f.groupID, "api-session", "claude-fable-5-1", excluded)
			}
			account, err := selectAccount(nil)
			require.NoError(t, err)
			require.Equal(t, int64(2), account.ID, "routed/sticky high-priority OAuth must be ignored")
			account, err = selectAccount(nil)
			require.NoError(t, err)
			require.Equal(t, int64(2), account.ID, "API affinity must be reusable")
			_, err = selectAccount(map[int64]struct{}{2: {}})
			require.ErrorIs(t, err, ErrNoAvailableAccounts, "failover must never change auth mode")
			reset := time.Now().Add(time.Hour)
			repo.accounts[1].RateLimitResetAt = &reset
			_, err = selectAccount(nil)
			require.ErrorIs(t, err, ErrNoAvailableAccounts)
			reset = time.Now().Add(-time.Minute)
			repo.accounts[1].RateLimitResetAt = &reset
			account, err = selectAccount(nil)
			require.NoError(t, err)
			require.Equal(t, int64(2), account.ID)
		}
	}
}

func TestAnthropicAPIOnlyRechecksHydratedAuthentication(t *testing.T) {
	account := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey}
	changed := *account
	changed.Type = AccountTypeOAuth
	svc := &GatewayService{schedulerSnapshot: &SchedulerSnapshotService{cache: &gatewayProfitSnapshotCache{account: &changed}}}
	released := false
	result, err := svc.newSelectionResult(WithAnthropicAPIOnly(t.Context()), account, true, func() { released = true }, nil)
	require.Error(t, err)
	require.Nil(t, result)
	require.True(t, released)
}

func TestAnthropicAPIOnlyRejectsAuthenticationChangeAtFinalAdmission(t *testing.T) {
	for _, platform := range []string{PlatformAnthropic, PlatformAntigravity} {
		selected := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey}
		changed := *selected
		changed.Platform = platform
		changed.Type = AccountTypeOAuth
		svc := &GatewayService{schedulerSnapshot: &SchedulerSnapshotService{cache: &gatewayProfitSnapshotCache{account: &changed}}}
		ctx := context.WithValue(WithAnthropicAPIOnly(t.Context()), openAIProfitControlGateCtxKey{}, &openAIProfitControlGate{groupID: 42, platform: PlatformAnthropic, threshold: 1})
		_, vetoed, reason := svc.GatewayProfitControlVetoLatest(ctx, selected)
		require.True(t, vetoed, "final snapshot refresh cannot escape API authentication policy")
		require.Equal(t, "anthropic_api_auth_policy", reason)
	}
}
