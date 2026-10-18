package service

import (
	"context"
	"strings"
)

type anthropicAPIOnlyKey struct{}

// WithAnthropicAPIOnly pins authentication for the explicit API-billed entry point.
func WithAnthropicAPIOnly(ctx context.Context) context.Context {
	return context.WithValue(ctx, anthropicAPIOnlyKey{}, true)
}

func IsAnthropicAPIOnly(ctx context.Context) bool {
	return ctx != nil && ctx.Value(anthropicAPIOnlyKey{}) == true
}

func allowsAnthropicAPIAccount(ctx context.Context, account *Account) bool {
	return !IsAnthropicAPIOnly(ctx) || (account != nil && account.Platform == PlatformAnthropic && account.Type == AccountTypeAPIKey)
}

// ScopeAnthropicAPISession keeps API-only affinity separate from legacy sessions.
func ScopeAnthropicAPISession(ctx context.Context, hash string) string {
	if IsAnthropicAPIOnly(ctx) && hash != "" {
		return "anthropic-api:" + hash
	}
	if strings.HasPrefix(hash, "anthropic-api:") {
		return "legacy:" + hash
	}
	return hash
}
