package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type gatewayCallerIsolationAPIKeyRepo struct {
	service.APIKeyRepository
	keys map[string]*service.APIKey
}

func (r *gatewayCallerIsolationAPIKeyRepo) GetByKeyForAuth(_ context.Context, key string) (*service.APIKey, error) {
	apiKey := r.keys[key]
	if apiKey == nil {
		return nil, service.ErrAPIKeyNotFound
	}
	clone := *apiKey
	return &clone, nil
}

func (r *gatewayCallerIsolationAPIKeyRepo) UpdateLastUsed(context.Context, int64, time.Time) error {
	return nil
}

type gatewayCallerIsolationAccountRepo struct {
	service.AccountRepository
	byGroup map[int64][]service.Account
	shared  []service.Account
}

func (r *gatewayCallerIsolationAccountRepo) ListModelAvailabilityCandidates(_ context.Context, groupID *int64, _ []string, _ bool) ([]service.Account, error) {
	if groupID == nil {
		return append([]service.Account(nil), r.shared...), nil
	}
	return append([]service.Account(nil), r.byGroup[*groupID]...), nil
}

func gatewayCallerIsolationAccount(id int64, publicModel, upstreamModel string, modalities []string) service.Account {
	account := gatewayPreflightRouteAccount(id, publicModel, upstreamModel, modalities)
	account.Name = fmt.Sprintf("caller-isolation-account-%d", id)
	return account
}

func newGatewayCallerIsolationRouter(t *testing.T, runMode string) (*gin.Engine, map[string]string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	groupA := &service.Group{
		ID: 42, Status: service.StatusActive, Hydrated: true, Platform: service.PlatformAnthropic,
		ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"caller-a-text"}},
	}
	groupB := &service.Group{
		ID: 43, Status: service.StatusActive, Hydrated: true, Platform: service.PlatformAnthropic,
		ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"caller-b-image"}},
	}
	userA := &service.User{ID: 7, Role: service.RoleUser, Status: service.StatusActive, Balance: 10}
	userB := &service.User{ID: 8, Role: service.RoleUser, Status: service.StatusActive, Balance: 10}
	keyA := &service.APIKey{ID: 100, UserID: userA.ID, Key: "caller-isolation-a", Status: service.StatusActive, User: userA, GroupID: &groupA.ID, Group: groupA}
	keyB := &service.APIKey{ID: 101, UserID: userB.ID, Key: "caller-isolation-b", Status: service.StatusActive, User: userB, GroupID: &groupB.ID, Group: groupB}
	accountsA := []service.Account{
		gatewayCallerIsolationAccount(1, "caller-a-text", "a-upstream-1", []string{"text"}),
		gatewayCallerIsolationAccount(2, "caller-a-text", "a-upstream-2", []string{"text"}),
	}
	accountsB := []service.Account{
		gatewayCallerIsolationAccount(3, "caller-b-image", "b-upstream", []string{"text", "image"}),
	}
	cfg := &config.Config{
		RunMode:    runMode,
		Gateway:    config.GatewayConfig{MaxBodySize: 1 << 20, TextMaxBodySize: 1 << 20},
		APIKeyAuth: config.APIKeyAuthCacheConfig{L1Size: 128, L1TTLSeconds: 60},
	}
	apiKeyService := service.NewAPIKeyService(&gatewayCallerIsolationAPIKeyRepo{keys: map[string]*service.APIKey{
		keyA.Key: keyA,
		keyB.Key: keyB,
	}}, nil, nil, nil, nil, nil, cfg)
	accountRepo := &gatewayCallerIsolationAccountRepo{
		byGroup: map[int64][]service.Account{groupA.ID: accountsA, groupB.ID: accountsB},
		shared:  append(append([]service.Account(nil), accountsA...), accountsB...),
	}
	gatewayService := service.NewGatewayService(
		accountRepo, nil, nil, nil, nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	gatewayHandler := handler.NewGatewayHandler(gatewayService, nil, nil, nil, nil, nil, nil, nil, apiKeyService, nil, nil, nil, nil, cfg, nil)
	router := gin.New()
	RegisterGatewayRoutes(router, &handler.Handlers{Gateway: gatewayHandler, OpenAIGateway: &handler.OpenAIGatewayHandler{}},
		servermiddleware.NewAPIKeyAuthMiddleware(apiKeyService, nil, cfg), apiKeyService, nil, nil, nil, nil, cfg)
	return router, map[string]string{"a": keyA.Key, "b": keyB.Key}
}

func gatewayCallerIsolationRequest(router http.Handler, method, path, key, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+key)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func gatewayCallerIsolationModelIDs(t *testing.T, body []byte, field string) []string {
	t.Helper()
	var response struct {
		Models []struct {
			ID string `json:"id"`
		} `json:"models"`
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &response))
	if field == "data" {
		ids := make([]string, 0, len(response.Data))
		for _, model := range response.Data {
			ids = append(ids, model.ID)
		}
		return ids
	}
	ids := make([]string, 0, len(response.Models))
	for _, model := range response.Models {
		ids = append(ids, model.ID)
	}
	return ids
}

