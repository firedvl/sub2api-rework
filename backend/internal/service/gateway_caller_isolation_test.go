package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type gatewayCallerIsolationAccountRepo struct {
	AccountRepository
	byGroup map[int64][]Account
	err     error
}

type gatewayCallerIsolationSchedulerCache struct {
	SchedulerCache
	mu        sync.RWMutex
	snapshots map[SchedulerBucket][]*Account
}

func (c *gatewayCallerIsolationSchedulerCache) GetSnapshot(_ context.Context, bucket SchedulerBucket) ([]*Account, bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	accounts, ok := c.snapshots[bucket]
	return append([]*Account(nil), accounts...), ok, nil
}

func gatewayCallerIsolationV1ModelIDs(models []GatewayCapabilityModel) []string {
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID)
	}
	return ids
}

func (r *gatewayCallerIsolationAccountRepo) ListModelAvailabilityCandidates(_ context.Context, groupID *int64, _ []string, _ bool) ([]Account, error) {
	if r.err != nil {
		return nil, r.err
	}
	if groupID == nil {
		return nil, nil
	}
	return append([]Account(nil), r.byGroup[*groupID]...), nil
}

func TestGatewayCapabilitiesV1DoesNotPublishOrCountStalePreviousGroupSnapshot(t *testing.T) {
	callerA := &Group{ID: 42, Platform: PlatformAnthropic}
	callerB := &Group{ID: 43, Platform: PlatformAnthropic}
	remainingA := gatewayCapabilityTestAccount(1, PlatformAnthropic, map[string]string{
		"caller-a-private": "a-private-upstream",
		"shared-model":     "a-shared-upstream",
	})
	remainingA.Extra = map[string]any{"quota_limit": 100.0, "quota_used": 50.0}
	// Account 2 was moved from A to B. Its stale A scheduler entry has already
	// been refreshed with B's durable mappings, which must still not affect A.
	movedToB := gatewayCapabilityTestAccount(2, PlatformAnthropic, map[string]string{
		"caller-b-private": "b-private-upstream",
		"shared-model":     "b-shared-upstream",
	})
	repo := &gatewayCallerIsolationAccountRepo{byGroup: map[int64][]Account{
		callerA.ID: {remainingA},
		callerB.ID: {movedToB},
	}}
	gateway := &GatewayService{
		accountRepo:       repo,
		schedulerSnapshot: gatewayCapabilitySchedulerSnapshot(callerA.ID, PlatformAnthropic, &remainingA, &movedToB),
	}

	callerAModels := gateway.BuildGatewayCapabilityModels(context.Background(), callerA, nil)
	require.NotContains(t, gatewayCallerIsolationV1ModelIDs(callerAModels), "caller-b-private")
	shared := gatewayCapabilityModelByID(t, callerAModels, "shared-model")
	require.NotNil(t, shared.Routing.CandidatePaths)
	require.Equal(t, 1, *shared.Routing.CandidatePaths, "moved account must not inflate A's shared route")
	require.Equal(t, GatewayCapacityKnown, shared.Capacity.Status)
	require.Equal(t, 50.0, *shared.Capacity.LimitingRemainingPercent, "capacity must use only A's durable account")

	callerBModels := gateway.BuildGatewayCapabilityModels(context.Background(), callerB, nil)
	require.Contains(t, gatewayCallerIsolationV1ModelIDs(callerBModels), "caller-b-private", "same account's new durable mapping must publish for B")

	repo.err = errors.New("configured account read failed")
	require.Empty(t, gateway.BuildGatewayCapabilityModels(context.Background(), callerA, nil), "a stale snapshot cannot substitute for unknown durable policy")

	routes := &gatewayCapabilityRouteRepoStub{routes: []CompositeModelRoute{{
		ID: 1, Enabled: true, MatchType: CompositeRouteMatchExact, Endpoint: CompositeRouteEndpointResponses,
		PublicModel: "explicit-model", TargetPlatform: PlatformAnthropic, UpstreamModel: "a-shared-upstream",
	}}}
	composite := &GatewayService{
		accountRepo:       repo,
		compositeResolver: NewCompositeRouteResolver(routes),
		schedulerSnapshot: gatewayCapabilitySchedulerSnapshot(callerA.ID, PlatformAnthropic, &remainingA),
	}
	unknown := gatewayCapabilityModelByID(t, composite.BuildGatewayCapabilityModels(context.Background(), &Group{ID: callerA.ID, Platform: PlatformComposite}, nil), "explicit-model")
	require.Equal(t, GatewayAvailabilityUnknown, unknown.Availability)
	require.Nil(t, unknown.Routing.Routable)
	require.Nil(t, unknown.Routing.CandidatePaths)
	require.Equal(t, GatewayCapacityUnknown, unknown.Capacity.Status)
}

