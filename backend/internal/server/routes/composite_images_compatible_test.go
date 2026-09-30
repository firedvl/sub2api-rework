//go:build unit

package routes

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type compatibleImagesAccounts struct {
	service.AccountRepository
	accounts []service.Account
}

func (repo compatibleImagesAccounts) GetByID(_ context.Context, id int64) (*service.Account, error) {
	for _, account := range repo.accounts {
		if account.ID == id {
			return &account, nil
		}
	}
	return nil, service.ErrNoAvailableAccounts
}

func (repo compatibleImagesAccounts) ListSchedulableByPlatform(context.Context, string) ([]service.Account, error) {
	return repo.accounts, nil
}

func (repo compatibleImagesAccounts) ListSchedulableUngroupedByPlatform(context.Context, string) ([]service.Account, error) {
	return repo.accounts, nil
}

func (repo compatibleImagesAccounts) ListSchedulableByGroupIDAndPlatform(context.Context, int64, string) ([]service.Account, error) {
	return repo.accounts, nil
}

func (repo compatibleImagesAccounts) ListModelAvailabilityCandidates(context.Context, *int64, []string, bool) ([]service.Account, error) {
	return repo.accounts, nil
}

type compatibleImagesUpstream struct {
	service.HTTPUpstream
	accountIDs []int64
	body       []byte
}

func (upstream *compatibleImagesUpstream) Do(req *http.Request, _ string, id int64, _ int) (*http.Response, error) {
	upstream.accountIDs = append(upstream.accountIDs, id)
	var err error
	upstream.body, err = io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}, "X-Request-Id": {"compatible-image-test"}}, Body: io.NopCloser(strings.NewReader(`{"data":[{"b64_json":"aW1hZ2U="}]}`))}, nil
}

type compatibleImagesUsage struct {
	service.UsageLogRepository
	logs []*service.UsageLog
}

func (repo *compatibleImagesUsage) Create(_ context.Context, log *service.UsageLog) (bool, error) {
	repo.logs = append(repo.logs, log)
	return true, nil
}

func TestCompositeCompatibleImagesEndToEnd(test *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, scenario := range []string{"generation", "json_edit", "multipart_alias", "disabled", "native_only", "restricted"} {
		test.Run(scenario, func(test *testing.T) {
			const model = "gemini-3.1-flash-image"
			groupID := int64(101)
			price := 0.17
			group := &service.Group{ID: groupID, Platform: service.PlatformComposite, AllowImageGeneration: scenario != "disabled", RateMultiplier: 1, ImagePrice1K: &price, ImagePrice2K: &price, ImagePrice4K: &price}
			accounts := []service.Account{}
			for index, typ := range []string{service.AccountTypeOAuth, service.AccountTypeSetupToken, service.AccountTypeAPIKey} {
				if scenario == "native_only" && typ == service.AccountTypeAPIKey {
					continue
				}
				mapping := map[string]any{model: model}
				if scenario == "restricted" && typ == service.AccountTypeAPIKey {
					mapping = map[string]any{"gpt-image-2": "gpt-image-2"}
				}
				accounts = append(accounts, service.Account{ID: int64(index + 1), Platform: service.PlatformOpenAI, Type: typ, Status: service.StatusActive, Schedulable: true, Priority: index, Credentials: map[string]any{"api_key": "fixture-key", "access_token": "unused-native-token", "base_url": "https://compatible.example/v1", "model_mapping": mapping}})
			}
			cfg := &config.Config{RunMode: config.RunModeSimple}
			repo := compatibleImagesAccounts{accounts: accounts}
			upstream, usage := &compatibleImagesUpstream{}, &compatibleImagesUsage{}
			billingCache := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
			test.Cleanup(billingCache.Stop)
			gateway := service.NewOpenAIGatewayService(repo, usage, nil, nil, nil, nil, nil, cfg, nil, nil, service.NewBillingService(cfg, nil), nil, billingCache, upstream, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil)
			imagesHandler := handler.NewOpenAIGatewayHandler(gateway, service.NewConcurrencyService(nil), billingCache, service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg), nil, nil, nil, nil, cfg)
			publicModel := model
			if scenario == "multipart_alias" {
				publicModel = "public-image"
			}
			resolver := service.NewCompositeRouteResolver(compositeRouteRepoStub{routes: []service.CompositeModelRoute{{ID: 1, GroupID: groupID, PublicModel: publicModel, MatchType: service.CompositeRouteMatchExact, TargetPlatform: service.PlatformOpenAI, UpstreamModel: model, Endpoint: service.CompositeRouteEndpointImages, Enabled: true}}})
			router := gin.New()
			router.Use(func(ginContext *gin.Context) {
				ginContext.Set(string(servermiddleware.ContextKeyAPIKey), &service.APIKey{ID: 202, GroupID: &groupID, Group: group, User: &service.User{ID: 303}})
				ginContext.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: 303})
				ginContext.Next()
			})
			router.Use(compositeTargetPlatformMiddleware(resolver))
			router.POST("/v1/images/generations", imagesHandler.Images)
			router.POST("/v1/images/edits", imagesHandler.Images)
			endpoint, contentType := "/v1/images/generations", "application/json"
			body := []byte(fmt.Sprintf(`{"model":%q,"prompt":"draw","size":"1024x1024"}`, publicModel))
			if scenario == "json_edit" {
				endpoint = "/v1/images/edits"
				body = []byte(fmt.Sprintf(`{"model":%q,"prompt":"draw","images":[{"image_url":"https://source.example/input.png"}]}`, publicModel))
			}
			if scenario == "multipart_alias" {
				endpoint = "/v1/images/edits"
				var buf bytes.Buffer
				writer := multipart.NewWriter(&buf)
				require.NoError(test, writer.WriteField("model", publicModel))
				require.NoError(test, writer.WriteField("prompt", "draw"))
				part, err := writer.CreateFormFile("image", "input.png")
				require.NoError(test, err)
				_, err = part.Write([]byte("fixture-image"))
				require.NoError(test, err)
				require.NoError(test, writer.Close())
				body, contentType = buf.Bytes(), writer.FormDataContentType()
			}
			req := httptest.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
			req.Header.Set("Content-Type", contentType)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if scenario == "disabled" || scenario == "native_only" || scenario == "restricted" {
				if scenario == "disabled" {
					require.Equal(test, http.StatusForbidden, rec.Code)
				} else {
					require.Equal(test, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
				}
				require.Empty(test, upstream.accountIDs)
				require.Empty(test, usage.logs)
				return
			}
			require.Equal(test, http.StatusOK, rec.Code, rec.Body.String())
			require.Equal(test, []int64{3}, upstream.accountIDs)
			require.Contains(test, string(upstream.body), model)
			require.Contains(test, rec.Body.String(), "aW1hZ2U=")
			require.Len(test, usage.logs, 1, "the image must reach usage recording, not just return HTTP 200")
			require.Equal(test, 1, usage.logs[0].ImageCount)
			require.InDelta(test, price, usage.logs[0].ActualCost, 1e-9)
			require.Equal(test, publicModel, usage.logs[0].RequestedModel)
		})
	}
}
