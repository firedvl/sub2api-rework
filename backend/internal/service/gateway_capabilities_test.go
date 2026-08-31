package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

type gatewayCapabilityAccountRepoStub struct {
	AccountRepository
	current         []Account
	configured      []Account
	currentErr      error
	configuredErr   error
	currentCalls    int
	configuredCalls int
}

func (r *gatewayCapabilityAccountRepoStub) ListSchedulableByGroupIDAndPlatform(context.Context, int64, string) ([]Account, error) {
	r.currentCalls++
	return r.current, r.currentErr
}

func (r *gatewayCapabilityAccountRepoStub) ListSchedulableByPlatform(context.Context, string) ([]Account, error) {
	r.currentCalls++
	return r.current, r.currentErr
}

func (r *gatewayCapabilityAccountRepoStub) ListSchedulableUngroupedByPlatform(context.Context, string) ([]Account, error) {
	r.currentCalls++
	return r.current, r.currentErr
}

type gatewayCapabilitySchedulerCacheStub struct {
	SchedulerCache
	snapshots    map[SchedulerBucket][]*Account
	errors       map[SchedulerBucket]error
	getCalls     map[SchedulerBucket]int
	captureCalls int
	setCalls     int
}

func (c *gatewayCapabilitySchedulerCacheStub) GetSnapshot(_ context.Context, bucket SchedulerBucket) ([]*Account, bool, error) {
	if c.getCalls == nil {
		c.getCalls = make(map[SchedulerBucket]int)
	}
	c.getCalls[bucket]++
	if err := c.errors[bucket]; err != nil {
		return nil, false, err
	}
	accounts, hit := c.snapshots[bucket]
	return accounts, hit, nil
}

func (c *gatewayCapabilitySchedulerCacheStub) CaptureBucketWriteToken(_ context.Context, bucket SchedulerBucket) (SchedulerBucketWriteToken, error) {
	c.captureCalls++
	return SchedulerBucketWriteToken{Bucket: bucket, Epoch: 1}, nil
}

func (c *gatewayCapabilitySchedulerCacheStub) SetSnapshot(_ context.Context, _ SchedulerBucket, _ SchedulerBucketWriteToken, _ []Account) error {
	c.setCalls++
	return nil
}

func (r *gatewayCapabilityAccountRepoStub) ListSchedulable(context.Context) ([]Account, error) {
	r.currentCalls++
	return r.current, r.currentErr
}

func (r *gatewayCapabilityAccountRepoStub) ListSchedulableByGroupID(context.Context, int64) ([]Account, error) {
	r.currentCalls++
	return r.current, r.currentErr
}

func (r *gatewayCapabilityAccountRepoStub) ListModelAvailabilityCandidates(context.Context, *int64, []string, bool) ([]Account, error) {
	r.configuredCalls++
	return r.configured, r.configuredErr
}

func (r *gatewayCapabilityAccountRepoStub) ListSchedulableUngroupedByPlatforms(context.Context, []string) ([]Account, error) {
	r.currentCalls++
	return r.current, r.currentErr
}

type gatewayCapabilityRouteRepoStub struct {
	CompositeModelRouteRepository
	routes []CompositeModelRoute
	err    error
	calls  int
}

type gatewayCapabilityUsageRepoStub struct {
	UsageLogRepository
	calls atomic.Int64
}

func (r *gatewayCapabilityUsageRepoStub) GetAccountWindowStatsBatch(context.Context, []int64, time.Time) (map[int64]*usagestats.AccountStats, error) {
	r.calls.Add(1)
	return nil, nil
}

func (r *gatewayCapabilityRouteRepoStub) ListByGroup(context.Context, int64, bool) ([]CompositeModelRoute, error) {
	r.calls++
	return r.routes, r.err
}

