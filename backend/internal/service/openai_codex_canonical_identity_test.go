package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetOpenAICodexCanonicalUserAgentOutboundIdentity(t *testing.T) {
	codexCanonicalUAMu.RLock()
	previous := codexCanonicalUAResolver
	codexCanonicalUAMu.RUnlock()
	t.Cleanup(func() { SetCodexCanonicalUserAgentResolver(previous) })

	for _, version := range []struct {
		name, manual, synced, want string
	}{
		{"manual", "0.154.0", "0.155.1", "0.154.0"},
		{"synced", "", "0.155.1", "0.155.1"},
		{"invalid_manual", "latest", "0.155.1", "0.155.1"},
		{"default", "", "", codexCLIVersion},
		{"invalid_versions", "latest", "invalid", codexCLIVersion},
	} {
		for _, ua := range []struct {
			name, value, client, suffix string
		}{
			{name: "browser", value: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"},
			{name: "arbitrary", value: "not-a-codex-client"},
			{name: "unknown_client", value: "custom/0.140.0 (Linux)"},
			{name: "empty"},
			{name: "missing_version", value: "codex_cli_rs/"},
			{name: "crlf", value: "codex_cli_rs/0.140.0 (Linux)\r\nX-Injected: value"},
			{name: "nul", value: "codex_cli_rs/0.140.0 (Linux\x00)"},
			{name: "del", value: "codex_cli_rs/0.140.0 terminal\x7f"},
			{name: "leading_newline", value: "\ncodex_cli_rs/0.140.0 (Linux)"},
			{name: "trailing_newline", value: "codex_cli_rs/0.140.0 (Linux)\n"},
			{name: "valid_suffix", value: "codex_cli_rs/0.140.0 (Mac OS X 15.1.0; arm64) iTerm.app", client: "codex_cli_rs", suffix: " (Mac OS X 15.1.0; arm64) iTerm.app"},
			{name: "valid_tab", value: "codex_cli_rs/0.140.0 (Linux)\tterminal", client: "codex_cli_rs", suffix: " (Linux)\tterminal"},
			{name: "recognized_trailer", value: "custom/0.140.0 (Linux) (codex-tui; 0.140.0)", client: "codex-tui", suffix: " (Linux) (codex-tui; " + version.want + ")"},
		} {
			t.Run(version.name+"/"+ua.name, func(t *testing.T) {
				svc := NewSettingService(&codexVersionSettingRepoStub{values: map[string]string{
					SettingKeyOpenAICodexUserAgent:           ua.value,
					SettingKeyOpenAICodexClientVersion:       version.manual,
					SettingKeyOpenAICodexClientVersionSynced: version.synced,
				}}, nil)
				wantUA := buildCodexCLIUserAgent(version.want)
				wantOriginator := "codex-tui"
				if ua.client != "" {
					wantUA = ua.client + "/" + version.want + ua.suffix
					wantOriginator = ua.client
				}
				SetCodexCanonicalUserAgentResolver(func() string {
					return svc.GetOpenAICodexCanonicalUserAgent(context.Background())
				})
				require.Equal(t, wantUA, svc.GetOpenAICodexCanonicalUserAgent(context.Background()))
				identity := resolveCodexOutboundIdentity("")
				require.Equal(t, wantUA, identity.userAgent)
				require.Equal(t, wantOriginator, identity.originator)
				require.Equal(t, version.want, identity.version)
			})
		}
	}
}
