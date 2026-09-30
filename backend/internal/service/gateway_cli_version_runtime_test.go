package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func withCLIVersionResolverForTest(test *testing.T, resolver func() string) {
	test.Helper()
	claude.SetCLIVersionResolver(resolver)
	test.Cleanup(func() { claude.SetCLIVersionResolver(nil) })
}

func TestBuildOAuthMimicRequest_RuntimeVersionConsistentBetweenHeaderAndBilling(test *testing.T) {
	gin.SetMode(gin.TestMode)
	const upgraded = "9.9.9"
	withCLIVersionResolverForTest(test, func() string { return upgraded })

	for _, endpoint := range []string{"messages", "count_tokens"} {
		test.Run(endpoint, func(test *testing.T) {
			resetGatewayForwardingSettingsCacheForTest(test)
			ginContext, _ := gin.CreateTestContext(httptest.NewRecorder())
			ginContext.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			body := []byte(`{"model":"claude-haiku-4-5","system":[{"type":"text","text":""}],"messages":[{"role":"user","content":"hello world"}]}`)
			billing, err := buildBillingAttributionText(body, "2.1.81")
			require.NoError(test, err)
			body, err = sjson.SetBytes(body, "system.0.text", billing)
			require.NoError(test, err)

			svc := &GatewayService{cfg: &config.Config{}}
			account := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeOAuth}

			var req *http.Request
			var wireBody []byte
			if endpoint == "messages" {
				req, wireBody, err = svc.buildUpstreamRequest(context.Background(), ginContext, account,
					body, "test-token", "oauth", "claude-haiku-4-5", false, true)
			} else {
				req, wireBody, err = svc.buildCountTokensRequest(context.Background(), ginContext, account,
					body, "test-token", "oauth", "claude-haiku-4-5", true)
			}
			require.NoError(test, err)

			wantUA := "claude-cli/" + upgraded + " (external, cli)"
			outboundUA := getHeaderRaw(req.Header, "User-Agent")
			require.Equal(test, wantUA, outboundUA)

			billingText := gjson.GetBytes(wireBody, "system.0.text").String()
			require.Contains(test, billingText, "x-anthropic-billing-header")
			require.Contains(test, billingText, "cc_version="+upgraded+"."+computeClaudeCodeFingerprint(wireBody, upgraded)+";")
			require.NotContains(test, billingText, "cc_version=2.1.81")

			headerVersion := ExtractCLIVersion(outboundUA)
			require.Equal(test, upgraded, headerVersion)
			require.Contains(test, billingText, "cc_version="+headerVersion+".")
		})
	}
}

func TestBuildOAuthMimicRequest_VersionChangeCannotSplitHeaderAndBilling(test *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, endpoint := range []string{"messages", "count_tokens"} {
		test.Run(endpoint, func(test *testing.T) {
			var calls atomic.Uint64
			withCLIVersionResolverForTest(test, func() string {
				if calls.Add(1)%2 == 0 {
					return "10.0.0"
				}
				return "9.9.9"
			})
			resetGatewayForwardingSettingsCacheForTest(test)
			ginContext, _ := gin.CreateTestContext(httptest.NewRecorder())
			ginContext.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			body := []byte(`{"model":"claude-haiku-4-5","system":[{"type":"text","text":""}],"messages":[{"role":"user","content":"hello"}]}`)
			billing, err := buildBillingAttributionText(body, "2.1.81")
			require.NoError(test, err)
			body, err = sjson.SetBytes(body, "system.0.text", billing)
			require.NoError(test, err)
			svc := &GatewayService{cfg: &config.Config{}}
			account := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeOAuth}
			var request *http.Request
			if endpoint == "messages" {
				request, body, err = svc.buildUpstreamRequest(context.Background(), ginContext, account, body, "token", "oauth", "claude-haiku-4-5", false, true)
			} else {
				request, body, err = svc.buildCountTokensRequest(context.Background(), ginContext, account, body, "token", "oauth", "claude-haiku-4-5", true)
			}
			require.NoError(test, err)
			version := ExtractCLIVersion(getHeaderRaw(request.Header, "User-Agent"))
			require.Contains(test, []string{"9.9.9", "10.0.0"}, version)
			require.Contains(test, gjson.GetBytes(body, "system.0.text").String(), "cc_version="+version+"."+computeClaudeCodeFingerprint(body, version)+";")
		})
	}
}

func TestFloorClaudeCLIUserAgentVersion_UsesRuntimeVersion(test *testing.T) {
	const upgraded = "9.9.9"
	withCLIVersionResolverForTest(test, func() string { return upgraded })

	floored, changed := floorClaudeCLIUserAgentVersion("claude-cli/2.1.100 (external, cli)")
	require.True(test, changed)
	require.Equal(test, "claude-cli/"+upgraded+" (external, cli)", floored)

	same, changed := floorClaudeCLIUserAgentVersion("claude-cli/" + upgraded + " (external, cli)")
	require.False(test, changed)
	require.Equal(test, "claude-cli/"+upgraded+" (external, cli)", same)

	greater, changed := floorClaudeCLIUserAgentVersion("claude-cli/99.0.0 (external, cli)")
	require.False(test, changed)
	require.Equal(test, "claude-cli/99.0.0 (external, cli)", greater)

	withCLIVersionResolverForTest(test, func() string { return "abc" })
	baselineFloored, changed := floorClaudeCLIUserAgentVersion("claude-cli/1.0.0 (external, cli)")
	require.True(test, changed)
	require.Equal(test, "claude-cli/"+claude.CLIVersion()+" (external, cli)", baselineFloored)
}

func TestIsAcceptableFingerprintUserAgent_UsesRuntimeVersion(test *testing.T) {
	const upgraded = "9.9.9"
	withCLIVersionResolverForTest(test, func() string { return upgraded })
	require.True(test, isAcceptableFingerprintUserAgent("claude-cli/"+upgraded+" (external, cli)"))
	require.True(test, isAcceptableFingerprintUserAgent("claude-cli/11.0.0 (external, cli)"))
	require.False(test, isAcceptableFingerprintUserAgent("claude-cli/12.0.0 (external, cli)"))
}

func TestDefaultFingerprintUsesRuntimeVersion(test *testing.T) {
	const upgraded = "9.9.9"
	withCLIVersionResolverForTest(test, func() string { return upgraded })
	require.Equal(test, "claude-cli/"+upgraded+" (external, cli)", defaultFingerprint().UserAgent)
}