func gatewayCapabilityTestAccount(id int64, platform string, mapping map[string]string) Account {
	rawMapping := make(map[string]any, len(mapping))
	for publicModel, upstreamModel := range mapping {
		rawMapping[publicModel] = upstreamModel
	}
	return Account{
		ID:          id,
		Name:        "synthetic account",
		Platform:    platform,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"model_mapping": rawMapping},
		Extra:       map[string]any{},
		Status:      StatusActive,
		Schedulable: true,
	}
}

func gatewayCapabilityModelByID(t *testing.T, models []GatewayCapabilityModel, id string) GatewayCapabilityModel {
	t.Helper()
	for _, model := range models {
		if model.ID == id {
			return model
		}
	}
	t.Fatalf("model %q not found in %#v", id, models)
	return GatewayCapabilityModel{}
}

func gatewayCapabilitySchedulerSnapshot(groupID int64, platform string, accounts ...*Account) *SchedulerSnapshotService {
	mode := SchedulerModeSingle
	if platform == PlatformAnthropic || platform == PlatformGemini {
		mode = SchedulerModeMixed
	}
	bucket := SchedulerBucket{GroupID: groupID, Platform: platform, Mode: mode}
	return &SchedulerSnapshotService{cache: &gatewayCapabilitySchedulerCacheStub{
		snapshots: map[SchedulerBucket][]*Account{bucket: accounts},
	}}
}

