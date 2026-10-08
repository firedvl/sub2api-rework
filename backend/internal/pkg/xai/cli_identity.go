package xai

import (
	"net/http"
	"os"
	"runtime"
	"strings"

	"golang.org/x/mod/semver"
)

// Fixed Grok Build / CLI-chat-proxy client identity.
// These values are intentionally pinned in-binary (not scraped from live CLI).
// Operators may bump the version via XAI_GROK_CLI_VERSION without a release.
const (
	// CLIProxyHost is the hostname that requires the official CLI identity headers.
	CLIProxyHost = "cli-chat-proxy.grok.com"

	// CLIStableVersion is the known-good minimum client version accepted by cli-chat-proxy.
	CLIStableVersion = "1.0.13"

	// CLIVersionEnv is the optional operator override for CLIStableVersion.
	CLIVersionEnv = "XAI_GROK_CLI_VERSION"

	// CLITokenAuth is required by cli-chat-proxy for Grok Build OAuth tokens.
	CLITokenAuth = "xai-grok-cli"

	// CLIClientIdentifier is the x-grok-client-identifier value used by Grok shell/CLI.
	CLIClientIdentifier = "grok-pager"

	// CLIClientMode is used by billing / quota probes on the CLI surface.
	CLIClientMode = "interactive"
)

// ResolveCLIVersion returns a supported CLI client version.
// Empty or invalid overrides fall back to CLIClientVersion (the pinned
// preferred client pin in billing.go). CLIStableVersion is only the minimum
// accepted by IsSupportedCLIVersion, not the default identity we advertise.
func ResolveCLIVersion() string {
	version := strings.TrimSpace(os.Getenv(CLIVersionEnv))
	if !IsSupportedCLIVersion(version) {
		return CLIClientVersion
	}
	return version
}

// IsSupportedCLIVersion reports whether version is a valid semver string at or
// above CLIStableVersion (prereleases below a higher release are rejected when
// they compare less than the stable pin).
func IsSupportedCLIVersion(version string) bool {
	canonical := "v" + version
	minimum := "v" + CLIStableVersion
	return semver.IsValid(canonical) &&
		semver.Canonical(canonical) == canonical &&
		semver.Compare(canonical, minimum) >= 0
}

// CLIUserAgent builds the official interactive CLI identity using Rust platform names.
func CLIUserAgent(version string) string {
	if strings.TrimSpace(version) == "" {
		version = CLIClientVersion
	}
	platform, arch := runtime.GOOS, runtime.GOARCH
	if platform == "darwin" {
		platform = "macos"
	}
	switch arch {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "aarch64"
	case "386":
		arch = "x86"
	}
	return "grok-pager/" + version + " grok-shell/" + version + " (" + platform + "; " + arch + ")"
}

// ApplyCLIProxyHeaders stamps the fixed Grok CLI identity when the request
// targets cli-chat-proxy. Direct api.x.ai traffic is left unchanged.
func ApplyCLIProxyHeaders(req *http.Request) {
	if req == nil || req.URL == nil || !strings.EqualFold(strings.TrimSpace(req.URL.Hostname()), CLIProxyHost) {
		return
	}
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	ApplyCLIIdentityHeaders(req.Header, ResolveCLIVersion())
}

// ApplyCLIIdentityHeaders replaces every casing of the official CLI identity headers.
func ApplyCLIIdentityHeaders(headers http.Header, version string) {
	if headers == nil {
		return
	}
	for name, value := range map[string]string{
		"X-XAI-Token-Auth":         CLITokenAuth,
		"X-Grok-Client-Version":    version,
		"X-Grok-Client-Identifier": CLIClientIdentifier,
		"X-Grok-Client-Mode":       CLIClientMode,
		"X-Authenticateresponse":   "authenticate-response",
		"User-Agent":               CLIUserAgent(version),
	} {
		// Configured overrides can use raw map keys that Header.Set does not replace.
		for existing := range headers {
			if strings.EqualFold(existing, name) {
				delete(headers, existing)
			}
		}
		headers.Set(name, value)
	}
}
