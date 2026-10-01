package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/gemini"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type failedGeminiCatalogRepo struct{ service.AccountRepository }

func (*failedGeminiCatalogRepo) ListModelAvailabilityCandidates(context.Context, *int64, []string, bool) ([]service.Account, error) {
	return nil, errors.New("private repository diagnostic")
}

func TestGeminiNativeModelsCatalogFailureIsExplicit(t *testing.T) {
	handler := newGatewayModelsHandlerForTest(&failedGeminiCatalogRepo{})
	recorder := httptest.NewRecorder()
	requestContext, _ := gin.CreateTestContext(recorder)
	requestContext.Request = httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)
	groupID := int64(44)
	requestContext.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{GroupID: &groupID,
		Group: &service.Group{ID: groupID, Platform: service.PlatformGemini}})
	handler.GeminiV1BetaListModels(requestContext)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Unable to list Antigravity models")
	require.NotContains(t, recorder.Body.String(), "private repository diagnostic")
}

func TestAppendUpstreamGeminiModelsPreservesMetadata(t *testing.T) {
	body := []byte(`{"models":[{"name":"models/gemini-native","inputTokenLimit":123,"custom":{"a":true}}],"nextPageToken":"next","other":42}`)
	extra := []gemini.Model{gemini.FallbackModel("gemini-native"), gemini.FallbackModel("gemini-synced-custom"), gemini.FallbackModel("gemini-synced-custom")}
	merged, err := appendUpstreamGeminiModels(body, extra)
	require.NoError(t, err)
	require.JSONEq(t, `{"models":[{"name":"models/gemini-native","inputTokenLimit":123,"custom":{"a":true}},{"name":"models/gemini-synced-custom","supportedGenerationMethods":["generateContent","streamGenerateContent"]}],"nextPageToken":"next","other":42}`, string(merged))
	unchanged, err := appendUpstreamGeminiModels(body, extra[:1])
	require.NoError(t, err)
	require.Equal(t, body, unchanged)
	filtered, dropped, ok := filterUpstreamGeminiModelsBody(merged, service.GroupModelAllowlist{Enabled: true, Models: []string{"gemini-synced-*"}})
	require.True(t, ok)
	require.True(t, dropped)
	require.NotContains(t, string(filtered), "gemini-native")
	require.Contains(t, string(filtered), "gemini-synced-custom")
	for _, invalid := range []string{`null`, `not-json`, `{"error":"bad"}`, `{"models":{}}`} {
		_, err := appendUpstreamGeminiModels([]byte(invalid), extra)
		require.Error(t, err)
	}
}

func TestGeminiNativeModelsUsesAccountMappings(t *testing.T) {
	for _, scenario := range []struct {
		name                     string
		forced, mixed, allowlist bool
		status                   int
	}{
		{"mixed", false, true, false, 200},
		{"mixed allowlist", false, true, true, 200},
		{"disabled mixed", false, false, false, 503},
		{"forced without mixed opt-in", true, false, false, 200},
		{"forced allowlist", true, false, true, 200},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			groupID := int64(45)
			repo := &geminiAllowlistAccountRepoStub{gatewayModelsAccountRepoStub: gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
				groupID: {{ID: 1, Platform: service.PlatformAntigravity,
					Extra:       map[string]any{"mixed_scheduling": scenario.mixed},
					Credentials: map[string]any{"model_mapping": map[string]any{"gemini-synced-custom": "gemini-3.8-flash-high", "claude-custom": "claude-sonnet-4-6"}}}},
			}}}
			h := newGatewayModelsHandlerForTest(repo)
			h.geminiCompatService = service.NewGeminiMessagesCompatService(repo, nil, nil, nil, nil, nil, nil, nil, nil)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)
			c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: service.PlatformGemini,
				ModelAllowlist: service.GroupModelAllowlist{Enabled: scenario.allowlist, Models: []string{"gemini-synced-custom"}},
			}})
			if scenario.forced {
				c.Set(string(middleware.ContextKeyForcePlatform), service.PlatformAntigravity)
			}
			h.GeminiV1BetaListModels(c)
			require.Equal(t, scenario.status, rec.Code, rec.Body.String())
			if scenario.status != 200 {
				return
			}
			var got gemini.ModelsListResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
			names := []string{}
			for _, model := range got.Models {
				names = append(names, model.Name)
				require.Contains(t, model.SupportedGenerationMethods, "generateContent")
			}
			require.Contains(t, names, "models/gemini-synced-custom")
			require.NotContains(t, names, "models/claude-custom")
			require.NotContains(t, names, "models/gemini-2.0-flash")
			if scenario.allowlist {
				require.Equal(t, []string{"models/gemini-synced-custom"}, names)
			}
		})
	}
}

type geminiMixedModelsUpstream struct {
	service.HTTPUpstream
	status int
	body   string
}

func (u *geminiMixedModelsUpstream) Do(_ *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	return &http.Response{StatusCode: u.status, Header: http.Header{"Content-Type": {"application/json"}, "X-Request-Id": {"native-id"}}, Body: io.NopCloser(strings.NewReader(u.body))}, nil
}
func TestGeminiNativeModelsMergesNativeUpstream(t *testing.T) {
	for _, scenario := range []struct {
		name           string
		status         int
		body           string
		expectedNative string
	}{
		{"native", 200, `{"models":[{"name":"models/gemini-native","inputTokenLimit":123}],"nextPageToken":"next"}`, "models/gemini-native"},
		{"scope fallback", 403, `{"error":"insufficient authentication scopes"}`, "models/gemini-2.5-pro"},
		{"upstream error", 429, `{"error":"rate limited"}`, ""},
		{"malformed successful upstream", 200, `{"models":{}}`, ""},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			id := int64(46)
			repo := &geminiAllowlistAccountRepoStub{gatewayModelsAccountRepoStub: gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{id: {
				{ID: 1, Platform: service.PlatformGemini, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"}},
				{ID: 2, Platform: service.PlatformAntigravity, Extra: map[string]any{"mixed_scheduling": true}, Credentials: map[string]any{"model_mapping": map[string]any{"gemini-synced-custom": "gemini-3.8-flash-high"}}},
			}}}}
			upstream := &geminiMixedModelsUpstream{status: scenario.status, body: scenario.body}
			h := newGatewayModelsHandlerForTest(repo)
			h.geminiCompatService = service.NewGeminiMessagesCompatService(repo, nil, nil, nil, nil, nil, upstream, nil, &config.Config{})
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)
			c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{GroupID: &id, Group: &service.Group{ID: id, Platform: service.PlatformGemini}})
			h.GeminiV1BetaListModels(c)
			if scenario.name == "malformed successful upstream" {
				require.Equal(t, http.StatusBadGateway, rec.Code)
				require.Contains(t, rec.Body.String(), "Unable to merge upstream Gemini models")
				return
			}
			if scenario.status == 429 {
				require.Equal(t, 429, rec.Code)
				require.JSONEq(t, scenario.body, rec.Body.String())
				return
			}
			require.Equal(t, 200, rec.Code, rec.Body.String())
			require.Contains(t, rec.Body.String(), scenario.expectedNative)
			require.Contains(t, rec.Body.String(), "models/gemini-synced-custom")
			if scenario.status == 200 {
				require.Contains(t, rec.Body.String(), `"inputTokenLimit":123`)
				require.Contains(t, rec.Body.String(), `"nextPageToken":"next"`)
				require.Equal(t, "native-id", rec.Header().Get("X-Request-Id"))
			}
		})
	}
}