func TestBuildGatewayCapabilityModelsStatesRoutesAndQueries(t *testing.T) {
	t.Run("available degraded alias and deterministic scope", func(t *testing.T) {
		current := gatewayCapabilityTestAccount(1, PlatformOpenAI, map[string]string{
			"model-z":      "model-z",
			"public-alias": "upstream-model",
		})
		current.Extra = map[string]any{"quota_limit": 100.0, "quota_used": 20.0}
		configuredExtra := gatewayCapabilityTestAccount(2, PlatformOpenAI, map[string]string{"model-z": "model-z"})
		repo := &gatewayCapabilityAccountRepoStub{
			current:    []Account{current},
			configured: []Account{current, configuredExtra},
		}
		service := &GatewayService{
			accountRepo:       repo,
			schedulerSnapshot: gatewayCapabilitySchedulerSnapshot(42, PlatformOpenAI, &current),
		}
		group := &Group{
			ID:       42,
			Platform: PlatformOpenAI,
			ModelsListConfig: GroupModelsListConfig{
				Enabled: true,
				Models:  []string{"model-z", "hidden-model", "public-alias"},
			},
		}

		models := service.BuildGatewayCapabilityModels(context.Background(), group, nil)

		require.Equal(t, []string{"model-z", "public-alias"}, []string{models[0].ID, models[1].ID})
		alias := gatewayCapabilityModelByID(t, models, "public-alias")
		require.Equal(t, GatewayAvailabilityAvailable, alias.Availability)
		require.Equal(t, GatewayRouteAlias, alias.Routing.Type)
		require.NotNil(t, alias.Routing.Routable)
		require.True(t, *alias.Routing.Routable)
		require.Equal(t, 1, *alias.Routing.CandidatePaths)
		require.Equal(t, GatewayCapacityKnown, alias.Capacity.Status)

		degraded := gatewayCapabilityModelByID(t, models, "model-z")
		require.Equal(t, GatewayAvailabilityDegraded, degraded.Availability)
		require.Equal(t, GatewayRouteDirect, degraded.Routing.Type)
		require.NotContains(t, []string{models[0].ID, models[1].ID}, "hidden-model")
		require.Zero(t, repo.currentCalls)
		require.Equal(t, 1, repo.configuredCalls)
	})

	t.Run("unavailable", func(t *testing.T) {
		configured := gatewayCapabilityTestAccount(3, PlatformOpenAI, map[string]string{"offline-model": "offline-model"})
		repo := &gatewayCapabilityAccountRepoStub{configured: []Account{configured}}
		models := (&GatewayService{
			accountRepo:       repo,
			schedulerSnapshot: gatewayCapabilitySchedulerSnapshot(42, PlatformOpenAI),
		}).BuildGatewayCapabilityModels(
			context.Background(),
			&Group{ID: 42, Platform: PlatformOpenAI},
			nil,
		)

		require.Len(t, models, 1)
		require.Equal(t, GatewayAvailabilityUnavailable, models[0].Availability)
		require.NotNil(t, models[0].Routing.Routable)
		require.False(t, *models[0].Routing.Routable)
		require.Equal(t, 0, *models[0].Routing.CandidatePaths)
		require.Equal(t, GatewayCapacityUnknown, models[0].Capacity.Status)
	})

	t.Run("ungrouped standard mode uses only ungrouped accounts", func(t *testing.T) {
		grouped := gatewayCapabilityTestAccount(5, PlatformAnthropic, map[string]string{"group-only-model": "group-only-model"})
		ungrouped := gatewayCapabilityTestAccount(6, PlatformAnthropic, map[string]string{"ungrouped-model": "ungrouped-model"})
		repo := &gatewayCapabilityAccountRepoStub{
			current:    []Account{grouped},
			configured: []Account{ungrouped},
		}
		service := &GatewayService{
			accountRepo:       repo,
			cfg:               &config.Config{RunMode: config.RunModeStandard},
			schedulerSnapshot: gatewayCapabilitySchedulerSnapshot(0, PlatformAnthropic, &ungrouped),
		}

		models := service.BuildGatewayCapabilityModels(context.Background(), nil, nil)

		require.Len(t, models, 1)
		require.Equal(t, "ungrouped-model", models[0].ID)
		require.Zero(t, repo.currentCalls)
		require.NotContains(t, []string{models[0].ID}, "group-only-model")
	})

	t.Run("nil snapshot stays unknown without database fallback", func(t *testing.T) {
		account := gatewayCapabilityTestAccount(7, PlatformOpenAI, map[string]string{"unknown-model": "unknown-model"})
		repo := &gatewayCapabilityAccountRepoStub{
			current:    []Account{account},
			configured: []Account{account},
		}
		models := (&GatewayService{accountRepo: repo}).BuildGatewayCapabilityModels(
			context.Background(),
			&Group{ID: 42, Platform: PlatformOpenAI},
			map[string][]string{PlatformOpenAI: {"unknown-model"}},
		)

		require.Len(t, models, 1)
		require.Equal(t, GatewayAvailabilityUnknown, models[0].Availability)
		require.Nil(t, models[0].Routing.Routable)
		require.Nil(t, models[0].Routing.CandidatePaths)
		require.Equal(t, GatewayCapacityUnknown, models[0].Capacity.Status)
		require.Zero(t, repo.currentCalls)
	})

	t.Run("composite route and route-query failure", func(t *testing.T) {
		account := gatewayCapabilityTestAccount(4, PlatformOpenAI, map[string]string{"gpt-route": "gpt-route"})
		accountRepo := &gatewayCapabilityAccountRepoStub{current: []Account{account}, configured: []Account{account}}
		routeRepo := &gatewayCapabilityRouteRepoStub{routes: []CompositeModelRoute{{
			PublicModel:    "gpt-route",
			MatchType:      CompositeRouteMatchExact,
			TargetPlatform: PlatformOpenAI,
			UpstreamModel:  "gpt-route",
			Endpoint:       CompositeRouteEndpointResponses,
			Enabled:        true,
		}}}
		service := &GatewayService{
			accountRepo:       accountRepo,
			schedulerSnapshot: gatewayCapabilitySchedulerSnapshot(77, PlatformOpenAI, &account),
			compositeResolver: NewCompositeRouteResolver(routeRepo),
		}
		group := &Group{ID: 77, Platform: PlatformComposite}

		models := service.BuildGatewayCapabilityModels(context.Background(), group, nil)

		require.Len(t, models, 1)
		require.Equal(t, GatewayRouteComposite, models[0].Routing.Type)
		require.Equal(t, GatewayAvailabilityAvailable, models[0].Availability)
		require.Equal(t, 1, routeRepo.calls)
		require.Zero(t, accountRepo.currentCalls)
		require.Equal(t, 1, accountRepo.configuredCalls)

		routeRepo.err = errors.New("synthetic route failure")
		models = service.BuildGatewayCapabilityModels(context.Background(), group, nil)
		require.Len(t, models, 1)
		require.Equal(t, GatewayRouteUnknown, models[0].Routing.Type)
		require.Equal(t, GatewayAvailabilityUnknown, models[0].Availability)
		require.Nil(t, models[0].Routing.Routable)
	})

	t.Run("composite exact alias is discoverable and scope-filtered", func(t *testing.T) {
		account := gatewayCapabilityTestAccount(5, PlatformOpenAI, map[string]string{"gpt-upstream": "gpt-upstream"})
		accountRepo := &gatewayCapabilityAccountRepoStub{current: []Account{account}, configured: []Account{account}}
		routeRepo := &gatewayCapabilityRouteRepoStub{routes: []CompositeModelRoute{
			{
				PublicModel:    "all/gpt",
				MatchType:      CompositeRouteMatchExact,
				TargetPlatform: PlatformOpenAI,
				UpstreamModel:  "gpt-upstream",
				Endpoint:       CompositeRouteEndpointResponses,
				Enabled:        true,
			},
			{
				PublicModel:    "prefix/",
				MatchType:      CompositeRouteMatchPrefix,
				TargetPlatform: PlatformOpenAI,
				Endpoint:       CompositeRouteEndpointResponses,
				Enabled:        true,
			},
		}}
		service := &GatewayService{
			accountRepo:       accountRepo,
			schedulerSnapshot: gatewayCapabilitySchedulerSnapshot(77, PlatformOpenAI, &account),
			compositeResolver: NewCompositeRouteResolver(routeRepo),
		}
		group := &Group{
			ID:       77,
			Platform: PlatformComposite,
			ModelsListConfig: GroupModelsListConfig{
				Enabled: true,
				Models:  []string{"all/gpt", "hidden-alias", "prefix/"},
			},
		}

		models := service.BuildGatewayCapabilityModels(context.Background(), group, nil)

		require.Len(t, models, 1)
		require.Equal(t, "all/gpt", models[0].ID)
		require.Equal(t, GatewayRouteComposite, models[0].Routing.Type)
		require.Equal(t, GatewayAvailabilityAvailable, models[0].Availability)
		require.NotContains(t, []string{models[0].ID}, "hidden-alias")
		require.NotContains(t, []string{models[0].ID}, "prefix/")
	})
}

