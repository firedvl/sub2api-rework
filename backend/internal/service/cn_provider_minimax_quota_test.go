package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type minimaxQuotaTestRepo struct {
	AccountRepository
	account *Account
	updates map[string]any
}

func (repo *minimaxQuotaTestRepo) GetByID(context.Context, int64) (*Account, error) {
	return repo.account, nil
}

func (repo *minimaxQuotaTestRepo) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	repo.updates = updates
	return nil
}

type minimaxQuotaTestUpstream struct {
	HTTPUpstream
	requests []*http.Request
}

func (upstream *minimaxQuotaTestUpstream) Do(request *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	upstream.requests = append(upstream.requests, request)
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"base_resp":{"status_code":0},"model_remains":[{"model_name":"general","current_interval_remaining_percent":75}]}`)),
	}, nil
}

func TestCNProviderQuotaService_MiniMaxRejectsUnofficialBaseURL(t *testing.T) {
	baseURLs := []string{
		"https://relay.example/v1",
		"https://relay.example/minimax.io/v1",
		"https://relay.example/minimaxi.com/v1",
		"https://relay.example/minimax.com/v1",
		"https://api.minimax.io.relay.example/v1",
		"https://api.minimaxi.com.relay.example/v1",
		"https://api.minimax.com.relay.example/v1",
		"https://relay.example/v1?provider=api.minimax.io",
		"https://relay.example/v1#api.minimaxi.com",
		"https://api.minimax.io@relay.example/v1",
		"https://relay.example@api.minimax.io/v1",
		"https://api%2eminimax.io/v1",
		"https://api.minimax.io\\relay.example/v1",
		"https://api.minimax.io:invalid/v1",
		"https://api.minimax.io:65536/v1",
		"//api.minimax.io/v1",
		"https:api.minimax.io/v1",
		"ftp://api.minimax.io/v1",
	}
	for _, baseURL := range baseURLs {
		t.Run(baseURL, func(t *testing.T) {
			for _, protocol := range []string{"", APIProtocolChatCompletions, APIProtocolAnthropic, APIProtocolResponses} {
				account := codingAccount(PlatformMiniMax)
				account.Credentials["api_key"] = "fake-relay-token"
				if protocol != "" {
					account.Credentials["api_protocol"] = APIProtocolAdaptive
					account.Credentials["api_base_urls"] = map[string]any{protocol: baseURL}
				} else {
					account.Credentials["base_url"] = baseURL
				}
				repo := &minimaxQuotaTestRepo{account: account}
				upstream := &minimaxQuotaTestUpstream{}
				service := NewCNProviderQuotaService(repo, nil, upstream, cnProbeAllowlistConfig("api.minimax.io", "api.minimaxi.com"))
				_, err := service.QueryUsage(context.Background(), account.ID)
				require.Empty(t, upstream.requests, "unofficial %s endpoint must not release the shared key", protocol)
				requireReason(t, err, "CN_QUOTA_NOT_CODING_PLAN")
				_, err = service.QueryUsageForAccount(context.Background(), account)
				requireReason(t, err, "CN_QUOTA_NOT_CODING_PLAN")
				require.Empty(t, upstream.requests)
				require.Nil(t, repo.updates)
				require.Empty(t, minimaxQuotaURL(baseURL))
				require.Empty(t, account.GetCodingPlanProvider())
				require.ErrorIs(t, monitorAccountQuotaCapability(account), ErrChannelMonitorAccountNotSupportable)
			}
		})
	}
}

func TestCNProviderQuotaService_MiniMaxOfficialBaseURL(t *testing.T) {
	cases := []struct {
		baseURL string
		host    string
	}{
		{"", "api.minimaxi.com"},
		{"https://api.minimax.io/v1", "api.minimax.io"},
		{"https://api.minimax.io/anthropic", "api.minimax.io"},
		{"https://api.minimaxi.com/v1", "api.minimaxi.com"},
		{"https://api.minimaxi.com/anthropic", "api.minimaxi.com"},
		{"https://api.minimax.com/v1", "api.minimaxi.com"},
		{"https://api.minimax.com/anthropic", "api.minimaxi.com"},
		{"https://api.minimaxi.com/minimax.io/v1?provider=minimax.io", "api.minimaxi.com"},
		{"https://api.minimax.com/v1?provider=minimax.io", "api.minimaxi.com"},
		{"https://API.MINIMAX.IO:443/v1/", "api.minimax.io"},
		{"http://api.minimaxi.com/v1", "api.minimaxi.com"},
	}
	for _, testCase := range cases {
		t.Run(testCase.baseURL, func(t *testing.T) {
			for _, adaptive := range []bool{false, true} {
				account := codingAccount(PlatformMiniMax)
				if adaptive {
					account.Credentials["api_protocol"] = APIProtocolAdaptive
					account.Credentials["api_base_urls"] = map[string]any{
						APIProtocolChatCompletions: testCase.baseURL,
						APIProtocolAnthropic:       testCase.baseURL,
						APIProtocolResponses:       testCase.baseURL,
					}
				} else {
					account.Credentials["base_url"] = testCase.baseURL
				}
				require.Equal(t, PlatformMiniMax, account.GetCodingPlanProvider())
				require.NoError(t, monitorAccountQuotaCapability(account))
				repo := &minimaxQuotaTestRepo{account: account}
				upstream := &minimaxQuotaTestUpstream{}
				service := NewCNProviderQuotaService(repo, nil, upstream, cnProbeAllowlistConfig("api.minimax.io", "api.minimaxi.com"))
				result, err := service.QueryUsage(context.Background(), account.ID)
				require.NoError(t, err)
				require.True(t, result.Success)
				require.True(t, result.CredentialValid)
				require.True(t, result.Persisted)
				require.Equal(t, float64(25), repo.updates[cnExtraKey(PlatformMiniMax, cnExtraSuffix5hUsed)])
				_, err = service.QueryUsageForAccount(context.Background(), account)
				require.NoError(t, err)
				require.Len(t, upstream.requests, 2)
				for _, request := range upstream.requests {
					require.Equal(t, "https://"+testCase.host+"/v1/api/openplatform/coding_plan/remains", request.URL.String())
					require.Equal(t, "Bearer sk-test", request.Header.Get("Authorization"))
				}
			}
		})
	}
}
