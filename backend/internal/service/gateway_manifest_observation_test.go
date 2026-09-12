package service

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type countedManifestSnapshots struct {
	body  []byte
	reads *atomic.Int64
}

func (s countedManifestSnapshots) MarshalJSON() ([]byte, error) { s.reads.Add(1); return s.body, nil }

func observationTestAccount(t *testing.T, now time.Time, identityMatches bool, versions map[string]openAICodexManifestSnapshot) (Account, *atomic.Int64) {
	t.Helper()
	account := gatewayCapabilityTestAccount(1, PlatformOpenAI, nil)
	account.Type = AccountTypeOAuth
	account.Extra[codexFingerprintSeedExtraKey] = "11111111-1111-4111-8111-111111111111"
	identity := openAICodexManifestIdentity(&account)
	if !identityMatches {
		identity = "different-identity"
	}
	raw, err := json.Marshal(openAICodexManifestSnapshots{Identity: identity, Versions: versions})
	require.NoError(t, err)
	reads := &atomic.Int64{}
	account.Extra[OpenAICodexManifestSnapshotExtraKey] = countedManifestSnapshots{raw, reads}
	return account, reads
}

func TestGatewayManifestObservationFreshnessAndIdentity(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	body := json.RawMessage(`{"models":[{"slug":"gpt-future-codex-model","input_modalities":["text"],"context_window":456789}]}`)
	for _, tc := range []struct {
		name     string
		age      time.Duration
		identity bool
		fresh    bool
	}{
		{"fresh", time.Hour, true, true},
		{"exact 24 hour boundary", 24 * time.Hour, true, true},
		{"expired by one nanosecond", 24*time.Hour + time.Nanosecond, true, false},
		{"identity mismatch", time.Hour, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account, reads := observationTestAccount(t, now, tc.identity, map[string]openAICodexManifestSnapshot{"v": {SyncedAt: now.Add(-tc.age).Format(time.RFC3339Nano), Body: body}})
			// Durable data still works when a manifest is expired or identity mismatched.
			account.SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{Models: map[string]UpstreamModelMetadata{"durable": {ID: "durable", ContextWindow: 99}}})
			gateway := &GatewayService{accountRepo: &gatewayCapabilityAccountRepoStub{configured: []Account{account}}}
			group := &Group{ID: 42, Platform: PlatformOpenAI}
			snapshot := gateway.loadGatewayEffectiveSnapshotAt(context.Background(), group, now)
			require.Equal(t, now, snapshot.observedAt)
			require.Equal(t, tc.fresh, snapshot.observedModels[account.ID]["gpt-future-codex-model"])
			_, hasMetadata := snapshot.metadata[account.ID]["gpt-future-codex-model"]
			require.Equal(t, tc.fresh, hasMetadata)
			require.EqualValues(t, 99, snapshot.metadata[account.ID]["durable"].ContextWindow)
			// Both consumers remain pinned even after the source disappears or time advances.
			delete(account.Extra, OpenAICodexManifestSnapshotExtraKey)
			ids := gateway.gatewayEffectiveModelIDs(context.Background(), group, snapshot)
			if tc.fresh {
				require.Contains(t, ids, "gpt-future-codex-model")
				require.EqualValues(t, 456789, snapshot.metadata[account.ID]["gpt-future-codex-model"].ContextWindow)
			} else {
				require.NotContains(t, ids, "gpt-future-codex-model")
			}
			require.EqualValues(t, 1, reads.Load(), "metadata, observations and publication must select once")
		})
	}
}