func TestGatewayCapabilityCapacityWindowsAndAdvisorySnapshotTruth(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	best := gatewayCapabilityTestAccount(1, PlatformOpenAI, map[string]string{"gpt-test": "gpt-test"})
	best.Extra = map[string]any{
		"quota_limit":       100.0,
		"quota_used":        20.0,
		"quota_daily_limit": 100.0,
		"quota_daily_used":  50.0,
		"quota_daily_start": now.Add(-time.Hour).Format(time.RFC3339),
	}
	other := gatewayCapabilityTestAccount(2, PlatformOpenAI, map[string]string{"gpt-test": "gpt-test"})
	other.Extra = map[string]any{"quota_limit": 100.0, "quota_used": 60.0}

	capacity := gatewayCapabilityCapacity([]Account{best, other}, "gpt-test", now)
	require.Equal(t, GatewayCapacityKnown, capacity.Status)
	require.Equal(t, 50.0, *capacity.LimitingRemainingPercent)
	require.Equal(t, []string{"daily", "total"}, []string{capacity.Windows[0].Window, capacity.Windows[1].Window})

	unknown := gatewayCapabilityTestAccount(3, PlatformOpenAI, map[string]string{"gpt-test": "gpt-test"})
	capacity = gatewayCapabilityCapacity([]Account{best, unknown}, "gpt-test", now)
	require.Equal(t, GatewayCapacityPartial, capacity.Status)
	require.Equal(t, 50.0, *capacity.LimitingRemainingPercent)
	require.Equal(t, GatewayCapacityUnknown, gatewayCapabilityCapacity([]Account{unknown}, "gpt-test", now).Status)

	zeroCapacity := gatewayCapabilityTestAccount(4, PlatformOpenAI, map[string]string{"gpt-zero": "gpt-zero"})
	zeroCapacity.Type = AccountTypeOAuth
	zeroCapacity.Extra = map[string]any{
		"codex_5h_used_percent": 100.0,
		"codex_5h_reset_at":     time.Now().Add(time.Hour).Format(time.RFC3339),
	}
	repo := &gatewayCapabilityAccountRepoStub{current: []Account{zeroCapacity}, configured: []Account{zeroCapacity}}
	models := (&GatewayService{
		accountRepo:       repo,
		schedulerSnapshot: gatewayCapabilitySchedulerSnapshot(42, PlatformOpenAI, &zeroCapacity),
	}).BuildGatewayCapabilityModels(
		context.Background(),
		&Group{ID: 42, Platform: PlatformOpenAI},
		nil,
	)
	require.Len(t, models, 1)
	require.Equal(t, GatewayAvailabilityAvailable, models[0].Availability)
	require.True(t, *models[0].Routing.Routable)
	require.Equal(t, 1, *models[0].Routing.CandidatePaths)
	require.Equal(t, 0.0, *models[0].Capacity.LimitingRemainingPercent)
}

