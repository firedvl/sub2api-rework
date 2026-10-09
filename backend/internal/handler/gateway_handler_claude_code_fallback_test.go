//go:build unit

package handler

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type fallbackWireUpstream struct{ models []string }

type fallbackReservationCache struct {
	*handlerInflightCache
	amounts []float64
}

func (c *fallbackReservationCache) ReserveInflightBalance(ctx context.Context, userID int64, id string, amount, balance float64, ttl time.Duration) (bool, float64, error) {
	c.amounts = append(c.amounts, amount)
	return c.handlerInflightCache.ReserveInflightBalance(ctx, userID, id, amount, balance, ttl)
}

func (u *fallbackWireUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	u.models = append(u.models, gjson.GetBytes(body, "model").String())
	return &http.Response{StatusCode: http.StatusBadRequest, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":{"message":"synthetic provider rejection"}}`))}, nil
}

func (u *fallbackWireUpstream) DoWithTLS(req *http.Request, proxy string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxy, accountID, concurrency)
}

// groupScopedSchedulerCache 按桶的分组只返回该分组的成员账号，并记录被查询过的分组。
type groupScopedSchedulerCache struct {
	*fakeSchedulerCache
	mu       sync.Mutex
	groupIDs []int64
}

func (c *groupScopedSchedulerCache) GetSnapshot(_ context.Context, bucket service.SchedulerBucket) ([]*service.Account, bool, error) {
	c.mu.Lock()
	c.groupIDs = append(c.groupIDs, bucket.GroupID)
	c.mu.Unlock()
	var members []*service.Account
	for _, account := range c.accounts {
		for _, ag := range account.AccountGroups {
			if ag.GroupID == bucket.GroupID {
				members = append(members, account)
				break
			}
		}
	}
	return members, true, nil
}

func (c *groupScopedSchedulerCache) queriedGroupIDs() []int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]int64(nil), c.groupIDs...)
}

type groupMapRepo struct {
	*fakeGroupRepo
	groups map[int64]*service.Group
}

func (r *groupMapRepo) GetByID(_ context.Context, id int64) (*service.Group, error) {
	if group, ok := r.groups[id]; ok {
		return group, nil
	}
	return nil, service.ErrGroupNotFound
}

func (r *groupMapRepo) GetByIDLite(ctx context.Context, id int64) (*service.Group, error) {
	return r.GetByID(ctx, id)
}

