package xai

import (
	"net/http"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/mod/semver"
)

func TestResolveCLIVersionDefaultsToPinnedClientVersion(t *testing.T) {
	t.Setenv(CLIVersionEnv, "")
	// Default advertise pin is CLIClientVersion; CLIStableVersion is only the floor.
	require.Equal(t, CLIClientVersion, ResolveCLIVersion())
	require.True(t, IsSupportedCLIVersion(CLIClientVersion))
	require.True(t, IsSupportedCLIVersion(CLIStableVersion))
}

func TestResolveCLIVersionAcceptsValidOverride(t *testing.T) {
	t.Setenv(CLIVersionEnv, "1.0.14-alpha.1")
	require.Equal(t, "1.0.14-alpha.1", ResolveCLIVersion())
}

func TestResolveCLIVersionRejectsUnsafeOrTooOld(t *testing.T) {
	for _, version := range []string{
		"0.2.120",
		"1.0.12",
		"1.0.13-beta.1",
		"1.0.14\r\nX-Injected: true",
		"1.0.014",
		"1.1",
		"2",
	} {
		t.Run(version, func(t *testing.T) {
			t.Setenv(CLIVersionEnv, version)
			require.Equal(t, CLIClientVersion, ResolveCLIVersion())
		})
	}
}

func TestApplyCLIProxyHeaders(t *testing.T) {
	t.Setenv(CLIVersionEnv, "")

	req, err := http.NewRequest(http.MethodPost, "https://cli-chat-proxy.grok.com/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "legacy-client/1.0")
	for _, name := range []string{"X-XAI-Token-Auth", "X-Grok-Client-Version", "X-Grok-Client-Identifier", "X-Grok-Client-Mode", "X-Authenticateresponse", "User-Agent"} {
		req.Header[strings.ToLower(name)] = []string{"lowercase-stale"}
		req.Header[strings.ToLower(name[:1])+strings.ToUpper(name[1:])] = []string{"mixed-case-stale"}
	}

	ApplyCLIProxyHeaders(req)

	require.Equal(t, CLIClientVersion, req.Header.Get("x-grok-client-version"))
	require.Equal(t, CLIClientIdentifier, req.Header.Get("x-grok-client-identifier"))
	require.Equal(t, CLITokenAuth, req.Header.Get("X-XAI-Token-Auth"))
	require.Equal(t, "interactive", req.Header.Get("x-grok-client-mode"))
	require.Equal(t, "authenticate-response", req.Header.Get("x-authenticateresponse"))
	require.Equal(t, CLIUserAgent(CLIClientVersion), req.Header.Get("User-Agent"))
	for name := range req.Header {
		require.Equal(t, http.CanonicalHeaderKey(name), name)
	}
}

func TestApplyCLIProxyHeadersLeavesAPIHostUnchanged(t *testing.T) {
	t.Setenv(CLIVersionEnv, "1.0.14")

	req, err := http.NewRequest(http.MethodPost, "https://api.x.ai/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "direct-api-client/1.0")

	ApplyCLIProxyHeaders(req)

	require.Empty(t, req.Header.Get("x-grok-client-version"))
	require.Empty(t, req.Header.Get("x-grok-client-identifier"))
	require.Empty(t, req.Header.Get("X-XAI-Token-Auth"))
	require.Empty(t, req.Header.Get("x-grok-client-mode"))
	require.Empty(t, req.Header.Get("x-authenticateresponse"))
	require.Equal(t, "direct-api-client/1.0", req.Header.Get("User-Agent"))
}

func TestApplyCLIProxyHeadersMeetsUpstreamMinimumVersion(t *testing.T) {
	for _, override := range []string{"", "0.2.93", "0.2.120", "1.0.12", "1.0.13-beta.1", "1.0.13", "1.0.14-alpha.1"} {
		t.Run("override="+override, func(t *testing.T) {
			t.Setenv(CLIVersionEnv, override)
			req, err := http.NewRequest(http.MethodPost, "https://cli-chat-proxy.grok.com/v1/responses", nil)
			require.NoError(t, err)
			ApplyCLIProxyHeaders(req)
			version := req.Header.Get("x-grok-client-version")
			require.True(t, semver.IsValid("v"+version))
			require.GreaterOrEqual(t, semver.Compare("v"+version, "v1.0.13"), 0)
			require.Contains(t, req.UserAgent(), "grok-pager/"+version+" grok-shell/"+version+" (")
		})
	}
}

func TestCLIUserAgentUsesRuntimePlatform(t *testing.T) {
	platform := runtime.GOOS
	if platform == "darwin" {
		platform = "macos"
	}
	arch := map[string]string{"amd64": "x86_64", "arm64": "aarch64", "386": "x86"}[runtime.GOARCH]
	if arch == "" {
		arch = runtime.GOARCH
	}
	require.Equal(t, "grok-pager/1.0.46 grok-shell/1.0.46 ("+platform+"; "+arch+")", CLIUserAgent("1.0.46"))
	require.Equal(t, CLIUserAgent(CLIClientVersion), CLIUserAgent(""))
}
