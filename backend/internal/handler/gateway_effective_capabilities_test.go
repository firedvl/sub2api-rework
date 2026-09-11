package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type countingGatewayEffectiveCapabilitiesRepo struct {
	gatewayModelsAccountRepoStub
	candidateCalls int
}

func (r *countingGatewayEffectiveCapabilitiesRepo) ListModelAvailabilityCandidates(ctx context.Context, groupID *int64, platforms []string, includeGrouped bool) ([]service.Account, error) {
	r.candidateCalls++
	return r.gatewayModelsAccountRepoStub.ListModelAvailabilityCandidates(ctx, groupID, platforms, includeGrouped)
}

func gatewayEffectiveCapabilityTestAccount(id int64, name, publicModel, upstreamModel string, modalities []string) service.Account {
	account := service.Account{
		ID: id, Name: name, Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth,
		Status: service.StatusActive, Schedulable: true,
		Credentials: map[string]any{
			"access_token":  "test-secret-" + name,
			"model_mapping": map[string]any{publicModel: upstreamModel},
		},
		Extra: map[string]any{},
	}
	account.SetUpstreamModelInventorySnapshot(service.UpstreamModelInventorySnapshot{Source: "upstream", Models: []string{upstreamModel}})
	account.SetUpstreamModelMetadataSnapshot(service.UpstreamModelMetadataSnapshot{Source: "upstream", Models: map[string]service.UpstreamModelMetadata{
		upstreamModel: {ID: upstreamModel, InputModalities: modalities},
	}})
	return account
}

func newGatewayEffectiveCapabilitiesHandler() (*GatewayHandler, *countingGatewayEffectiveCapabilitiesRepo) {
	shared := "partner-shared-model"
	repo := &countingGatewayEffectiveCapabilitiesRepo{gatewayModelsAccountRepoStub: gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
		42: {
			gatewayEffectiveCapabilityTestAccount(101, "private-account-a", shared, "upstream-alpha-71", []string{"text", "image"}),
			gatewayEffectiveCapabilityTestAccount(102, "private-account-a-only", "partner-a-only", "upstream-alpha-72", []string{"text"}),
		},
		43: {
			gatewayEffectiveCapabilityTestAccount(201, "private-account-b", shared, "upstream-bravo-81", []string{"text"}),
			gatewayEffectiveCapabilityTestAccount(202, "private-account-b-only", "partner-b-only", "upstream-bravo-82", []string{"text", "image"}),
		},
	}}}
	return newGatewayModelsHandlerForTest(repo), repo
}

func gatewayEffectiveTestContext(method, target, body string, group *service.Group) (*gin.Context, *httptest.ResponseRecorder) {
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{Group: group, GroupID: &group.ID})
	return c, response
}

func TestGatewayEffectiveCapabilitiesRemainGroupScoped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, repo := newGatewayEffectiveCapabilitiesHandler()
	shared := "partner-shared-model"
	groupA := &service.Group{ID: 42, Platform: service.PlatformAnthropic, ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{shared, "partner-a-only"}}}
	groupB := &service.Group{ID: 43, Platform: service.PlatformAnthropic, ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{shared, "partner-b-only"}}}

	request := func(group *service.Group, target string) *httptest.ResponseRecorder {
		c, response := gatewayEffectiveTestContext(http.MethodGet, target, "", group)
		c.Request.Header.Set("If-None-Match", `"other-group-response"`)
		h.Capabilities(c)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
		require.Empty(t, response.Header().Get("ETag"))
		for _, private := range []string{"test-secret-", "private-account", "upstream-", "credentials", "model_mapping"} {
			require.NotContains(t, response.Body.String(), private)
		}
		return response
	}

	defaultV1 := request(groupA, "/v1/gateway/capabilities")
	var v1 struct {
		SchemaVersion int `json:"schema_version"`
		Models        []struct {
			ID string `json:"id"`
		} `json:"models"`
	}
	require.NoError(t, json.Unmarshal(defaultV1.Body.Bytes(), &v1))
	require.Equal(t, 1, v1.SchemaVersion)
	require.Len(t, v1.Models, 2)
	require.ElementsMatch(t, []string{shared, "partner-a-only"}, []string{v1.Models[0].ID, v1.Models[1].ID})

	features := func(body []byte, id string) string {
		var result struct {
			SchemaVersion int `json:"schema_version"`
			Models        []struct {
				ID        string `json:"id"`
				Protocols map[string]struct {
					Capabilities struct {
						Features map[string]string `json:"features"`
					} `json:"capabilities"`
				} `json:"protocols"`
			} `json:"models"`
		}
		require.NoError(t, json.Unmarshal(body, &result))
		require.Equal(t, 2, result.SchemaVersion)
		for _, model := range result.Models {
			if model.ID == id {
				return model.Protocols[service.CompositeRouteEndpointResponses].Capabilities.Features["image_input"]
			}
		}
		require.Failf(t, "missing model", "%s", id)
		return ""
	}

	aV2 := request(groupA, "/v1/gateway/capabilities?schema_version=2")
	bV2 := request(groupB, "/v1/gateway/capabilities?schema_version=2")
	require.Equal(t, "supported", features(aV2.Body.Bytes(), shared))
	require.Equal(t, "unknown", features(bV2.Body.Bytes(), shared))
	require.NotContains(t, aV2.Body.String(), "partner-b-only")
	require.NotContains(t, bV2.Body.String(), "partner-a-only")

	displayOnlyA := *groupA
	displayOnlyA.ModelsListConfig = service.GroupModelsListConfig{Enabled: true, Models: []string{"partner-a-only"}}
	require.NotContains(t, request(&displayOnlyA, "/v1/gateway/capabilities?schema_version=2").Body.String(), shared)
	c, displayPreflight := gatewayEffectiveTestContext(http.MethodPost, "/v1/gateway/preflight", `{"schema_version":2,"model":"partner-shared-model","protocol":"responses","input_modalities":["image"]}`, &displayOnlyA)
	h.Preflight(c)
	require.Equal(t, http.StatusOK, displayPreflight.Code)
	require.Contains(t, displayPreflight.Body.String(), `"state":"supported"`, "display filtering controls publication only")

	deniedA := *groupA
	deniedA.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"partner-a-only"}}
	require.NotContains(t, request(&deniedA, "/v1/gateway/capabilities?schema_version=1").Body.String(), shared)
	require.Contains(t, request(groupB, "/v1/gateway/capabilities?schema_version=1").Body.String(), shared)

	repo.candidateCalls = 0
	c, response := gatewayEffectiveTestContext(http.MethodPost, "/v1/gateway/preflight", `{"schema_version":2,"model":"partner-b-only","protocol":"responses"}`, &deniedA)
	h.Preflight(c)
	require.Equal(t, http.StatusOK, response.Code)
	require.Zero(t, repo.candidateCalls, "allowlist denial must not inspect upstream accounts")
	require.Contains(t, response.Body.String(), "MODEL_NOT_ALLOWED")
}

func TestGatewayPreflightStrictRequestContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := newGatewayEffectiveCapabilitiesHandler()
	group := &service.Group{ID: 42, Platform: service.PlatformAnthropic, ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"partner-shared-model"}}}

	for _, tc := range []struct {
		name   string
		body   string
		status int
	}{
		{name: "missing schema version", body: `{"model":"partner-shared-model","protocol":"responses"}`, status: http.StatusBadRequest},
		{name: "unsupported schema version", body: `{"schema_version":1,"model":"partner-shared-model","protocol":"responses"}`, status: http.StatusBadRequest},
		{name: "unknown field", body: `{"schema_version":2,"model":"partner-shared-model","protocol":"responses","prompt":"never accepted"}`, status: http.StatusBadRequest},
		{name: "trailing json", body: `{"schema_version":2,"model":"partner-shared-model","protocol":"responses"} {}`, status: http.StatusBadRequest},
		{name: "invalid json", body: `{`, status: http.StatusBadRequest},
		{name: "unsupported protocol", body: `{"schema_version":2,"model":"partner-shared-model","protocol":"embeddings"}`, status: http.StatusBadRequest},
		{name: "unsupported tool", body: `{"schema_version":2,"model":"partner-shared-model","protocol":"responses","tools":["computer_use"]}`, status: http.StatusBadRequest},
		{name: "unsupported modality", body: `{"schema_version":2,"model":"partner-shared-model","protocol":"responses","input_modalities":["document"]}`, status: http.StatusBadRequest},
		{name: "trailing body too large", body: `{"schema_version":2,"model":"partner-shared-model","protocol":"responses"}` + strings.Repeat(" ", 8192), status: http.StatusRequestEntityTooLarge},
		{name: "body too large", body: `{"schema_version":2,"model":"partner-shared-model","protocol":"responses","tools":["` + strings.Repeat("x", 8192) + `"]}`, status: http.StatusRequestEntityTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, response := gatewayEffectiveTestContext(http.MethodPost, "/v1/gateway/preflight", tc.body, group)
			h.Preflight(c)
			require.Equal(t, tc.status, response.Code)
			require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
		})
	}

	c, response := gatewayEffectiveTestContext(http.MethodPost, "/v1/gateway/preflight", `{"schema_version":2,"model":"partner-hidden-model","protocol":"responses"}`, group)
	h.Preflight(c)
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	require.NotContains(t, response.Body.String(), "test-secret-")
	var result struct {
		SchemaVersion int    `json:"schema_version"`
		Model         string `json:"model"`
		Routing       struct {
			State  string `json:"state"`
			Reason string `json:"reason"`
		} `json:"routing"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
	require.Equal(t, 2, result.SchemaVersion)
	require.Equal(t, "partner-hidden-model", result.Model)
	require.Equal(t, "restricted", result.Routing.State)
	require.Equal(t, "MODEL_NOT_ALLOWED", result.Routing.Reason)
}