func TestGatewayCapabilityCurrentPoolHonorsPassiveSchedulerGates(t *testing.T) {
	t.Run("scheduling threshold", func(t *testing.T) {
		account := gatewayCapabilityTestAccount(923336, PlatformOpenAI, map[string]string{"threshold-model": "threshold-model"})
		account.Credentials[accountSchedulingThresholdCredentialKey] = 80
		account.Extra = map[string]any{
			"codex_usage_updated_at": time.Now().Add(-time.Minute).Format(time.RFC3339),
			"codex_5h_used_percent":  90.0,
			"codex_5h_reset_at":      time.Now().Add(time.Hour).Format(time.RFC3339),
		}
		repo := &gatewayCapabilityAccountRepoStub{current: []Account{account}, configured: []Account{account}}
		gateway := &GatewayService{
			accountRepo:       repo,
			schedulerSnapshot: gatewayCapabilitySchedulerSnapshot(42, PlatformOpenAI, &account),
			rateLimitService: &RateLimitService{
				accountRepo:    repo,
				settingService: NewSettingService(nil, &config.Config{}),
			},
		}

		models := gateway.BuildGatewayCapabilityModels(context.Background(), &Group{ID: 42, Platform: PlatformOpenAI}, nil)

		require.Len(t, models, 1)
		require.Equal(t, GatewayAvailabilityUnavailable, models[0].Availability)
	})

	t.Run("cached Grok free-quota gate without refresh", func(t *testing.T) {
		const accountID = int64(923337)
		account := gatewayCapabilityTestAccount(accountID, PlatformGrok, map[string]string{"grok-gated-model": "grok-gated-model"})
		account.Type = AccountTypeOAuth
		account.Credentials["subscription_tier"] = "free"
		repo := &gatewayCapabilityAccountRepoStub{current: []Account{account}, configured: []Account{account}}
		usageRepo := &gatewayCapabilityUsageRepoStub{}
		cfg := &config.Config{}
		cfg.Gateway.Grok.FreeQuotaSoftGateEnabled = true
		cfg.Gateway.Grok.FreeQuotaTokenLimit = 500_000
		cfg.Gateway.Grok.FreeQuotaSoftGatePercent = 95
		cfg.Gateway.Grok.FreeQuotaWindowHours = 24
		cfg.Gateway.Grok.FreeQuotaStatsCacheSeconds = 60
		gatewayGrokFreeQuotaGateCache.Store(accountID, grokFreeQuotaGateCacheEntry{
			tokens: 475_000, checkedAt: time.Now(), known: true,
		})
		defer gatewayGrokFreeQuotaGateCache.Delete(accountID)

		models := (&GatewayService{
			accountRepo:       repo,
			usageLogRepo:      usageRepo,
			cfg:               cfg,
			schedulerSnapshot: gatewayCapabilitySchedulerSnapshot(42, PlatformGrok, &account),
		}).BuildGatewayCapabilityModels(
			context.Background(),
			&Group{ID: 42, Platform: PlatformGrok},
			nil,
		)

		require.Len(t, models, 1)
		require.Equal(t, GatewayAvailabilityUnavailable, models[0].Availability)
		require.Zero(t, usageRepo.calls.Load())
	})
}

