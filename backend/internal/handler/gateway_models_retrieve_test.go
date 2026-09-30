package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func requestModelForTest(gateway *GatewayHandler, group *service.Group, modelID, etag string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	ctx.Request.Header.Set("If-None-Match", etag)
	if modelID != "" {
		ctx.Params = gin.Params{{Key: "model", Value: modelID}}
	}
	ctx.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{GroupID: &group.ID, Group: group})
	gateway.Models(ctx)
	return rec
}

func TestRetrieveModelMatchesVisibleCatalogue(test *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, platform := range []string{service.PlatformOpenAI, service.PlatformAnthropic, service.PlatformGemini, service.PlatformGrok, service.PlatformComposite} {
		for _, mapped := range []bool{false, true} {
			name := platform + "/fallback"
			if mapped {
				name = platform + "/mapped"
			}
			test.Run(name, func(test *testing.T) {
				accountPlatform := platform
				if platform == service.PlatformComposite {
					accountPlatform = service.PlatformOpenAI
				}
				credentials := map[string]any{}
				if mapped {
					credentials["model_mapping"] = map[string]any{"custom-model": "upstream-only-model"}
				}
				group := &service.Group{ID: 71, Platform: platform}
				gateway := newGatewayModelsHandlerForTest(&gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
					group.ID: {{ID: 1, Platform: accountPlatform, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Credentials: credentials}},
				}})
				list := requestModelForTest(gateway, group, "", "")
				require.Equal(test, http.StatusOK, list.Code, list.Body.String())
				var catalog struct {
					Data []json.RawMessage `json:"data"`
				}
				require.NoError(test, json.Unmarshal(list.Body.Bytes(), &catalog))
				if mapped {
					require.NotEmpty(test, catalog.Data)
				}
				if len(catalog.Data) == 0 {
					require.Equal(test, http.StatusNotFound, requestModelForTest(gateway, group, "unknown-model", "").Code)
					return
				}
				var model struct {
					ID string `json:"id"`
				}
				require.NoError(test, json.Unmarshal(catalog.Data[0], &model))
				retrieved := requestModelForTest(gateway, group, model.ID, "")
				require.Equal(test, http.StatusOK, retrieved.Code, retrieved.Body.String())
				require.JSONEq(test, string(catalog.Data[0]), retrieved.Body.String())
				require.Equal(test, http.StatusNotFound, requestModelForTest(gateway, group, "unknown-model", "").Code)
				group.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"absent-from-source"}}
				hidden := requestModelForTest(gateway, group, model.ID, "")
				require.Equal(test, http.StatusNotFound, hidden.Code, hidden.Body.String())
				require.Contains(test, hidden.Body.String(), `"code":"model_not_found"`)
			})
		}
	}
}

func TestRetrievePinnedModelPreservesMetadataFilteringAndErrors(test *testing.T) {
	for _, scenario := range []struct {
		name       string
		status     int
		selected   []string
		wantStatus int
	}{
		{name: "metadata", wantStatus: http.StatusOK},
		{name: "hidden", selected: []string{"other-model"}, wantStatus: http.StatusNotFound},
		{name: "upstream failure", status: http.StatusServiceUnavailable, wantStatus: http.StatusBadGateway},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			upstream := &codexModelsPinnedHTTPUpstream{bodies: map[int64]string{
				2: `{"data":[{"id":"special-model","ID":"not-a-model-id","owned_by":"source-owner","created":123,"extra":{"context":999}}]}`,
			}, statuses: map[int64]int{}}
			if scenario.status != 0 {
				upstream.statuses[2] = scenario.status
			}
			codex := newPinnedCodexTestHandler([]service.Account{newPinnedCodexAccount(2, service.StatusActive, true, false)}, upstream, 3)
			gateway := &GatewayHandler{openAIGatewayService: codex.gatewayService, maxAccountSwitches: 3}
			group := &service.Group{ID: 72, Platform: service.PlatformOpenAI,
				ModelAllowlist:            service.GroupModelAllowlist{Enabled: len(scenario.selected) > 0, Models: scenario.selected},
				CodexModelsManifestConfig: service.GroupCodexModelsManifestConfig{Enabled: true, AccountIDs: []int64{2}}}
			got := requestModelForTest(gateway, group, "special-model", "collection-etag")
			require.Equal(test, scenario.wantStatus, got.Code, got.Body.String())
			if scenario.wantStatus == http.StatusOK {
				list := requestModelForTest(gateway, group, "", "")
				var catalog struct {
					Data []json.RawMessage `json:"data"`
				}
				require.NoError(test, json.Unmarshal(list.Body.Bytes(), &catalog))
				require.JSONEq(test, string(catalog.Data[0]), got.Body.String())
				require.Contains(test, got.Body.String(), `"owned_by":"source-owner"`)
				require.Contains(test, got.Body.String(), `"created":123`)
				again := requestModelForTest(gateway, group, "special-model", list.Header().Get("ETag"))
				require.Equal(test, http.StatusOK, again.Code)
				require.Empty(test, again.Header().Get("ETag"))
				require.Equal(test, http.StatusNotFound, requestModelForTest(gateway, group, "not-a-model-id", "").Code)
			}
		})
	}
}

func TestRetrieveModelRejectsMalformedCatalogue(test *testing.T) {
	for _, body := range []string{
		`{`,
		`{"data":["not a model"]}`,
		`{"data":[{"id":42}]}`,
		`{"data":[{"display_name":"missing ID"}]}`,
	} {
		test.Run(body, func(test *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Params = gin.Params{{Key: "model", Value: "wanted"}}
			writeRetrievedModel(ctx, []byte(body))
			require.Equal(test, http.StatusBadGateway, recorder.Code)
			require.Contains(test, recorder.Body.String(), `"type":"upstream_error"`)
		})
	}
}
