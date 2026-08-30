package admin

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type dataResponse struct {
	Code int         `json:"code"`
	Data dataPayload `json:"data"`
}

type dataPayload struct {
	Type           string        `json:"type"`
	Version        int           `json:"version"`
	Proxies        []dataProxy   `json:"proxies"`
	Accounts       []dataAccount `json:"accounts"`
	SkippedShadows int           `json:"skipped_shadows"`
}

type dataProxy struct {
	ProxyKey string `json:"proxy_key"`
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Status   string `json:"status"`
}

type dataAccount struct {
	Name        string         `json:"name"`
	Platform    string         `json:"platform"`
	Type        string         `json:"type"`
	Credentials map[string]any `json:"credentials"`
	Extra       map[string]any `json:"extra"`
	ProxyKey    *string        `json:"proxy_key"`
	Concurrency int            `json:"concurrency"`
	Priority    int            `json:"priority"`
}

func setupAccountDataRouter() (*gin.Engine, *stubAdminService) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	adminSvc := newStubAdminService()

	h := NewAccountHandler(
		adminSvc,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	router.GET("/api/v1/admin/accounts/data", h.ExportData)
	router.POST("/api/v1/admin/accounts/data/preview", h.PreviewImportData)
	router.POST("/api/v1/admin/accounts/data", h.ImportData)
	return router, adminSvc
}