func TestGatewayManifestObservationSelectionAndFiltering(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	body := json.RawMessage(`{"models":[
 {"slug":"gpt-future-codex-model","input_modalities":["text"],"supports_search_tool":true},
 {"slug":"hidden-model","visibility":"hidden","context_window":111},
 {"slug":"api-disabled","supported_in_api":false,"context_window":222},
 {"slug":"gpt-image-2"},{"slug":"codex-auto-test"},{"slug":"wild-*"}
 ]}`)
	account, reads := observationTestAccount(t, now, true, map[string]openAICodexManifestSnapshot{
		"a": {SyncedAt: now.Format(time.RFC3339Nano), Body: body},
		"b": {SyncedAt: now.Format(time.RFC3339Nano), Body: json.RawMessage(`{"models":[{"slug":"other-body","context_window":999}]}`)},
	})
	account.SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{Models: map[string]UpstreamModelMetadata{
		"api-disabled": {ID: "api-disabled", CodexToolCapabilities: map[string]json.RawMessage{"supported_in_api": json.RawMessage("true")}},
	}})
	gateway := &GatewayService{accountRepo: &gatewayCapabilityAccountRepoStub{configured: []Account{account}}}
	group := &Group{ID: 42, Platform: PlatformOpenAI}
	snapshot := gateway.loadGatewayEffectiveSnapshotAt(context.Background(), group, now)
	require.Equal(t, []string{"gpt-future-codex-model"}, snapshot.manifestIDs[account.ID])
	require.NotContains(t, snapshot.metadata[account.ID], "other-body")
	require.EqualValues(t, 111, snapshot.metadata[account.ID]["hidden-model"].ContextWindow, "hidden affects publication, not stored metadata semantics")
	require.Equal(t, json.RawMessage("false"), snapshot.metadata[account.ID]["api-disabled"].CodexToolCapabilities["supported_in_api"])
	require.Equal(t, []string{"gpt-future-codex-model"}, gateway.gatewayEffectiveModelIDs(context.Background(), group, snapshot))
	require.EqualValues(t, 1, reads.Load())
	// Explicit mappings remain authoritative even though the observation retains the discovered ID.
	account.Credentials["model_mapping"] = map[string]any{"operator-alias": "gpt-future-codex-model"}
	require.Equal(t, []string{"operator-alias"}, gateway.gatewayEffectiveModelIDs(context.Background(), group, snapshot))
	require.EqualValues(t, 1, reads.Load())
}

func TestGatewayManifestObservationHasOneBodyDecoder(t *testing.T) {
	// Enforce the requested single-decode architecture as well as the behavioral
	// read-count tests: both consumers must derive fields from this parsed body.
	file, err := parser.ParseFile(token.NewFileSet(), "gateway_effective_features.go", nil, 0)
	require.NoError(t, err)
	decodes := 0
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "parseCodexManifestObservation" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if ok && pkg.Name == "decoder" && selector.Sel.Name == "Decode" {
				decodes++
			}
			return true
		})
	}
	require.Equal(t, 1, decodes)
	selector, err := parser.ParseFile(token.NewFileSet(), "openai_codex_models_service.go", nil, 0)
	require.NoError(t, err)
	for _, decl := range selector.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "observeOpenAICodexManifest" {
			continue
		}
		parses := 0
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			if name, ok := call.Fun.(*ast.Ident); ok {
				require.NotEqual(t, "validateCodexModelsManifestEnvelope", name.Name, "observation validates its own envelope without a second decoder")
				if name.Name == "parseCodexManifestObservation" {
					parses++
				}
			}
			return true
		})
		require.Equal(t, 1, parses)
	}

}

func TestGatewayManifestObservationMalformedNewestDoesNotSelectOlder(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	account, _ := observationTestAccount(t, now, true, map[string]openAICodexManifestSnapshot{
		"new": {SyncedAt: now.Format(time.RFC3339Nano), Body: json.RawMessage(`{"models":[{"slug":"new","context_window":123},42]}`)},
		"old": {SyncedAt: now.Add(-time.Hour).Format(time.RFC3339Nano), Body: json.RawMessage(`{"models":[{"slug":"old","context_window":456}]}`)},
	})
	observation := observeOpenAICodexManifest(&account, now)
	require.Empty(t, observation.publicIDs)
	require.EqualValues(t, 123, observation.metadata["new"].ContextWindow)
	require.NotContains(t, observation.metadata, "old")
}

func TestGatewayManifestObservationPreservesCaseFoldedEfforts(t *testing.T) {
	observation, ok := parseCodexManifestObservation([]byte(`{"models":[{"Slug":"future","Reasoning":true,"supported_reasoning_levels":[{"Effort":"ultra"},{"EFFORT":"low","effort":"high"}]}]}`))
	require.True(t, ok)
	require.Equal(t, []string{"future"}, observation.publicIDs)
	require.Equal(t, []string{"ultra", "high"}, observation.metadata["future"].SupportedReasoningLevels)
}
