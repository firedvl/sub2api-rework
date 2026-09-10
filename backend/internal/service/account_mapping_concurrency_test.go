package service

import (
	"fmt"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountMappingAndHeadersConcurrentReaders(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"model_mapping":           map[string]any{"public": "upstream"},
		"header_override_enabled": true, "header_overrides": map[string]any{"x-route": "fixture"},
	}}
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			for range 100 {
				mapping := account.GetModelMapping()
				assert.Equal(t, "upstream", mapping["public"])
				mapping["public"] = "caller-local"
				headers := account.GetHeaderOverrides()
				assert.Equal(t, "fixture", headers["x-route"])
				headers["x-route"] = "caller-local"
			}
		})
	}
	wg.Wait()
}

func BenchmarkAccountMappingResolution(b *testing.B) {
	for _, size := range []int{0, 4, 32} {
		raw := make(map[string]any, size)
		for i := range size {
			raw[fmt.Sprintf("public-%d", i)] = fmt.Sprintf("upstream-%d", i)
		}
		account := &Account{Platform: PlatformOpenAI, Credentials: map[string]any{"model_mapping": raw}}
		b.Run(fmt.Sprintf("current-%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = account.GetModelMapping()
			}
		})
	}
}

func TestAccountMappingRuntimeOptionsConcurrent(t *testing.T) {
	previous := xai.RuntimeModelMappingOptions()
	t.Cleanup(func() { xai.SetRuntimeModelMappingOptions(previous) })
	account := &Account{Platform: PlatformGrok}
	var wg sync.WaitGroup
	wg.Go(func() {
		for range 100 {
			xai.SetRuntimeModelMappingOptions(xai.ModelMappingOptions{EnableCrossClientMap: true})
			xai.SetRuntimeModelMappingOptions(xai.ModelMappingOptions{})
		}
	})
	for range 8 {
		wg.Go(func() {
			for range 100 {
				assert.NotEmpty(t, account.GetModelMapping())
			}
		})
	}
	wg.Wait()
	xai.SetRuntimeModelMappingOptions(xai.ModelMappingOptions{EnableCrossClientMap: true})
	want := xai.DefaultModelMapping()
	require.Equal(t, want, account.GetModelMapping())
	xai.SetRuntimeModelMappingOptions(xai.ModelMappingOptions{})
	require.Equal(t, xai.DefaultModelMapping(), account.GetModelMapping())
}

func TestAccountMappingCopyAndInventoryReplacement(t *testing.T) {
	account := Account{Platform: PlatformGemini}
	account.SetUpstreamModelInventorySnapshot(UpstreamModelInventorySnapshot{Models: []string{"gemini-first"}})
	first := account.GetModelMapping()
	copy := account
	copy.Credentials = map[string]any{"model_mapping": map[string]any{"public": "gemini-target"}}
	copy.Extra = nil
	require.Equal(t, map[string]string{"public": "gemini-target"}, copy.GetModelMapping())
	account.SetUpstreamModelInventorySnapshot(UpstreamModelInventorySnapshot{Models: []string{"gemini-second"}})
	require.Equal(t, map[string]string{"gemini-second": "gemini-second"}, account.GetModelMapping())
	require.Equal(t, map[string]string{"gemini-first": "gemini-first"}, first)
	for _, platform := range []string{PlatformAntigravity, PlatformGrok, PlatformOpenAI} {
		a := &Account{Platform: platform, Credentials: map[string]any{"model_mapping": map[string]any{"public": "target"}}}
		got := a.GetModelMapping()
		got["public"] = "mutated"
		require.Equal(t, "target", a.GetModelMapping()["public"])
	}
	defaultAccount := &Account{Platform: PlatformAntigravity}
	baseline := defaultAccount.GetModelMapping()
	for key := range defaultAccount.GetModelMapping() {
		mutated := defaultAccount.GetModelMapping()
		delete(mutated, key)
		break
	}
	require.Equal(t, baseline, defaultAccount.GetModelMapping())
}

func TestAccountHeaderOverridesObserveReplacementAndCopy(t *testing.T) {
	raw := map[string]any{"x-route": "first"}
	account := Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"header_override_enabled": true, "header_overrides": raw,
	}}
	first := account.GetHeaderOverrides()
	raw["x-route"] = "second"
	require.Equal(t, "second", account.GetHeaderOverrides()["x-route"])
	copy := account
	copy.Credentials = map[string]any{"header_override_enabled": true, "header_overrides": map[string]any{"x-route": "copy"}}
	require.Equal(t, "copy", copy.GetHeaderOverrides()["x-route"])
	require.Equal(t, "second", account.GetHeaderOverrides()["x-route"])
	require.Equal(t, "first", first["x-route"])
	account.Credentials["header_overrides"] = map[string]any{"x-route": "replaced"}
	require.Equal(t, "replaced", account.GetHeaderOverrides()["x-route"])
}
