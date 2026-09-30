package claude

import (
	"strings"
	"testing"
)

func withCLIVersionResolver(test *testing.T, resolver func() string) {
	test.Helper()
	SetCLIVersionResolver(resolver)
	test.Cleanup(func() { SetCLIVersionResolver(nil) })
}

func TestEffectiveCLIVersionDefaultsToCLIVersion(test *testing.T) {
	SetCLIVersionResolver(nil)
	if got := EffectiveCLIVersion(); got != CLIVersion() {
		test.Fatalf("EffectiveCLIVersion() = %q, want CLIVersion() = %q", got, CLIVersion())
	}
	wantUA := "claude-cli/" + CLIVersion() + " (external, cli)"
	if got := DefaultHeaders()["User-Agent"]; got != wantUA {
		test.Fatalf("DefaultHeaders()[User-Agent] = %q, want %q", got, wantUA)
	}
}

func TestEffectiveCLIVersionFollowsResolver(test *testing.T) {
	const upgraded = "9.9.9"
	withCLIVersionResolver(test, func() string { return upgraded })

	if got := EffectiveCLIVersion(); got != upgraded {
		test.Fatalf("EffectiveCLIVersion() = %q, want %q", got, upgraded)
	}
	wantUA := "claude-cli/" + upgraded + " (external, cli)"
	if got := DefaultHeaders()["User-Agent"]; got != wantUA {
		test.Fatalf("DefaultHeaders()[User-Agent] = %q, want %q", got, wantUA)
	}
	if got := DefaultHeaders()["X-App"]; got != "cli" {
		test.Fatalf("DefaultHeaders()[X-App] = %q, want cli", got)
	}
}

func TestEffectiveCLIVersionFallsBackOnInvalidResolverValues(test *testing.T) {
	for name, invalid := range map[string]string{
		"empty":          "",
		"not semver":     "abc",
		"below baseline": "1.0.0",
		"whitespace":     "   ",
	} {
		test.Run(name, func(test *testing.T) {
			withCLIVersionResolver(test, func() string { return invalid })
			if got := EffectiveCLIVersion(); got != CLIVersion() {
				test.Fatalf("resolver %q: EffectiveCLIVersion() = %q, want fallback %q", invalid, got, CLIVersion())
			}
			ua := DefaultHeaders()["User-Agent"]
			if !strings.HasPrefix(ua, "claude-cli/"+CLIVersion()+" ") {
				test.Fatalf("UA %q does not carry fallback version %q", ua, CLIVersion())
			}
		})
	}
}

func TestSetCLIVersionResolverNilRestoresFallback(test *testing.T) {
	withCLIVersionResolver(test, func() string { return "9.9.9" })
	SetCLIVersionResolver(nil)
	if got := EffectiveCLIVersion(); got != CLIVersion() {
		test.Fatalf("after nil resolver: EffectiveCLIVersion() = %q, want %q", got, CLIVersion())
	}
}