func TestGatewayCapabilitiesV1SameAccountMappingChurnUsesDurablePublication(t *testing.T) {
	group := &Group{ID: 42, Platform: PlatformAnthropic}
	durable := gatewayCapabilityTestAccount(1, PlatformAnthropic, map[string]string{"added-model": "added-upstream"})
	stale := gatewayCapabilityTestAccount(1, PlatformAnthropic, map[string]string{"removed-model": "removed-upstream"})
	gateway := &GatewayService{
		accountRepo:       &gatewayCallerIsolationAccountRepo{byGroup: map[int64][]Account{group.ID: {durable}}},
		schedulerSnapshot: gatewayCapabilitySchedulerSnapshot(group.ID, PlatformAnthropic, &stale),
	}

	models := gateway.BuildGatewayCapabilityModels(context.Background(), group, nil)
	require.NotContains(t, gatewayCallerIsolationV1ModelIDs(models), "removed-model", "stale same-ID mapping must not publish a removed model")
	added := gatewayCapabilityModelByID(t, models, "added-model")
	require.Equal(t, GatewayAvailabilityUnavailable, added.Availability, "the active snapshot does not yet support the new mapping")
	require.NotNil(t, added.Routing.CandidatePaths)
	require.Equal(t, 0, *added.Routing.CandidatePaths, "stale same-ID account must not supply capacity for a new mapping")
}

func TestGatewayEffectiveCapabilitiesConcurrentSnapshotIsolation(t *testing.T) {
	groupA := &Group{ID: 42, Platform: PlatformAnthropic, ModelAllowlist: GroupModelAllowlist{Enabled: true, Models: []string{"caller-a-model"}}}
	groupB := &Group{ID: 43, Platform: PlatformAnthropic, ModelAllowlist: GroupModelAllowlist{Enabled: true, Models: []string{"caller-b-model"}}}
	until := time.Now().Add(time.Hour)
	accountA := gatewayEffectiveTestAccount(1, PlatformAnthropic, "caller-a-model", "a-upstream")
	accountA.RateLimitResetAt = &until
	accountB := gatewayEffectiveTestAccount(2, PlatformAnthropic, "caller-b-model", "b-upstream")
	cache := &gatewayCallerIsolationSchedulerCache{snapshots: map[SchedulerBucket][]*Account{
		{GroupID: groupA.ID, Platform: PlatformAnthropic, Mode: SchedulerModeMixed}: {&accountA},
		{GroupID: groupB.ID, Platform: PlatformAnthropic, Mode: SchedulerModeMixed}: {&accountB},
	}}
	gateway := &GatewayService{
		accountRepo: &gatewayCallerIsolationAccountRepo{byGroup: map[int64][]Account{
			groupA.ID: {accountA},
			groupB.ID: {accountB},
		}},
		schedulerSnapshot: &SchedulerSnapshotService{cache: cache},
	}

	type expected struct {
		group        *Group
		model, state string
	}
	start := make(chan struct{})
	errs := make(chan error, 64)
	var wait sync.WaitGroup
	for _, tc := range []expected{
		{group: groupA, model: "caller-a-model", state: "temporarily_unavailable"},
		{group: groupB, model: "caller-b-model", state: "available"},
	} {
		wait.Add(1)
		go func(tc expected) {
			defer wait.Done()
			<-start
			for i := 0; i < 32; i++ {
				result := gateway.BuildGatewayEffectiveCapabilities(context.Background(), tc.group, nil)
				if len(result.Models) != 1 || result.Models[0].ID != tc.model {
					errs <- fmt.Errorf("group %d catalog: got %#v", tc.group.ID, result.Models)
					return
				}
				if got := result.Models[0].Protocols[CompositeRouteEndpointResponses].Availability.State; got != tc.state {
					errs <- fmt.Errorf("group %d availability: got %q, want %q", tc.group.ID, got, tc.state)
					return
				}
				preflight, err := gateway.PreflightGatewayRequest(context.Background(), tc.group, GatewayPreflightRequest{SchemaVersion: 2, Model: tc.model, Protocol: CompositeRouteEndpointResponses}, nil)
				if err != nil || preflight.Availability.State != tc.state {
					errs <- fmt.Errorf("group %d preflight: state %q err %v", tc.group.ID, preflight.Availability.State, err)
					return
				}
			}
		}(tc)
	}
	close(start)
	wait.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
}
