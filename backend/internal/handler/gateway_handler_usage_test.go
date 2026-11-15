package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	gocache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/require"
)

func TestUsageUnrestrictedIncludesWeeklyWindowStart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/usage", nil)

	weeklyWindowStart := time.Date(2026, time.July, 13, 0, 30, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	c.Set(string(middleware.ContextKeySubscription), &service.UserSubscription{
		WeeklyWindowStart: &weeklyWindowStart,
	})

	handler := &GatewayHandler{}
	handler.usageUnrestricted(
		c,
		context.Background(),
		&service.APIKey{Group: &service.Group{
			Name:             "Weekly plan",
			SubscriptionType: service.SubscriptionTypeSubscription,
		}},
		middleware.AuthSubject{},
		nil,
		nil,
		nil,
	)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Subscription struct {
			WeeklyWindowStart *time.Time `json:"weekly_window_start"`
		} `json:"subscription"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.NotNil(t, response.Subscription.WeeklyWindowStart)
	require.True(t, weeklyWindowStart.Equal(*response.Subscription.WeeklyWindowStart))
}

type usageStatsRepoStub struct {
	service.UsageLogRepository
	dashboardCalls int
	trendCalls     int
	modelCalls     int
	dashboardErr   error
}

func (r *usageStatsRepoStub) GetAPIKeyDashboardStats(context.Context, int64) (*usagestats.UserDashboardStats, error) {
	r.dashboardCalls++
	if r.dashboardErr != nil {
		return nil, r.dashboardErr
	}
	return &usagestats.UserDashboardStats{TotalRequests: int64(r.dashboardCalls)}, nil
}

func (r *usageStatsRepoStub) GetUsageTrendWithFilters(context.Context, time.Time, time.Time, string, int64, int64, int64, int64, string, *int16, *bool, *int8) ([]usagestats.TrendDataPoint, error) {
	r.trendCalls++
	return []usagestats.TrendDataPoint{{Date: "2026-11-14", Requests: 3}}, nil
}

func (r *usageStatsRepoStub) GetModelStatsWithFilters(context.Context, time.Time, time.Time, int64, int64, int64, int64, *int16, *bool, *int8) ([]usagestats.ModelStat, error) {
	r.modelCalls++
	return []usagestats.ModelStat{{Model: "claude-fable-5-2", Requests: 3}}, nil
}

func newUsageStatsTestHandler(repo *usageStatsRepoStub) *GatewayHandler {
	return &GatewayHandler{
		usageService:    service.NewUsageService(repo, nil, nil, nil),
		usageStatsCache: gocache.New(apiKeyUsageStatsTTL, time.Minute),
	}
}

func loadUsageStatsForTest(h *GatewayHandler, apiKeyID int64, rawQuery string) apiKeyUsageStats {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/usage?"+rawQuery, nil)
	startTime, endTime := h.parseUsageDateRange(c)
	return h.loadAPIKeyUsageStats(c, 1, apiKeyID, defaultAPIKeyDailyUsageDays, startTime, endTime)
}

func TestLoadAPIKeyUsageStatsServesRepeatedPollsFromCache(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &usageStatsRepoStub{}
	h := newUsageStatsTestHandler(repo)

	first := loadUsageStatsForTest(h, 10, "")
	second := loadUsageStatsForTest(h, 10, "")
	require.Equal(t, 1, repo.dashboardCalls)
	require.Equal(t, 1, repo.trendCalls)
	require.Equal(t, 1, repo.modelCalls)
	require.Equal(t, first, second)
	require.NotNil(t, first.usageData)
	require.NotNil(t, first.dailyUsage)
	require.NotNil(t, first.modelStats)

	// 无效参数回落到默认范围，与默认请求共用同一条缓存。
	loadUsageStatsForTest(h, 10, "timezone=not-a-zone&start_date=garbage")
	require.Equal(t, 1, repo.dashboardCalls)

	// 不同 API Key、不同查询范围各自独立。
	loadUsageStatsForTest(h, 11, "")
	require.Equal(t, 2, repo.dashboardCalls)
	loadUsageStatsForTest(h, 10, "start_date=2026-11-01")
	require.Equal(t, 3, repo.dashboardCalls)
}

func TestLoadAPIKeyUsageStatsReloadsAfterExpiry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &usageStatsRepoStub{}
	h := newUsageStatsTestHandler(repo)

	first := loadUsageStatsForTest(h, 10, "")
	// 等价于 TTL 到期。
	h.usageStatsCache.Flush()
	second := loadUsageStatsForTest(h, 10, "")

	require.Equal(t, 2, repo.dashboardCalls)
	require.Equal(t, 2, repo.trendCalls)
	require.Equal(t, 2, repo.modelCalls)
	require.NotEqual(t, first.usageData, second.usageData)
}

func TestLoadAPIKeyUsageStatsDoesNotCacheFailedQueries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &usageStatsRepoStub{dashboardErr: errors.New("db unavailable")}
	h := newUsageStatsTestHandler(repo)

	partial := loadUsageStatsForTest(h, 10, "")
	require.Nil(t, partial.usageData)
	require.NotNil(t, partial.dailyUsage, "other sections are still returned")

	repo.dashboardErr = nil
	recovered := loadUsageStatsForTest(h, 10, "")
	require.NotNil(t, recovered.usageData)
	require.Equal(t, 2, repo.dashboardCalls)
	require.Equal(t, 2, repo.trendCalls)
}
