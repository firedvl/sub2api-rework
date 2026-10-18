//go:build unit

package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

type anthropicAPIHandlerUpstream struct {
	firstStatus int
	accountIDs  []int64
	keys        []string
	bodies      [][]byte
}

func (u *anthropicAPIHandlerUpstream) Do(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	u.accountIDs = append(u.accountIDs, accountID)
	u.keys = append(u.keys, req.Header.Get("x-api-key"))
	u.bodies = append(u.bodies, body)
	status := http.StatusOK
	response := `{"id":"msg_fixture","type":"message","role":"assistant","model":"claude-haiku-4-5","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":2,"output_tokens":1}}`
	if accountID == 71 && u.firstStatus != http.StatusOK {
		status = u.firstStatus
		response = `{"type":"error","error":{"type":"invalid_request_error","message":"synthetic upstream failure"}}`
	}
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(response)), Request: req}, nil
}

func (u *anthropicAPIHandlerUpstream) DoWithTLS(req *http.Request, proxy string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxy, accountID, concurrency)
}

func TestGatewayMessagesAnthropicAPIOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name        string
		firstStatus int
		wantStatus  int
		wantIDs     []int64
		wantKeys    []string
	}{
		{"429 fails over with successful account attribution", http.StatusTooManyRequests, http.StatusOK, []int64{71, 72}, []string{"fixture-A", "fixture-B"}},
		{"400 does not fail over", http.StatusBadRequest, http.StatusBadRequest, []int64{71}, []string{"fixture-A"}},
		{"native warmup reaches upstream", http.StatusOK, http.StatusOK, []int64{71}, []string{"fixture-A"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			groupID := int64(70)
			group := &service.Group{ID: groupID, Hydrated: true, Platform: service.PlatformAnthropic, Status: service.StatusActive, RateMultiplier: 1}
			accounts := make([]*service.Account, 0, 3)
			for i, id := range []int64{73, 71, 72} {
				account := &service.Account{ID: id, Name: fmt.Sprintf("fixture-%d", id), Platform: service.PlatformAnthropic,
					Type: service.AccountTypeAPIKey, Priority: i, Concurrency: 1, Schedulable: true, Status: service.StatusActive,
					Credentials:   map[string]any{"api_key": fmt.Sprintf("fixture-%c", 'A'+i-1), "base_url": "https://api.fixture.test", "intercept_warmup_requests": true},
					AccountGroups: []service.AccountGroup{{AccountID: id, GroupID: groupID}}}
				if id == 73 {
					account.Type = service.AccountTypeOAuth
					account.Credentials = map[string]any{"access_token": "must-never-forward"}
				}
				accounts = append(accounts, account)
			}
			cfg := &config.Config{RunMode: config.RunModeSimple}
			cfg.Default.RateMultiplier = 1
			upstream := &anthropicAPIHandlerUpstream{firstStatus: tc.firstStatus}
			usageRepo := &openAIWSUsageHandlerUsageLogRepoStub{created: make(chan *service.UsageLog, 2)}
			rateLimits := service.NewRateLimitService(&openAIWSFailoverHandlerAccountRepoStub{}, usageRepo, cfg, nil, nil)
			channels := service.NewChannelService(&openAIWSUsageHandlerChannelRepoStub{
				channels:       []service.Channel{{ID: 74, Status: service.StatusActive, GroupIDs: []int64{groupID}, FeaturesConfig: map[string]any{"bedrock_cc_compat": true}}},
				groupPlatforms: map[int64]string{groupID: service.PlatformAnthropic},
			}, nil, nil, nil, nil)
			gateway := service.NewGatewayService(nil, &fakeGroupRepo{group: group}, usageRepo, nil, nil, nil, nil, nil, cfg,
				service.NewSchedulerSnapshotService(&fakeSchedulerCache{accounts: accounts}, nil, nil, nil, nil),
				nil, service.NewBillingService(cfg, nil), rateLimits, nil, nil, upstream, &service.DeferredService{}, nil, nil, nil, nil, nil, nil,
				channels, nil, nil, nil, nil)
			billingCache := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
			t.Cleanup(billingCache.Stop)
			h := &GatewayHandler{gatewayService: gateway, billingCacheService: billingCache, cfg: cfg, maxAccountSwitches: 1,
				concurrencyHelper:     NewConcurrencyHelper(service.NewConcurrencyService(&fakeConcurrencyCache{}), SSEPingFormatClaude, 0),
				usageRecordWorkerPool: newUsageRecordTestPool(t)}
			body := `{"model":"claude-haiku-4-5","max_tokens":1,"messages":[{"role":"user","content":"Warmup"}],"context_management":{"edits":[]}}`
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			ctx := service.WithAnthropicAPIOnly(context.WithValue(context.Background(), ctxkey.Group, group))
			c.Request = httptest.NewRequest(http.MethodPost, "/anthropic-api/v1/messages", strings.NewReader(body)).WithContext(ctx)
			c.Request.Header.Set("Content-Type", "application/json")
			c.Request.Header.Set("anthropic-beta", "context-management-2025-06-27")
			key := &service.APIKey{ID: 75, UserID: 76, GroupID: &groupID, Group: group, Status: service.StatusActive,
				User: &service.User{ID: 76, Concurrency: 10, Balance: 100}}
			c.Set(string(middleware.ContextKeyAPIKey), key)
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: key.UserID, Concurrency: 10})

			h.Messages(c)

			require.Equal(t, tc.wantStatus, recorder.Code, recorder.Body.String())
			require.Equal(t, tc.wantIDs, upstream.accountIDs)
			require.Equal(t, tc.wantKeys, upstream.keys)
			for _, sent := range upstream.bodies {
				require.Equal(t, int64(1), gjson.GetBytes(sent, "max_tokens").Int())
				require.True(t, gjson.GetBytes(sent, "context_management").Exists(), string(sent))
			}
			if tc.wantStatus != http.StatusOK {
				return
			}
			require.Contains(t, recorder.Body.String(), "msg_fixture")
			select {
			case usage := <-usageRepo.created:
				require.Equal(t, tc.wantIDs[len(tc.wantIDs)-1], usage.AccountID)
				require.Equal(t, key.ID, usage.APIKeyID)
				require.Equal(t, key.UserID, usage.UserID)
				require.NotNil(t, usage.GroupID)
				require.Equal(t, groupID, *usage.GroupID)
			case <-time.After(5 * time.Second):
				t.Fatal("successful API response did not record usage")
			}
		})
	}
}