func TestGatewayCallerIsolationContracts(t *testing.T) {
	for _, runMode := range []string{config.RunModeStandard, config.RunModeSimple} {
		t.Run(runMode, func(t *testing.T) {
			router, keys := newGatewayCallerIsolationRouter(t, runMode)
			for caller, expected := range map[string]struct{ want, denied string }{
				"a": {want: "caller-a-text", denied: "caller-b-image"},
				"b": {want: "caller-b-image", denied: "caller-a-text"},
			} {
				t.Run("caller_"+caller, func(t *testing.T) {
					want, denied := expected.want, expected.denied
					models := gatewayCallerIsolationRequest(router, http.MethodGet, "/v1/models", keys[caller], "")
					require.Equal(t, http.StatusOK, models.Code, models.Body.String())
					require.Equal(t, []string{want}, gatewayCallerIsolationModelIDs(t, models.Body.Bytes(), "data"))

					v1 := gatewayCallerIsolationRequest(router, http.MethodGet, "/v1/gateway/capabilities", keys[caller], "")
					require.Equal(t, http.StatusOK, v1.Code, v1.Body.String())
					require.Equal(t, []string{want}, gatewayCallerIsolationModelIDs(t, v1.Body.Bytes(), "models"))
					require.NotContains(t, v1.Body.String(), denied)

					v2 := gatewayCallerIsolationRequest(router, http.MethodGet, "/v1/gateway/capabilities?schema_version=2", keys[caller], "")
					require.Equal(t, http.StatusOK, v2.Code, v2.Body.String())
					require.Equal(t, []string{want}, gatewayCallerIsolationModelIDs(t, v2.Body.Bytes(), "models"))
					require.NotContains(t, v2.Body.String(), denied)

					allowed := gatewayCallerIsolationRequest(router, http.MethodPost, "/v1/gateway/preflight", keys[caller], fmt.Sprintf(`{"schema_version":2,"model":%q,"protocol":"responses"}`, want))
					require.Equal(t, http.StatusOK, allowed.Code, allowed.Body.String())
					var preflight service.GatewayPreflightResponse
					require.NoError(t, json.Unmarshal(allowed.Body.Bytes(), &preflight))
					require.Equal(t, "configured", preflight.Routing.State)
					if caller == "a" {
						require.Equal(t, "multiple", preflight.Decision.CandidateRoutes)
					} else {
						require.Equal(t, "single", preflight.Decision.CandidateRoutes)
					}

					unsupported := gatewayCallerIsolationRequest(router, http.MethodPost, "/v1/gateway/preflight", keys[caller], fmt.Sprintf(`{"schema_version":2,"model":%q,"protocol":"responses","input_modalities":["image"]}`, want))
					require.Equal(t, http.StatusOK, unsupported.Code, unsupported.Body.String())
					require.NoError(t, json.Unmarshal(unsupported.Body.Bytes(), &preflight))
					if caller == "a" {
						require.Equal(t, "unknown", preflight.Support.State)
						require.Equal(t, "UNKNOWN", preflight.Support.Reason)
					} else {
						require.Equal(t, "supported", preflight.Support.State)
					}

					blocked := gatewayCallerIsolationRequest(router, http.MethodPost, "/v1/gateway/preflight", keys[caller], fmt.Sprintf(`{"schema_version":2,"model":%q,"protocol":"responses"}`, denied))
					require.Equal(t, http.StatusOK, blocked.Code, blocked.Body.String())
					require.NoError(t, json.Unmarshal(blocked.Body.Bytes(), &preflight))
					require.Equal(t, "restricted", preflight.Routing.State)
					require.Equal(t, "MODEL_NOT_ALLOWED", preflight.Routing.Reason)

					routeBlocked := gatewayCallerIsolationRequest(router, http.MethodPost, "/v1/messages", keys[caller], fmt.Sprintf(`{"model":%q}`, denied))
					require.Equal(t, http.StatusNotFound, routeBlocked.Code, routeBlocked.Body.String())
				})
			}
		})
	}
}

func TestGatewayCallerIsolationConcurrentRequests(t *testing.T) {
	router, keys := newGatewayCallerIsolationRouter(t, config.RunModeSimple)
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wait sync.WaitGroup
	for caller, want := range map[string]string{"a": "caller-a-text", "b": "caller-b-image"} {
		wait.Add(1)
		go func(caller, want string) {
			defer wait.Done()
			<-start
			for i := 0; i < 20; i++ {
				for _, path := range []string{"/v1/models", "/v1/gateway/capabilities", "/v1/gateway/capabilities?schema_version=2"} {
					response := gatewayCallerIsolationRequest(router, http.MethodGet, path, keys[caller], "")
					if response.Code != http.StatusOK {
						errs <- fmt.Errorf("%s %s: status %d: %s", caller, path, response.Code, response.Body.String())
						return
					}
					field := "models"
					if path == "/v1/models" {
						field = "data"
					}
					var decoded struct {
						Models []struct {
							ID string `json:"id"`
						} `json:"models"`
						Data []struct {
							ID string `json:"id"`
						} `json:"data"`
					}
					if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
						errs <- err
						return
					}
					var ids []string
					if field == "data" {
						for _, model := range decoded.Data {
							ids = append(ids, model.ID)
						}
					} else {
						for _, model := range decoded.Models {
							ids = append(ids, model.ID)
						}
					}
					if len(ids) != 1 || ids[0] != want {
						errs <- fmt.Errorf("%s %s: got models %v, want [%s]", caller, path, ids, want)
						return
					}
				}
				response := gatewayCallerIsolationRequest(router, http.MethodPost, "/v1/gateway/preflight", keys[caller], fmt.Sprintf(`{"schema_version":2,"model":%q,"protocol":"responses"}`, want))
				if response.Code != http.StatusOK {
					errs <- fmt.Errorf("%s preflight: status %d: %s", caller, response.Code, response.Body.String())
					return
				}
				var preflight service.GatewayPreflightResponse
				if err := json.Unmarshal(response.Body.Bytes(), &preflight); err != nil {
					errs <- err
					return
				}
				if preflight.Model != want || preflight.Routing.State != "configured" {
					errs <- fmt.Errorf("%s preflight: got model %q routing %q", caller, preflight.Model, preflight.Routing.State)
					return
				}
			}
		}(caller, want)
	}
	close(start)
	wait.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
}
