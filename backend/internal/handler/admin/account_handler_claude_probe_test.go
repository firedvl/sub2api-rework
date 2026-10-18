package admin

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type claudeProbeRecoveryRepo struct {
	service.AccountRepository
	account *service.Account
	cleared []string
}

func (r *claudeProbeRecoveryRepo) GetByID(context.Context, int64) (*service.Account, error) {
	return r.account, nil
}

func (r *claudeProbeRecoveryRepo) ClearError(context.Context, int64) error {
	r.cleared = append(r.cleared, "error")
	return nil
}

func (r *claudeProbeRecoveryRepo) ClearRateLimit(context.Context, int64) error {
	r.cleared = append(r.cleared, "rate_limit")
	return nil
}

func (r *claudeProbeRecoveryRepo) ClearAntigravityQuotaScopes(context.Context, int64) error {
	r.cleared = append(r.cleared, "quota_scopes")
	return nil
}

func (r *claudeProbeRecoveryRepo) ClearModelRateLimits(context.Context, int64) error {
	r.cleared = append(r.cleared, "model_rate_limit")
	return nil
}

func (r *claudeProbeRecoveryRepo) ClearTempUnschedulable(context.Context, int64) error {
	r.cleared = append(r.cleared, "temp_unschedulable")
	return nil
}

func TestAccountHandlerClaudeProbeRecoversOnlyAfterNativeCompletion(t *testing.T) {
	start := "data: {\"type\":\"message_start\",\"message\":{\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"stop_reason\":null}}\n\n"
	delta := "data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"max_tokens\"}}\n\n"
	stop := "data: {\"type\":\"message_stop\"}"
	for _, tc := range []struct {
		name    string
		stream  string
		success bool
	}{
		{name: "empty"},
		{name: "truncated", stream: start},
		{name: "done", stream: "data: [DONE]\n\n"},
		{name: "stop without terminal delta", stream: start + stop + "\n\n"},
		{name: "malformed", stream: start + "data: {oops}\n\n" + delta + stop},
		{name: "error", stream: start + "data: {\"type\":\"error\",\"error\":{\"message\":\"overloaded\"}}\n\n"},
		{name: "complete without final newline", stream: start + delta + stop, success: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			until := time.Now().Add(time.Hour)
			repo := &claudeProbeRecoveryRepo{account: &service.Account{
				ID: 881, Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey,
				Status: service.StatusError, Concurrency: 1, RateLimitResetAt: &until,
				TempUnschedulableUntil: &until,
				Credentials:            map[string]any{"api_key": "test-key", "base_url": "https://anthropic.example"},
			}}
			upstream := &syncUpstreamHTTPUpstream{resp: &http.Response{
				StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(tc.stream)),
			}}
			cfg := &config.Config{}
			probe := service.NewAccountTestService(repo, nil, nil, nil, nil, upstream, cfg, nil)
			handler := &AccountHandler{
				accountTestService: probe,
				rateLimitService:   service.NewRateLimitService(repo, nil, cfg, nil, nil),
			}
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.POST("/accounts/:id/test", handler.Test)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/accounts/881/test", nil))

			if tc.success {
				require.Contains(t, recorder.Body.String(), `"success":true`)
				require.Equal(t, []string{"error", "rate_limit", "quota_scopes", "model_rate_limit", "temp_unschedulable"}, repo.cleared)
			} else {
				require.Contains(t, recorder.Body.String(), `"type":"error"`)
				require.NotContains(t, recorder.Body.String(), `"success":true`)
				require.Empty(t, repo.cleared, "failed probe must preserve account error and cooldowns")
			}
		})
	}
}