func TestGatewayCapabilityCurrentPoolUsesSchedulerSnapshotPassively(t *testing.T) {
	t.Run("snapshot wins over database candidates", func(t *testing.T) {
		snapshotAccount := gatewayCapabilityTestAccount(923338, PlatformOpenAI, map[string]string{"snapshot-model": "snapshot-model"})
		databaseAccount := gatewayCapabilityTestAccount(923339, PlatformOpenAI, map[string]string{"database-model": "database-model"})
		repo := &gatewayCapabilityAccountRepoStub{
			current:    []Account{databaseAccount},
			configured: []Account{snapshotAccount, databaseAccount},
		}
		bucket := SchedulerBucket{GroupID: 42, Platform: PlatformOpenAI, Mode: SchedulerModeSingle}
		cache := &gatewayCapabilitySchedulerCacheStub{snapshots: map[SchedulerBucket][]*Account{
			bucket: {&snapshotAccount},
		}}
		gateway := &GatewayService{
			accountRepo:       repo,
			schedulerSnapshot: &SchedulerSnapshotService{cache: cache},
		}

		models := gateway.BuildGatewayCapabilityModels(context.Background(), &Group{ID: 42, Platform: PlatformOpenAI}, nil)

		require.Equal(t, GatewayAvailabilityAvailable, gatewayCapabilityModelByID(t, models, "snapshot-model").Availability)
		require.Equal(t, GatewayAvailabilityUnavailable, gatewayCapabilityModelByID(t, models, "database-model").Availability)
		require.Zero(t, repo.currentCalls)
		require.Equal(t, 1, cache.getCalls[bucket])
		require.Zero(t, cache.captureCalls)
		require.Zero(t, cache.setCalls)
	})

	t.Run("snapshot miss stays unknown without fallback", func(t *testing.T) {
		account := gatewayCapabilityTestAccount(923340, PlatformOpenAI, map[string]string{"unknown-model": "unknown-model"})
		repo := &gatewayCapabilityAccountRepoStub{current: []Account{account}, configured: []Account{account}}
		bucket := SchedulerBucket{GroupID: 42, Platform: PlatformOpenAI, Mode: SchedulerModeSingle}
		cache := &gatewayCapabilitySchedulerCacheStub{snapshots: map[SchedulerBucket][]*Account{}}
		gateway := &GatewayService{
			accountRepo:       repo,
			schedulerSnapshot: &SchedulerSnapshotService{cache: cache},
		}

		models := gateway.BuildGatewayCapabilityModels(context.Background(), &Group{ID: 42, Platform: PlatformOpenAI}, nil)

		model := gatewayCapabilityModelByID(t, models, "unknown-model")
		require.Equal(t, GatewayAvailabilityUnknown, model.Availability)
		require.Nil(t, model.Routing.Routable)
		require.Nil(t, model.Routing.CandidatePaths)
		require.Zero(t, repo.currentCalls)
		require.Equal(t, 1, cache.getCalls[bucket])
		require.Zero(t, cache.captureCalls)
		require.Zero(t, cache.setCalls)
	})

	t.Run("composite miss affects only its target platform", func(t *testing.T) {
		openAI := gatewayCapabilityTestAccount(923341, PlatformOpenAI, map[string]string{
			"gpt-snapshot-one": "gpt-snapshot-one",
			"gpt-snapshot-two": "gpt-snapshot-two",
		})
		grok := gatewayCapabilityTestAccount(923342, PlatformGrok, map[string]string{"grok-unknown": "grok-unknown"})
		repo := &gatewayCapabilityAccountRepoStub{current: []Account{grok}, configured: []Account{openAI, grok}}
		openAIBucket := SchedulerBucket{GroupID: 77, Platform: PlatformOpenAI, Mode: SchedulerModeSingle}
		grokBucket := SchedulerBucket{GroupID: 77, Platform: PlatformGrok, Mode: SchedulerModeSingle}
		cache := &gatewayCapabilitySchedulerCacheStub{
			snapshots: map[SchedulerBucket][]*Account{openAIBucket: {&openAI}},
			errors:    map[SchedulerBucket]error{grokBucket: errors.New("synthetic snapshot failure")},
		}
		gateway := &GatewayService{
			accountRepo:       repo,
			schedulerSnapshot: &SchedulerSnapshotService{cache: cache},
			compositeResolver: NewCompositeRouteResolver(&gatewayCapabilityRouteRepoStub{}),
		}

		models := gateway.BuildGatewayCapabilityModels(context.Background(), &Group{ID: 77, Platform: PlatformComposite}, nil)

		require.Equal(t, GatewayAvailabilityAvailable, gatewayCapabilityModelByID(t, models, "gpt-snapshot-one").Availability)
		require.Equal(t, GatewayAvailabilityAvailable, gatewayCapabilityModelByID(t, models, "gpt-snapshot-two").Availability)
		require.Equal(t, GatewayAvailabilityUnknown, gatewayCapabilityModelByID(t, models, "grok-unknown").Availability)
		require.Equal(t, 1, cache.getCalls[openAIBucket])
		require.Equal(t, 1, cache.getCalls[grokBucket])
		require.Len(t, cache.getCalls, len(gatewayCapabilityPlatforms))
		require.Zero(t, repo.currentCalls)
		require.Zero(t, cache.captureCalls)
		require.Zero(t, cache.setCalls)
	})
}

