package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
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

func (s *GatewayService) anthropicAPIAccountInRequestGroup(ctx context.Context, account *Account) bool {
	if group, ok := ctx.Value(ctxkey.Group).(*Group); ok && group != nil {
		return group.Platform == PlatformAnthropic && s.isAccountInGroup(account, &group.ID)
	}
	return s.isAccountInGroup(account, nil)
}

// API-only admission must not trust an eventually consistent scheduler snapshot.
func (s *GatewayService) revalidateAnthropicAPIAccount(ctx context.Context, selected *Account) (*Account, error) {
	if !allowsAnthropicAPIAccount(ctx, selected) {
		return nil, errors.New("anthropic API-only authentication policy denied account")
	}
	if !IsAnthropicAPIOnly(ctx) {
		return selected, nil
	}
	if s.accountRepo == nil {
		return nil, errors.New("anthropic API-only account validation unavailable")
	}
	current, err := s.accountRepo.GetByID(ctx, selected.ID)
	if err != nil {
		return nil, fmt.Errorf("anthropic API-only account validation: %w", err)
	}
	if !allowsAnthropicAPIAccount(ctx, current) || current.ID != selected.ID || !current.IsSchedulable() || !s.anthropicAPIAccountInRequestGroup(ctx, current) {
		return nil, errors.New("anthropic API-only authentication policy denied current account")
	}
	// A configuration change requires a new selection, never mixed URL/model/key revisions.
	if !reflect.DeepEqual(current.Credentials, selected.Credentials) ||
		current.GetAnthropicAPIKeyAuthScheme() != selected.GetAnthropicAPIKeyAuthScheme() ||
		!reflect.DeepEqual(current.ProxyID, selected.ProxyID) ||
		(current.Proxy == nil) != (selected.Proxy == nil) ||
		(current.Proxy != nil && current.Proxy.URL() != selected.Proxy.URL()) ||
		current.Concurrency != selected.Concurrency {
		return nil, errors.New("anthropic API-only account changed during routing")
	}
	return current, nil
}

// ScopeAnthropicAPISession isolates API-only affinity by authenticated downstream key.
func ScopeAnthropicAPISession(ctx context.Context, hash string, apiKeyID int64) string {
	if IsAnthropicAPIOnly(ctx) && hash != "" {
		if apiKeyID <= 0 {
			return ""
		}
		return fmt.Sprintf("anthropic-api:%d:%x", apiKeyID, sha256.Sum256([]byte(hash)))
	}
	if strings.HasPrefix(hash, "anthropic-api:") {
		return "legacy:" + hash
	}
	return hash
}