func TestGatewayOpenAICompatibleHandlersClaudeCodeOnlyFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const (
		primaryGroupID    = int64(9300)
		fallbackGroupID   = int64(9301)
		primaryAccountID  = int64(9310)
		fallbackAccountID = int64(9311)
	)

	// 账号不带 api_key：转发在取令牌时失败，请求不会触达上游，断言只看选号结果。
	newAccount := func(id, groupID int64) *service.Account {
		return &service.Account{
			ID: id, Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey,
			Status: service.StatusActive, Schedulable: true, Concurrency: 1,
			AccountGroups: []service.AccountGroup{{AccountID: id, GroupID: groupID}},
		}
	}

	endpoints := []struct {
		name string
		path string
		body string
		call func(*GatewayHandler, *gin.Context)
	}{
		{
			name: "responses", path: "/v1/responses",
			body: `{"model":"claude-sonnet-4-5","input":"hello","stream":false}`,
			call: (*GatewayHandler).Responses,
		},
		{
			name: "chat completions", path: "/v1/chat/completions",
			body: `{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":"hello"}],"stream":false}`,
			call: (*GatewayHandler).ChatCompletions,
		},
	}

	for _, ep := range endpoints {
		for _, tc := range []struct {
			name        string
			hasFallback bool
			wireMapping bool
			reservation bool
		}{
			{name: "with fallback group", hasFallback: true},
			{name: "without fallback group", hasFallback: false},
			{name: "fallback mapping reaches provider", hasFallback: true, wireMapping: true},
			{name: "expensive fallback reserves original billing price", hasFallback: true, wireMapping: true, reservation: true},
		} {
			t.Run(ep.name+"/"+tc.name, func(t *testing.T) {
				primary := &service.Group{
					ID: primaryGroupID, Hydrated: true, Platform: service.PlatformAnthropic,
					Status: service.StatusActive, ClaudeCodeOnly: true,
				}
				if tc.hasFallback {
					fallbackID := fallbackGroupID
					primary.FallbackGroupID = &fallbackID
				}
				fallback := &service.Group{
					ID: fallbackGroupID, Hydrated: true, Platform: service.PlatformAnthropic,
					Status: service.StatusActive,
				}

				schedulerCache := &groupScopedSchedulerCache{fakeSchedulerCache: &fakeSchedulerCache{accounts: []*service.Account{
					newAccount(primaryAccountID, primaryGroupID),
					newAccount(fallbackAccountID, fallbackGroupID),
				}}}
				cfg := &config.Config{RunMode: config.RunModeSimple}
				cheap, expensive := 1.0, 10.0
				if tc.reservation {
					cfg.RunMode = config.RunModeStandard
					cfg.Billing.InflightReservation = config.InflightReservationConfig{Enabled: true, TTLSeconds: 60}
					primary.RateMultiplier = 2
					primary.ModelPricing = []service.ChannelModelPricing{
						{Models: []string{"claude-sonnet-4-5"}, BillingMode: service.BillingModePerRequest, PerRequestPrice: &cheap},
						{Models: []string{"claude-opus-4-6"}, BillingMode: service.BillingModePerRequest, PerRequestPrice: &expensive},
					}
					fallback.RateMultiplier = 50
				}
				upstream := &fallbackWireUpstream{}
				var channels *service.ChannelService
				if tc.wireMapping {
					schedulerCache.accounts[1].Credentials = map[string]any{"api_key": "fixture-key", "base_url": "https://fixture.example", "model_mapping": map[string]any{"claude-sonnet-4-5": "claude-sonnet-4-5"}}
					channels = service.NewChannelService(&openAIWSUsageHandlerChannelRepoStub{
						channels: []service.Channel{
							{ID: 1, Status: service.StatusActive, GroupIDs: []int64{primaryGroupID}, ModelMapping: map[string]map[string]string{service.PlatformAnthropic: {"claude-sonnet-4-5": "claude-opus-4-6"}}},
							{ID: 2, Status: service.StatusActive, GroupIDs: []int64{fallbackGroupID}, RestrictModels: true, BillingModelSource: service.BillingModelSourceUpstream, ModelMapping: map[string]map[string]string{service.PlatformAnthropic: {"claude-sonnet-4-5": "claude-sonnet-4-6"}}, ModelPricing: []service.ChannelModelPricing{{Platform: service.PlatformAnthropic, Models: []string{"claude-sonnet-4-6"}}}},
						},
						groupPlatforms: map[int64]string{primaryGroupID: service.PlatformAnthropic, fallbackGroupID: service.PlatformAnthropic},
					}, nil, nil, nil, nil)
					if tc.reservation {
						channels = service.NewChannelService(&openAIWSUsageHandlerChannelRepoStub{
							channels: []service.Channel{
								{ID: 1, Status: service.StatusActive, GroupIDs: []int64{primaryGroupID}, ModelMapping: map[string]map[string]string{service.PlatformAnthropic: {"claude-sonnet-4-5": "claude-sonnet-4-5"}}},
								{ID: 2, Status: service.StatusActive, GroupIDs: []int64{fallbackGroupID}, BillingModelSource: service.BillingModelSourceChannelMapped, ModelMapping: map[string]map[string]string{service.PlatformAnthropic: {"claude-sonnet-4-5": "claude-opus-4-6"}}},
							},
							groupPlatforms: map[int64]string{primaryGroupID: service.PlatformAnthropic, fallbackGroupID: service.PlatformAnthropic},
						}, nil, nil, nil, nil)
					}
				}
				billingService := service.NewBillingService(cfg, nil)
				gatewayService := service.NewGatewayService(
					nil, &groupMapRepo{fakeGroupRepo: &fakeGroupRepo{}, groups: map[int64]*service.Group{
						primaryGroupID:  primary,
						fallbackGroupID: fallback,
					}}, nil, nil, nil, nil, nil, nil, cfg,
					service.NewSchedulerSnapshotService(schedulerCache, nil, nil, nil, nil),
					nil, billingService, nil, nil, nil, upstream, nil, nil, nil, nil, nil, nil, nil, channels, service.NewModelPricingResolver(channels, billingService), nil, nil, nil,
				)
				reservationCache := &fallbackReservationCache{handlerInflightCache: newHandlerInflightCache(100)}
				billingCacheService := service.NewBillingCacheService(reservationCache, nil, nil, nil, nil, nil, cfg, nil)
				t.Cleanup(billingCacheService.Stop)
				h := &GatewayHandler{
					gatewayService:      gatewayService,
					billingCacheService: billingCacheService,
					concurrencyHelper:   NewConcurrencyHelper(service.NewConcurrencyService(&fakeConcurrencyCache{}), SSEPingFormatClaude, 0),
					maxAccountSwitches:  1,
					cfg:                 cfg,
				}

				primaryGroupIDRef := primaryGroupID
				apiKey := &service.APIKey{
					ID: 9320, UserID: 9330, GroupID: &primaryGroupIDRef, Group: primary, Status: service.StatusActive,
					User: &service.User{ID: 9330, Concurrency: 10, Balance: 100},
				}
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				ctx := context.WithValue(context.Background(), ctxkey.Group, primary)
				req := httptest.NewRequest(http.MethodPost, ep.path, bytes.NewBufferString(ep.body)).WithContext(ctx)
				req.Header.Set("Content-Type", "application/json")
				c.Request = req
				c.Set(string(middleware.ContextKeyAPIKey), apiKey)
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.UserID, Concurrency: 10})

				ep.call(h, c)

				selected, reachedSelection := c.Get(opsAccountIDKey)
				if !tc.hasFallback {
					require.Equal(t, http.StatusForbidden, recorder.Code)
					require.Contains(t, recorder.Body.String(), "This group is restricted to Claude Code clients")
					require.False(t, reachedSelection, "a Claude Code only group without fallback must be rejected before account selection")
					require.Empty(t, schedulerCache.queriedGroupIDs())
					return
				}
				require.NotContains(t, recorder.Body.String(), "restricted to Claude Code clients")
				require.True(t, reachedSelection, "a Claude Code only group with fallback must reach account selection")
				require.Equal(t, fallbackAccountID, selected)
				queried := schedulerCache.queriedGroupIDs()
				require.Contains(t, queried, fallbackGroupID)
				require.NotContains(t, queried, primaryGroupID)
				if tc.wireMapping {
					require.NotEmpty(t, upstream.models)
					for _, model := range upstream.models {
						if tc.reservation {
							require.Equal(t, "claude-opus-4-6", model)
						} else {
							require.Equal(t, "claude-sonnet-4-6", model)
						}
					}
					require.Equal(t, primaryGroupID, *apiKey.GroupID, "fallback routing must not move caller billing ownership")
					if tc.reservation {
						require.Equal(t, []float64{20}, reservationCache.amounts, "reserve expensive fallback price 10 times original billing rate 2")
						require.Zero(t, reservationCache.count(), "synthetic provider errors release the reservation")
						require.Same(t, primary, apiKey.Group)
					}
				}
			})
		}
	}
}