func TestExportDataIncludesSecrets(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()

	proxyID := int64(11)
	adminSvc.proxies = []service.Proxy{
		{
			ID:       proxyID,
			Name:     "proxy",
			Protocol: "http",
			Host:     "127.0.0.1",
			Port:     8080,
			Username: "user",
			Password: "pass",
			Status:   service.StatusActive,
		},
		{
			ID:       12,
			Name:     "orphan",
			Protocol: "https",
			Host:     "10.0.0.1",
			Port:     443,
			Username: "o",
			Password: "p",
			Status:   service.StatusActive,
		},
	}
	adminSvc.accounts = []service.Account{
		{
			ID:          21,
			Name:        "account",
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeOAuth,
			Credentials: map[string]any{"token": "secret"},
			Extra:       map[string]any{"note": "x"},
			ProxyID:     &proxyID,
			Concurrency: 3,
			Priority:    50,
			Status:      service.StatusDisabled,
		},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/data", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp dataResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Empty(t, resp.Data.Type)
	require.Equal(t, 0, resp.Data.Version)
	require.Len(t, resp.Data.Proxies, 1)
	require.Equal(t, "pass", resp.Data.Proxies[0].Password)
	require.Len(t, resp.Data.Accounts, 1)
	require.Equal(t, "secret", resp.Data.Accounts[0].Credentials["token"])
	require.NotContains(t, rec.Body.String(), "source_index")
}

func TestExportDataWithoutProxies(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()

	proxyID := int64(11)
	adminSvc.proxies = []service.Proxy{
		{
			ID:       proxyID,
			Name:     "proxy",
			Protocol: "http",
			Host:     "127.0.0.1",
			Port:     8080,
			Username: "user",
			Password: "pass",
			Status:   service.StatusActive,
		},
	}
	adminSvc.accounts = []service.Account{
		{
			ID:          21,
			Name:        "account",
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeOAuth,
			Credentials: map[string]any{"token": "secret"},
			ProxyID:     &proxyID,
			Concurrency: 3,
			Priority:    50,
			Status:      service.StatusDisabled,
		},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/data?include_proxies=false", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp dataResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Len(t, resp.Data.Proxies, 0)
	require.Len(t, resp.Data.Accounts, 1)
	require.Nil(t, resp.Data.Accounts[0].ProxyKey)
}

// TestExportDataExcludesSparkShadow 验证外审第5轮 P1/P2:导出时排除 spark 影子账号
// (影子无凭据、导入侧强制 credentials 非空,混入会产出无法还原的坏备份),并透出跳过计数。
func TestExportDataExcludesSparkShadow(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()

	parentID := int64(21)
	adminSvc.accounts = []service.Account{
		{
			ID:          parentID,
			Name:        "mother",
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeOAuth,
			Credentials: map[string]any{"token": "secret"},
			Status:      service.StatusActive,
		},
		{
			ID:              22,
			Name:            "mother (Spark)",
			Platform:        service.PlatformOpenAI,
			Type:            service.AccountTypeOAuth,
			Credentials:     map[string]any{}, // 影子恒空凭据
			ParentAccountID: &parentID,        // 影子标记
			QuotaDimension:  service.QuotaDimensionSpark,
			Status:          service.StatusActive,
		},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/data?include_proxies=false", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp dataResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Len(t, resp.Data.Accounts, 1, "影子应被排除,仅导出母账号")
	require.Equal(t, "mother", resp.Data.Accounts[0].Name)
	require.Equal(t, 1, resp.Data.SkippedShadows, "跳过的影子数量应透出")
}

func TestExportDataPassesAccountFiltersAndSort(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()
	adminSvc.accounts = []service.Account{
		{ID: 1, Name: "acc-1", Status: service.StatusActive},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/admin/accounts/data?platform=openai&type=oauth&status=active&group=12&privacy_mode=blocked&search=keyword&sort_by=priority&sort_order=desc",
		nil,
	)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	require.Equal(t, 1, adminSvc.lastListAccounts.calls)
	require.Equal(t, "openai", adminSvc.lastListAccounts.platform)
	require.Equal(t, "oauth", adminSvc.lastListAccounts.accountType)
	require.Equal(t, "active", adminSvc.lastListAccounts.status)
	require.Equal(t, int64(12), adminSvc.lastListAccounts.groupID)
	require.Equal(t, "blocked", adminSvc.lastListAccounts.privacyMode)
	require.Equal(t, "keyword", adminSvc.lastListAccounts.search)
	require.Equal(t, "priority", adminSvc.lastListAccounts.sortBy)
	require.Equal(t, "desc", adminSvc.lastListAccounts.sortOrder)
}

func TestExportDataSelectedIDsOverrideFilters(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/admin/accounts/data?ids=1,2&platform=openai&search=keyword&sort_by=priority&sort_order=desc",
		nil,
	)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp dataResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Len(t, resp.Data.Accounts, 2)
	require.Equal(t, 0, adminSvc.lastListAccounts.calls)
}

func TestImportDataReusesProxyAndSkipsDefaultGroup(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()

	adminSvc.proxies = []service.Proxy{
		{
			ID:       1,
			Name:     "proxy",
			Protocol: "socks5",
			Host:     "1.2.3.4",
			Port:     1080,
			Username: "u",
			Password: "p",
			Status:   service.StatusActive,
		},
	}

	dataPayload := map[string]any{
		"data": map[string]any{
			"type":    dataType,
			"version": dataVersion,
			"proxies": []map[string]any{
				{
					"proxy_key": "socks5|1.2.3.4|1080|u|p",
					"name":      "proxy",
					"protocol":  "socks5",
					"host":      "1.2.3.4",
					"port":      1080,
					"username":  "u",
					"password":  "p",
					"status":    "active",
				},
			},
			"accounts": []map[string]any{
				{
					"name":        "acc",
					"platform":    service.PlatformOpenAI,
					"type":        service.AccountTypeOAuth,
					"credentials": map[string]any{"token": "x"},
					"proxy_key":   "socks5|1.2.3.4|1080|u|p",
					"concurrency": 3,
					"priority":    50,
				},
			},
		},
		"skip_default_group_bind": true,
	}

	body, _ := json.Marshal(dataPayload)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/data", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	require.Len(t, adminSvc.createdProxies, 0)
	require.Len(t, adminSvc.createdAccounts, 1)
	require.True(t, adminSvc.createdAccounts[0].AtomicGroupBind)
	require.True(t, adminSvc.createdAccounts[0].SkipDefaultGroupBind)
}

func TestPreviewImportDataDetectsExistingAndBatchDuplicatesWithoutSecrets(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()
	adminSvc.accounts = []service.Account{{ID: 9, Name: "existing", Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Credentials: map[string]any{"chatgpt_account_id": "acct-1", "access_token": "existing-secret"}}}
	payload := map[string]any{"data": map[string]any{"proxies": []any{}, "accounts": []map[string]any{
		{"source_index": 10, "name": "one", "platform": service.PlatformOpenAI, "type": service.AccountTypeOAuth, "credentials": map[string]any{"chatgpt_account_id": "acct-1", "access_token": "s3cr"}},
		{"source_index": 11, "name": "two", "platform": service.PlatformOpenAI, "type": service.AccountTypeOAuth, "credentials": map[string]any{"chatgpt_account_id": "acct-2", "access_token": "very-secret-5678"}},
		{"source_index": 12, "name": "three", "platform": service.PlatformOpenAI, "type": service.AccountTypeSetupToken, "credentials": map[string]any{"chatgpt_account_id": "acct-2", "access_token": "very-secret-9012"}},
	}}}
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/data/preview", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "very-secret")
	require.NotContains(t, rec.Body.String(), "s3cr")
	var response struct {
		Data DataPreviewResult `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Equal(t, 3, response.Data.Total)
	require.Equal(t, 2, response.Data.Duplicate)
	require.Equal(t, 1, response.Data.Ready)
	require.Equal(t, 0, response.Data.Invalid)
	require.Equal(t, 0, response.Data.Unsupported)
	require.Len(t, response.Data.Items, 3)
	require.Equal(t, "existing", response.Data.Items[0].DuplicateScope)
	require.Equal(t, int64(9), response.Data.Items[0].ExistingAccountID)
	require.Equal(t, "batch", response.Data.Items[2].DuplicateScope)
	require.Equal(t, "••••5678", response.Data.Items[1].CredentialHint)
}

func TestImportDataSkipsDuplicatesAndAppliesBatchOptions(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()
	accounts := make([]map[string]any, 50)
	for i := range accounts {
		platform, accountType := service.PlatformOpenAI, service.AccountTypeOAuth
		if i == 0 {
			platform, accountType = " OpenAI ", " OAuth "
		}
		accounts[i] = map[string]any{"source_index": i + 1, "name": "account", "platform": platform, "type": accountType, "credentials": map[string]any{"chatgpt_account_id": "acct-" + strconv.Itoa(i/2), "token": "secret"}}
	}
	payload := map[string]any{"data": map[string]any{"proxies": []any{}, "accounts": accounts}, "options": map[string]any{"status": service.StatusDisabled, "schedulable": false, "priority": 7, "group_ids": []int64{3}}}
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/data", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, adminSvc.createdAccounts, 25)
	for _, input := range adminSvc.createdAccounts {
		require.Equal(t, service.PlatformOpenAI, input.Platform)
		require.Equal(t, service.AccountTypeOAuth, input.Type)
		require.Equal(t, service.StatusDisabled, input.Status)
		require.NotNil(t, input.Schedulable)
		require.False(t, *input.Schedulable)
		require.Equal(t, 7, input.Priority)
		require.True(t, input.AtomicGroupBind)
	}
	var response struct {
		Data DataImportResult `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Equal(t, 0, response.Data.AccountUpdated)
	require.Equal(t, 25, response.Data.AccountSkipped)
	require.Len(t, response.Data.Items, 50)
}

func TestImportDataDoesNotSerializeProxyKeyOrRawServiceErrors(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()
	adminSvc.createProxyErr = errors.New("proxy password super-secret failed")
	payload := map[string]any{"data": map[string]any{"proxies": []map[string]any{{"protocol": "http", "host": "example.test", "port": 8080, "username": "user", "password": "proxy-secret"}}, "accounts": []any{}}}
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/data", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "proxy-secret")
	require.NotContains(t, rec.Body.String(), "super-secret")
	require.NotContains(t, rec.Body.String(), "proxy_key")
	require.Contains(t, rec.Body.String(), "proxy could not be created")
}

func TestPreviewImportDataRejectsInvalidOptionsAndProviderShapes(t *testing.T) {
	router, _ := setupAccountDataRouter()
	payload := map[string]any{"data": map[string]any{"proxies": []any{}, "accounts": []map[string]any{
		{"name": "setup", "platform": service.PlatformOpenAI, "type": service.AccountTypeSetupToken, "credentials": map[string]any{"access_token": "token"}},
		{"name": "oauth", "platform": service.PlatformOpenAI, "type": service.AccountTypeOAuth, "credentials": map[string]any{"refresh_token": "token"}},
		{"name": "vertex", "platform": service.PlatformGemini, "type": service.AccountTypeServiceAccount, "credentials": map[string]any{"service_account_json": map[string]any{"client_email": "a@example.test", "private_key": "pem", "project_id": "project"}}},
		{"name": "vertex-copy", "platform": service.PlatformGemini, "type": service.AccountTypeServiceAccount, "credentials": map[string]any{"service_account_json": `{"client_email":"a@example.test","private_key":"pem","project_id":"project"}`}},
		{"name": "unsupported", "platform": service.PlatformKimi, "type": service.AccountTypeOAuth, "credentials": map[string]any{"token": "token"}},
	}}, "options": map[string]any{"priority": -1}}
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/data/preview", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	delete(payload, "options")
	body, err = json.Marshal(payload)
	require.NoError(t, err)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/data/preview", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var response struct {
		Data DataPreviewResult `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Equal(t, 3, response.Data.Ready)
	require.Equal(t, 1, response.Data.Duplicate)
	require.Equal(t, 1, response.Data.Unsupported)
}

func TestImportDataValidatesProxyOptionBeforeMutation(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()
	payload := map[string]any{
		"data": map[string]any{
			"proxies":  []map[string]any{{"name": "new", "protocol": "http", "host": "example.test", "port": 8080, "status": "active"}},
			"accounts": []map[string]any{{"name": "account", "platform": service.PlatformOpenAI, "type": service.AccountTypeAPIKey, "credentials": map[string]any{"api_key": "secret"}}},
		},
		"options": map[string]any{"proxy_id": 999},
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	for _, path := range []string{"/api/v1/admin/accounts/data/preview", "/api/v1/admin/accounts/data"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Contains(t, rec.Body.String(), "selected proxy is unavailable")
	}
	require.Empty(t, adminSvc.createdProxies)
	require.Empty(t, adminSvc.createdAccounts)
}

func TestDataImportRejectsSecretBearingHeaderWithoutEcho(t *testing.T) {
	router, _ := setupAccountDataRouter()
	payload := map[string]any{"data": map[string]any{"type": "secret-value", "proxies": []any{}, "accounts": []any{}}}
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/data/preview", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "unsupported data type")
	require.NotContains(t, rec.Body.String(), "secret-value")
}

func TestExportPreviewImportRoundTripPreservesLinkedProxy(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()
	proxyID := int64(11)
	adminSvc.proxies = []service.Proxy{{ID: proxyID, Name: "proxy", Protocol: "http", Host: "127.0.0.1", Port: 8080, Status: service.StatusActive}}
	adminSvc.accounts = []service.Account{{
		ID: 21, Name: "account", Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth,
		Credentials: map[string]any{"email": "roundtrip@example.test", "refresh_token": "secret"}, ProxyID: &proxyID,
	}}

	exportRec := httptest.NewRecorder()
	router.ServeHTTP(exportRec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/data", nil))
	require.Equal(t, http.StatusOK, exportRec.Code)
	var exported struct {
		Data DataPayload `json:"data"`
	}
	require.NoError(t, json.Unmarshal(exportRec.Body.Bytes(), &exported))
	adminSvc.accounts = nil
	adminSvc.proxies = nil

	body, err := json.Marshal(DataImportRequest{Data: exported.Data})
	require.NoError(t, err)
	previewRec := httptest.NewRecorder()
	previewReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/data/preview", bytes.NewReader(body))
	previewReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(previewRec, previewReq)
	require.Equal(t, http.StatusOK, previewRec.Code)
	var previewResponse struct {
		Data DataPreviewResult `json:"data"`
	}
	require.NoError(t, json.Unmarshal(previewRec.Body.Bytes(), &previewResponse))
	require.Equal(t, 1, previewResponse.Data.Ready)

	importRec := httptest.NewRecorder()
	importReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/data", bytes.NewReader(body))
	importReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(importRec, importReq)
	require.Equal(t, http.StatusOK, importRec.Code)
	require.Len(t, adminSvc.createdProxies, 1)
	require.Len(t, adminSvc.createdAccounts, 1)
	require.NotNil(t, adminSvc.createdAccounts[0].ProxyID)
}