func TestGatewayCapabilityResponseNeverSerializesAccountSecrets(t *testing.T) {
	account := gatewayCapabilityTestAccount(991337, PlatformOpenAI, map[string]string{"safe-model": "safe-model"})
	account.Name = "operator@example.invalid"
	account.Credentials["api_key"] = "sk-secret-value"
	account.Credentials["authorization"] = "Bearer secret-token"
	account.Credentials["access_token"] = "oauth-access-secret"
	account.Credentials["refresh_token"] = "oauth-refresh-secret"
	account.Credentials["credential_json"] = `{"password":"raw-json-secret"}`
	account.Extra["password"] = "account-password-secret"
	account.Proxy = &Proxy{
		ID:       881122,
		Name:     "private-proxy",
		Protocol: "https",
		Host:     "proxy.internal.invalid",
		Port:     8443,
		Username: "proxy-user",
		Password: "proxy-password-secret",
	}
	repo := &gatewayCapabilityAccountRepoStub{current: []Account{account}, configured: []Account{account}}
	models := (&GatewayService{accountRepo: repo}).BuildGatewayCapabilityModels(
		context.Background(),
		&Group{ID: 42, Platform: PlatformOpenAI},
		nil,
	)

	payload, err := json.Marshal(models)
	require.NoError(t, err)
	serialized := strings.ToLower(string(payload))
	for _, forbidden := range []string{
		"991337", "881122", "operator@example.invalid", "sk-secret-value", "bearer secret-token",
		"oauth-access-secret", "oauth-refresh-secret", "raw-json-secret", "account-password-secret",
		"private-proxy", "proxy.internal.invalid", "proxy-user", "proxy-password-secret",
		"account_id", "group_id", "credentials", "proxy_url", "access_token", "refresh_token",
	} {
		require.NotContains(t, serialized, strings.ToLower(forbidden))
	}
}
