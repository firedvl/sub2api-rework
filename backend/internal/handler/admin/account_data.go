package admin

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"log/slog"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	dataType       = "sub2api-data"
	legacyDataType = "sub2api-bundle"
	dataVersion    = 1
	dataPageCap    = 1000
)

type DataPayload struct {
	Type       string        `json:"type,omitempty"`
	Version    int           `json:"version,omitempty"`
	ExportedAt string        `json:"exported_at"`
	Proxies    []DataProxy   `json:"proxies"`
	Accounts   []DataAccount `json:"accounts"`
	// SkippedShadows 记录导出时被排除的 spark 影子账号数量(见 ExportData)。仅作可见性提示,
	// 导入侧忽略该字段;omitempty 保持向后兼容。
	SkippedShadows int `json:"skipped_shadows,omitempty"`
}

type DataProxy struct {
	ProxyKey        string `json:"proxy_key"`
	Name            string `json:"name"`
	Protocol        string `json:"protocol"`
	Host            string `json:"host"`
	Port            int    `json:"port"`
	Username        string `json:"username,omitempty"`
	Password        string `json:"password,omitempty"`
	Status          string `json:"status"`
	ExpiresAt       *int64 `json:"expires_at,omitempty"`        // unix 秒，与 DataAccount.ExpiresAt 风格一致
	FallbackMode    string `json:"fallback_mode,omitempty"`     // none/direct/proxy
	BackupProxyName string `json:"backup_proxy_name,omitempty"` // 备用代理 name（跨实例按 name 反查）
	ExpiryWarnDays  int    `json:"expiry_warn_days,omitempty"`
}

// DataAccount 是管理员显式备份导出使用的账号结构，故意不走 dto.Account 的脱敏路径，
// Credentials 原文返回。这是"管理员备份"这一显式行为的一部分；如未来需要导出脱敏版本，
// 应新增独立结构而非修改这里。
// 注意:本结构不含 parent_account_id/quota_dimension——spark 影子账号在 ExportData 处被显式
// 排除(影子不持凭据、通用凭据型导入强制 credentials 非空无法重建父子链接),不在此表达。
// 影子的独立调度配置(priority/并发/分组/status 管理员可单独调)亦不在本备份范围,属已知局限
// (外审第6轮裁决:保持排除 + 前端警告,而非升级格式做完整往返)。
type DataAccount struct {
	SourceIndex        int            `json:"source_index,omitempty"`
	Name               string         `json:"name"`
	Notes              *string        `json:"notes,omitempty"`
	Platform           string         `json:"platform"`
	Type               string         `json:"type"`
	Credentials        map[string]any `json:"credentials"`
	Extra              map[string]any `json:"extra,omitempty"`
	ProxyKey           *string        `json:"proxy_key,omitempty"`
	Concurrency        int            `json:"concurrency"`
	Priority           int            `json:"priority"`
	RateMultiplier     *float64       `json:"rate_multiplier,omitempty"`
	ExpiresAt          *int64         `json:"expires_at,omitempty"`
	AutoPauseOnExpired *bool          `json:"auto_pause_on_expired,omitempty"`
}

type DataImportRequest struct {
	Data                 DataPayload        `json:"data"`
	SkipDefaultGroupBind *bool              `json:"skip_default_group_bind"`
	Options              *DataImportOptions `json:"options,omitempty"`
}

// DataImportOptions contains batch-wide, non-secret creation settings.
type DataImportOptions struct {
	Status      string  `json:"status,omitempty"`
	Schedulable *bool   `json:"schedulable,omitempty"`
	ProxyID     *int64  `json:"proxy_id,omitempty"`
	Priority    *int    `json:"priority,omitempty"`
	GroupIDs    []int64 `json:"group_ids,omitempty"`
}

type DataImportResult struct {
	ProxyCreated   int               `json:"proxy_created"`
	ProxyReused    int               `json:"proxy_reused"`
	ProxyFailed    int               `json:"proxy_failed"`
	AccountCreated int               `json:"account_created"`
	AccountUpdated int               `json:"account_updated"`
	AccountSkipped int               `json:"account_skipped"`
	AccountFailed  int               `json:"account_failed"`
	Errors         []DataImportError `json:"errors,omitempty"`
	Items          []DataImportRow   `json:"items,omitempty"`
}

type DataImportRow struct {
	Index     int    `json:"index"`
	Action    string `json:"action"`
	AccountID int64  `json:"account_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Message   string `json:"message,omitempty"`
}

type DataPreviewResult struct {
	Total       int              `json:"total"`
	Ready       int              `json:"ready"`
	Duplicate   int              `json:"duplicate"`
	Invalid     int              `json:"invalid"`
	Unsupported int              `json:"unsupported"`
	Items       []DataPreviewRow `json:"items"`
}

type DataPreviewRow struct {
	Index               int    `json:"index"`
	Status              string `json:"status"`
	Platform            string `json:"platform,omitempty"`
	Type                string `json:"type,omitempty"`
	Name                string `json:"name,omitempty"`
	Identity            string `json:"identity,omitempty"`
	DuplicateScope      string `json:"duplicate_scope,omitempty"`
	ExistingAccountID   int64  `json:"existing_account_id,omitempty"`
	ExistingAccountName string `json:"existing_account_name,omitempty"`
	CredentialHint      string `json:"credential_hint,omitempty"`
	Message             string `json:"message,omitempty"`
}

type DataImportError struct {
	Kind     string `json:"kind"`
	Name     string `json:"name,omitempty"`
	ProxyKey string `json:"-"`
	Message  string `json:"message"`
}

func buildProxyKey(protocol, host string, port int, username, password string) string {
	return fmt.Sprintf("%s|%s|%d|%s|%s", strings.TrimSpace(protocol), strings.TrimSpace(host), port, strings.TrimSpace(username), strings.TrimSpace(password))
}

func (h *AccountHandler) ExportData(c *gin.Context) {
	ctx := c.Request.Context()

	selectedIDs, err := parseAccountIDs(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	accounts, err := h.resolveExportAccounts(ctx, selectedIDs, c)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 排除 spark 影子账号:影子不持凭据,通用凭据型导出无法表达父子链接、导入侧又强制 credentials
	// 非空——若混入会产出无法还原的坏备份(导入即失败)。影子的独立调度配置(priority/并发/分组/
	// status,管理员可单独调)随之不进备份,还原后需在重建的影子上重新调优;前端按 skipped_shadows
	// 提示用户(外审第5轮发现、第6轮裁决:保持排除 + 警告,不做完整往返)。
	skippedShadows := 0
	exportable := make([]service.Account, 0, len(accounts))
	for i := range accounts {
		if accounts[i].IsCredentialShadow() {
			skippedShadows++
			continue
		}
		exportable = append(exportable, accounts[i])
	}
	accounts = exportable
	if skippedShadows > 0 {
		slog.Info("export_skipped_spark_shadows", "count", skippedShadows)
	}

	includeProxies, err := parseIncludeProxies(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var proxies []service.Proxy
	if includeProxies {
		proxies, err = h.resolveExportProxies(ctx, accounts)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
	} else {
		proxies = []service.Proxy{}
	}

	// 构建 id→name 映射，用于导出备用代理 name
	proxyNameByID := make(map[int64]string, len(proxies))
	for i := range proxies {
		proxyNameByID[proxies[i].ID] = proxies[i].Name
	}

	proxyKeyByID := make(map[int64]string, len(proxies))
	dataProxies := make([]DataProxy, 0, len(proxies))
	for i := range proxies {
		p := proxies[i]
		key := buildProxyKey(p.Protocol, p.Host, p.Port, p.Username, p.Password)
		proxyKeyByID[p.ID] = key

		var expiresAt *int64
		if p.ExpiresAt != nil {
			v := p.ExpiresAt.Unix()
			expiresAt = &v
		}
		var backupProxyName string
		if p.BackupProxyID != nil {
			backupProxyName = proxyNameByID[*p.BackupProxyID]
		}
		dataProxies = append(dataProxies, DataProxy{
			ProxyKey:        key,
			Name:            p.Name,
			Protocol:        p.Protocol,
			Host:            p.Host,
			Port:            p.Port,
			Username:        p.Username,
			Password:        p.Password,
			Status:          p.Status,
			ExpiresAt:       expiresAt,
			FallbackMode:    p.FallbackMode,
			BackupProxyName: backupProxyName,
			ExpiryWarnDays:  p.ExpiryWarnDays,
		})
	}

	dataAccounts := make([]DataAccount, 0, len(accounts))
	for i := range accounts {
		acc := accounts[i]
		var proxyKey *string
		if acc.ProxyID != nil {
			if key, ok := proxyKeyByID[*acc.ProxyID]; ok {
				proxyKey = &key
			}
		}
		var expiresAt *int64
		if acc.ExpiresAt != nil {
			v := acc.ExpiresAt.Unix()
			expiresAt = &v
		}
		dataAccounts = append(dataAccounts, DataAccount{
			Name:               acc.Name,
			Notes:              acc.Notes,
			Platform:           acc.Platform,
			Type:               acc.Type,
			Credentials:        acc.Credentials,
			Extra:              acc.Extra,
			ProxyKey:           proxyKey,
			Concurrency:        acc.Concurrency,
			Priority:           acc.Priority,
			RateMultiplier:     acc.RateMultiplier,
			ExpiresAt:          expiresAt,
			AutoPauseOnExpired: &acc.AutoPauseOnExpired,
		})
	}

	payload := DataPayload{
		ExportedAt:     time.Now().UTC().Format(time.RFC3339),
		Proxies:        dataProxies,
		Accounts:       dataAccounts,
		SkippedShadows: skippedShadows,
	}

	response.Success(c, payload)
}

func (h *AccountHandler) ImportData(c *gin.Context) {
	var req DataImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}

	if err := validateDataHeader(req.Data); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := validateImportOptions(req.Options); err != nil {
		response.BadRequest(c, safeValidationMessage(err))
		return
	}

	executeAdminIdempotentJSON(c, "admin.accounts.import_data", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		result, err := h.importData(ctx, req)
		middleware.SetAuditExtra(c, map[string]any{"requested": len(req.Data.Accounts), "created": result.AccountCreated, "skipped": result.AccountSkipped, "failed": result.AccountFailed})
		return result, err
	})
}

func (h *AccountHandler) PreviewImportData(c *gin.Context) {
	var req DataImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	if err := validateDataHeader(req.Data); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := validateImportOptions(req.Options); err != nil {
		response.BadRequest(c, safeValidationMessage(err))
		return
	}
	result, err := h.previewData(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AccountHandler) importData(ctx context.Context, req DataImportRequest) (DataImportResult, error) {
	skipDefaultGroupBind := true
	if req.SkipDefaultGroupBind != nil {
		skipDefaultGroupBind = *req.SkipDefaultGroupBind
	}

	dataPayload := req.Data
	result := DataImportResult{}

	existingProxies, err := h.listAllProxies(ctx)
	if err != nil {
		return result, err
	}
	if err := validateImportProxyOption(req.Options, existingProxies); err != nil {
		return result, err
	}

	proxyKeyToID := make(map[string]int64, len(existingProxies))
	// proxyNameToID 用于 backup_proxy_name 反查：DB 已有 + 本批次新建均会写入
	proxyNameToID := make(map[string]int64, len(existingProxies))
	for i := range existingProxies {
		p := existingProxies[i]
		key := buildProxyKey(p.Protocol, p.Host, p.Port, p.Username, p.Password)
		proxyKeyToID[key] = p.ID
		if p.Name != "" {
			proxyNameToID[p.Name] = p.ID
		}
	}

	for i := range dataPayload.Proxies {
		item := dataPayload.Proxies[i]
		key := item.ProxyKey
		if key == "" {
			key = buildProxyKey(item.Protocol, item.Host, item.Port, item.Username, item.Password)
		}
		if err := validateDataProxy(item); err != nil {
			result.ProxyFailed++
			result.Errors = append(result.Errors, DataImportError{
				Kind:    "proxy",
				Name:    item.Name,
				Message: "proxy data is invalid",
			})
			continue
		}
		normalizedStatus := normalizeProxyStatus(item.Status)
		if existingID, ok := proxyKeyToID[key]; ok {
			proxyKeyToID[key] = existingID
			result.ProxyReused++
			if normalizedStatus != "" {
				if proxy, getErr := h.adminService.GetProxy(ctx, existingID); getErr == nil && proxy != nil && proxy.Status != normalizedStatus {
					// 同步 status 时传入完整字段，避免零值覆盖已存在代理的有效期/fallback 配置。
					var existingExpiresAt *time.Time
					if item.ExpiresAt != nil {
						t := time.Unix(*item.ExpiresAt, 0).UTC()
						existingExpiresAt = &t
					}
					existingFallbackMode := item.FallbackMode
					if existingFallbackMode == "" {
						existingFallbackMode = service.FallbackModeNone
					}
					var existingBackupProxyID *int64
					if item.BackupProxyName != "" {
						if bid, ok := proxyNameToID[item.BackupProxyName]; ok {
							existingBackupProxyID = &bid
						}
					}
					_, _ = h.adminService.UpdateProxy(ctx, existingID, &service.UpdateProxyInput{
						Status:         normalizedStatus,
						ExpiresAt:      existingExpiresAt,
						FallbackMode:   existingFallbackMode,
						BackupProxyID:  existingBackupProxyID,
						ExpiryWarnDays: item.ExpiryWarnDays,
						Name:           proxy.Name,
						Protocol:       proxy.Protocol,
						Host:           proxy.Host,
						Port:           proxy.Port,
						Username:       proxy.Username,
						Password:       proxy.Password,
					})
				}
			}
			continue
		}

		// 解析 expires_at（unix 秒 → *time.Time）
		var expiresAt *time.Time
		if item.ExpiresAt != nil {
			t := time.Unix(*item.ExpiresAt, 0).UTC()
			expiresAt = &t
		}

		// 解析 backup_proxy_name → backup_proxy_id
		fallbackMode := item.FallbackMode
		var backupProxyID *int64
		if item.BackupProxyName != "" {
			if bid, ok := proxyNameToID[item.BackupProxyName]; ok {
				backupProxyID = &bid
			} else {
				// 查不到备用代理：降级 fallback_mode=none，记录 warning
				fallbackMode = service.FallbackModeNone
				result.Errors = append(result.Errors, DataImportError{
					Kind:    "proxy",
					Name:    item.Name,
					Message: "backup proxy is unavailable",
				})
			}
		}

		created, createErr := h.adminService.CreateProxy(ctx, &service.CreateProxyInput{
			Name:           defaultProxyName(item.Name),
			Protocol:       item.Protocol,
			Host:           item.Host,
			Port:           item.Port,
			Username:       item.Username,
			Password:       item.Password,
			ExpiresAt:      expiresAt,
			FallbackMode:   fallbackMode,
			BackupProxyID:  backupProxyID,
			ExpiryWarnDays: item.ExpiryWarnDays,
		})
		if createErr != nil {
			result.ProxyFailed++
			result.Errors = append(result.Errors, DataImportError{
				Kind:    "proxy",
				Name:    item.Name,
				Message: "proxy could not be created",
			})
			continue
		}
		proxyKeyToID[key] = created.ID
		// 把新建代理的 name 也加入反查表，供后续批内代理引用
		if created.Name != "" {
			proxyNameToID[created.Name] = created.ID
		}
		result.ProxyCreated++

		if normalizedStatus != "" && normalizedStatus != created.Status {
			// 新建后同步 status 时，传入完整字段，避免零值覆盖刚创建的有效期/fallback 配置。
			_, _ = h.adminService.UpdateProxy(ctx, created.ID, &service.UpdateProxyInput{
				Status:         normalizedStatus,
				ExpiresAt:      expiresAt,
				FallbackMode:   fallbackMode,
				BackupProxyID:  backupProxyID,
				ExpiryWarnDays: item.ExpiryWarnDays,
				Name:           created.Name,
				Protocol:       created.Protocol,
				Host:           created.Host,
				Port:           created.Port,
				Username:       created.Username,
				Password:       created.Password,
			})
		}
	}

	// Run OAuth privacy once after the batch instead of one goroutine per row.
	var privacyAccounts []*service.Account
	existingAccounts, err := h.listAccountsFiltered(ctx, "", "", "", "", 0, "", "id", "asc")
	if err != nil {
		return result, err
	}
	seenIdentities := make(map[string]struct{}, len(existingAccounts)+len(dataPayload.Accounts))
	for i := range existingAccounts {
		if identity := dataAccountIdentity(existingAccounts[i].Platform, existingAccounts[i].Type, existingAccounts[i].Credentials, existingAccounts[i].Extra); identity != "" {
			seenIdentities[identity] = struct{}{}
		}
	}
	for i := range dataPayload.Accounts {
		item := normalizeDataAccount(dataPayload.Accounts[i])
		index := item.SourceIndex
		if index == 0 {
			index = i + 1
		}
		if err := validateDataAccount(item); err != nil {
			result.AccountFailed++
			result.Errors = append(result.Errors, DataImportError{
				Kind:    "account",
				Name:    item.Name,
				Message: safeValidationMessage(err),
			})
			result.Items = append(result.Items, DataImportRow{Index: index, Action: "failed", Name: item.Name, Message: safeValidationMessage(err)})
			continue
		}
		identity := dataAccountIdentity(item.Platform, item.Type, item.Credentials, item.Extra)
		if identity != "" {
			if _, exists := seenIdentities[identity]; exists {
				result.AccountSkipped++
				result.Items = append(result.Items, DataImportRow{Index: index, Action: "skipped", Name: item.Name, Message: "duplicate account"})
				continue
			}
			seenIdentities[identity] = struct{}{}
		}

		var proxyID *int64
		if item.ProxyKey != nil && *item.ProxyKey != "" {
			if id, ok := proxyKeyToID[*item.ProxyKey]; ok {
				proxyID = &id
			} else {
				result.AccountFailed++
				result.Errors = append(result.Errors, DataImportError{
					Kind:    "account",
					Name:    item.Name,
					Message: "proxy_key not found",
				})
				result.Items = append(result.Items, DataImportRow{Index: index, Action: "failed", Name: item.Name, Message: "proxy is unavailable"})
				continue
			}
		}
		if req.Options != nil && req.Options.ProxyID != nil {
			proxyID = req.Options.ProxyID
		}

		enrichCredentialsFromIDToken(&item)

		accountInput := &service.CreateAccountInput{
			Name:                 item.Name,
			Notes:                item.Notes,
			Platform:             item.Platform,
			Type:                 item.Type,
			Credentials:          item.Credentials,
			Extra:                item.Extra,
			ProxyID:              proxyID,
			Concurrency:          item.Concurrency,
			Priority:             item.Priority,
			RateMultiplier:       item.RateMultiplier,
			GroupIDs:             nil,
			ExpiresAt:            item.ExpiresAt,
			AutoPauseOnExpired:   item.AutoPauseOnExpired,
			AtomicGroupBind:      true,
			SkipDefaultGroupBind: skipDefaultGroupBind,
			SkipAsyncPrivacy:     true,
		}
		if req.Options != nil {
			accountInput.Status = req.Options.Status
			accountInput.Schedulable = req.Options.Schedulable
			if req.Options.Priority != nil {
				accountInput.Priority = *req.Options.Priority
			}
			if len(req.Options.GroupIDs) > 0 {
				accountInput.GroupIDs = req.Options.GroupIDs
				accountInput.SkipDefaultGroupBind = true
			}
		}

		created, err := h.adminService.CreateAccount(ctx, accountInput)
		if err != nil {
			result.AccountFailed++
			result.Errors = append(result.Errors, DataImportError{
				Kind:    "account",
				Name:    item.Name,
				Message: "account could not be created",
			})
			result.Items = append(result.Items, DataImportRow{Index: index, Action: "failed", Name: item.Name, Message: "account could not be created"})
			continue
		}
		// 收集 Antigravity OAuth 账号，稍后异步设置隐私
		if (created.Platform == service.PlatformOpenAI || created.Platform == service.PlatformAntigravity) && created.Type == service.AccountTypeOAuth {
			privacyAccounts = append(privacyAccounts, created)
		}
		h.scheduleGrokImportProbe(created)
		result.AccountCreated++
		result.Items = append(result.Items, DataImportRow{Index: index, Action: "created", AccountID: created.ID, Name: created.Name})
	}

	// Bounded sequential post-import privacy work avoids unbounded goroutine fanout.
	if len(privacyAccounts) > 0 {
		adminSvc := h.adminService
		go func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("import_antigravity_privacy_panic", "recover", r)
				}
			}()
			bgCtx := context.Background()
			for _, acc := range privacyAccounts {
				if acc.Platform == service.PlatformOpenAI {
					adminSvc.EnsureOpenAIPrivacy(bgCtx, acc)
				} else {
					adminSvc.EnsureAntigravityPrivacy(bgCtx, acc)
				}
			}
			slog.Info("import_oauth_privacy_done", "count", len(privacyAccounts))
		}()
	}

	return result, nil
}

func (h *AccountHandler) listAllProxies(ctx context.Context) ([]service.Proxy, error) {
	page := 1
	pageSize := dataPageCap
	var out []service.Proxy
	for {
		items, total, err := h.adminService.ListProxies(ctx, page, pageSize, "", "", "", "created_at", "desc")
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
		if len(out) >= int(total) || len(items) == 0 {
			break
		}
		page++
	}
	return out, nil
}

func (h *AccountHandler) previewData(ctx context.Context, req DataImportRequest) (DataPreviewResult, error) {
	payload := req.Data
	existing, err := h.listAccountsFiltered(ctx, "", "", "", "", 0, "", "id", "asc")
	if err != nil {
		return DataPreviewResult{}, err
	}
	existingByIdentity := make(map[string]service.Account, len(existing))
	for i := range existing {
		if identity := dataAccountIdentity(existing[i].Platform, existing[i].Type, existing[i].Credentials, existing[i].Extra); identity != "" {
			existingByIdentity[identity] = existing[i]
		}
	}
	existingProxies, err := h.listAllProxies(ctx)
	if err != nil {
		return DataPreviewResult{}, err
	}
	if err := validateImportProxyOption(req.Options, existingProxies); err != nil {
		return DataPreviewResult{}, err
	}
	proxyKeys := make(map[string]struct{}, len(existingProxies)+len(payload.Proxies))
	for _, proxy := range existingProxies {
		proxyKeys[buildProxyKey(proxy.Protocol, proxy.Host, proxy.Port, proxy.Username, proxy.Password)] = struct{}{}
	}
	for _, proxy := range payload.Proxies {
		if validateDataProxy(proxy) != nil {
			continue
		}
		key := proxy.ProxyKey
		if key == "" {
			key = buildProxyKey(proxy.Protocol, proxy.Host, proxy.Port, proxy.Username, proxy.Password)
		}
		proxyKeys[key] = struct{}{}
	}
	seen := make(map[string]struct{}, len(payload.Accounts))
	result := DataPreviewResult{Total: len(payload.Accounts), Items: make([]DataPreviewRow, 0, len(payload.Accounts))}
	for i := range payload.Accounts {
		item := normalizeDataAccount(payload.Accounts[i])
		index := item.SourceIndex
		if index == 0 {
			index = i + 1
		}
		row := DataPreviewRow{Index: index, Status: "ready", Platform: item.Platform, Type: item.Type, Name: item.Name}
		if err := validateDataAccount(item); err != nil {
			row.Status, row.Message = validationPreviewStatus(err), safeValidationMessage(err)
			if row.Status == "unsupported" {
				result.Unsupported++
			} else {
				result.Invalid++
			}
			result.Items = append(result.Items, row)
			continue
		}
		if item.ProxyKey != nil && *item.ProxyKey != "" && (req.Options == nil || req.Options.ProxyID == nil) {
			if _, ok := proxyKeys[*item.ProxyKey]; !ok {
				row.Status, row.Message = "invalid", "proxy is unavailable"
				result.Invalid++
				result.Items = append(result.Items, row)
				continue
			}
		}
		identity := dataAccountIdentity(item.Platform, item.Type, item.Credentials, item.Extra)
		row.Identity = identityDisplay(identity)
		row.CredentialHint = credentialHint(item.Credentials)
		if identity != "" {
			if account, ok := existingByIdentity[identity]; ok {
				row.Status, row.DuplicateScope, row.ExistingAccountID, row.ExistingAccountName = "duplicate", "existing", account.ID, account.Name
			} else if _, ok := seen[identity]; ok {
				row.Status, row.DuplicateScope = "duplicate", "batch"
			}
			seen[identity] = struct{}{}
		}
		switch row.Status {
		case "duplicate":
			result.Duplicate++
		default:
			result.Ready++
		}
		result.Items = append(result.Items, row)
	}
	return result, nil
}

func safeValidationMessage(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	switch message {
	case "account name is required", "account platform is required", "account type is required",
		"account credentials is required", "account platform is unsupported", "account type is invalid",
		"bedrock accounts require anthropic", "service accounts require anthropic or gemini",
		"setup-token accounts require anthropic or openai", "upstream accounts require antigravity",
		"this platform requires an api key", "api_key is required", "oauth token is required",
		"setup token is required", "service account credential is invalid", "bedrock credentials are required",
		"upstream base_url is invalid", "upstream api_key is required", "rate_multiplier must be >= 0", "concurrency must be >= 0",
		"priority must be >= 0", "account status is invalid", "proxy_id must be positive",
		"group_ids must be positive":
		return message
	}
	return "account data is invalid"
}

func validationPreviewStatus(err error) string {
	if err == nil {
		return "ready"
	}
	message := err.Error()
	if strings.Contains(message, "unsupported") || strings.Contains(message, "requires ") || message == "account type is invalid" {
		return "unsupported"
	}
	return "invalid"
}

func validateImportOptions(options *DataImportOptions) error {
	if options == nil {
		return nil
	}
	if options.Status != "" && options.Status != service.StatusActive && options.Status != service.StatusDisabled {
		return errors.New("account status is invalid")
	}
	if options.ProxyID != nil && *options.ProxyID <= 0 {
		return errors.New("proxy_id must be positive")
	}
	if options.Priority != nil && *options.Priority < 0 {
		return errors.New("priority must be >= 0")
	}
	for _, groupID := range options.GroupIDs {
		if groupID <= 0 {
			return errors.New("group_ids must be positive")
		}
	}
	return nil
}

func validateImportProxyOption(options *DataImportOptions, proxies []service.Proxy) error {
	if options == nil || options.ProxyID == nil {
		return nil
	}
	for i := range proxies {
		if proxies[i].ID == *options.ProxyID {
			return nil
		}
	}
	return infraerrors.BadRequest("IMPORT_PROXY_NOT_FOUND", "selected proxy is unavailable")
}

func dataAccountIdentity(platform, accountType string, credentials, extra map[string]any) string {
	platform, accountType = strings.ToLower(strings.TrimSpace(platform)), strings.ToLower(strings.TrimSpace(accountType))
	if accountType == service.AccountTypeServiceAccount {
		if email, err := service.VertexServiceAccountClientEmail(credentials); err == nil {
			return identityKey(platform, "client_email", email)
		}
	}
	for _, key := range []string{"crs_account_id", "chatgpt_account_id", "chatgpt_user_id", "account_uuid", "email", "client_email"} {
		value := stringValue(extra, key)
		if value == "" {
			value = stringValue(credentials, key)
		}
		if value != "" {
			return identityKey(platform, key, value)
		}
	}
	return ""
}

func identityKey(platform, kind, value string) string {
	return strings.ToLower(strings.TrimSpace(platform)) + "\x00" + kind + "\x00" + strings.ToLower(strings.TrimSpace(value))
}

func identityDisplay(identity string) string {
	parts := strings.SplitN(identity, "\x00", 3)
	if len(parts) == 3 {
		return parts[2]
	}
	return ""
}

func stringValue(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

func credentialHint(credentials map[string]any) string {
	for _, key := range []string{"api_key", "access_token", "token", "refresh_token", "setup_token"} {
		value := stringValue(credentials, key)
		if value == "" {
			continue
		}
		runes := []rune(value)
		if len(runes) <= 4 {
			return "••••"
		}
		return "••••" + string(runes[len(runes)-4:])
	}
	return ""
}

func (h *AccountHandler) listAccountsFiltered(ctx context.Context, platform, accountType, status, search string, groupID int64, privacyMode, sortBy, sortOrder string) ([]service.Account, error) {
	page := 1
	pageSize := dataPageCap
	var out []service.Account
	for {
		items, total, err := h.adminService.ListAccounts(ctx, page, pageSize, platform, accountType, status, search, groupID, privacyMode, sortBy, sortOrder)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
		if len(out) >= int(total) || len(items) == 0 {
			break
		}
		page++
	}
	return out, nil
}

func (h *AccountHandler) resolveExportAccounts(ctx context.Context, ids []int64, c *gin.Context) ([]service.Account, error) {
	if len(ids) > 0 {
		accounts, err := h.adminService.GetAccountsByIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
		out := make([]service.Account, 0, len(accounts))
		for _, acc := range accounts {
			if acc == nil {
				continue
			}
			out = append(out, *acc)
		}
		return out, nil
	}

	platform := c.Query("platform")
	accountType := c.Query("type")
	status := c.Query("status")
	privacyMode := strings.TrimSpace(c.Query("privacy_mode"))
	search := strings.TrimSpace(c.Query("search"))
	sortBy := c.DefaultQuery("sort_by", "name")
	sortOrder := c.DefaultQuery("sort_order", "asc")
	if len(search) > 100 {
		search = search[:100]
	}

	groupID := int64(0)
	if groupIDStr := c.Query("group"); groupIDStr != "" {
		if groupIDStr == accountListGroupUngroupedQueryValue {
			groupID = service.AccountListGroupUngrouped
		} else {
			parsedGroupID, parseErr := strconv.ParseInt(groupIDStr, 10, 64)
			if parseErr != nil || parsedGroupID <= 0 {
				return nil, infraerrors.BadRequest("INVALID_GROUP_FILTER", "invalid group filter")
			}
			groupID = parsedGroupID
		}
	}

	return h.listAccountsFiltered(ctx, platform, accountType, status, search, groupID, privacyMode, sortBy, sortOrder)
}

func (h *AccountHandler) resolveExportProxies(ctx context.Context, accounts []service.Account) ([]service.Proxy, error) {
	if len(accounts) == 0 {
		return []service.Proxy{}, nil
	}

	seen := make(map[int64]struct{})
	ids := make([]int64, 0)
	for i := range accounts {
		if accounts[i].ProxyID == nil {
			continue
		}
		id := *accounts[i].ProxyID
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return []service.Proxy{}, nil
	}

	return h.adminService.GetProxiesByIDs(ctx, ids)
}

func parseAccountIDs(c *gin.Context) ([]int64, error) {
	values := c.QueryArray("ids")
	if len(values) == 0 {
		raw := strings.TrimSpace(c.Query("ids"))
		if raw != "" {
			values = []string{raw}
		}
	}
	if len(values) == 0 {
		return nil, nil
	}

	ids := make([]int64, 0, len(values))
	for _, item := range values {
		for _, part := range strings.Split(item, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, err := strconv.ParseInt(part, 10, 64)
			if err != nil || id <= 0 {
				return nil, fmt.Errorf("invalid account id: %s", part)
			}
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func parseIncludeProxies(c *gin.Context) (bool, error) {
	raw := strings.TrimSpace(strings.ToLower(c.Query("include_proxies")))
	if raw == "" {
		return true, nil
	}
	switch raw {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return true, fmt.Errorf("invalid include_proxies value: %s", raw)
	}
}

func validateDataHeader(payload DataPayload) error {
	if payload.Type != "" && payload.Type != dataType && payload.Type != legacyDataType {
		return errors.New("unsupported data type")
	}
	if payload.Version != 0 && payload.Version != dataVersion {
		return errors.New("unsupported data version")
	}
	if payload.Proxies == nil {
		return errors.New("proxies is required")
	}
	if payload.Accounts == nil {
		return errors.New("accounts is required")
	}
	return nil
}

func validateDataProxy(item DataProxy) error {
	if strings.TrimSpace(item.Protocol) == "" {
		return errors.New("proxy protocol is required")
	}
	if strings.TrimSpace(item.Host) == "" {
		return errors.New("proxy host is required")
	}
	if item.Port <= 0 || item.Port > 65535 {
		return errors.New("proxy port is invalid")
	}
	switch item.Protocol {
	case "http", "https", "socks5", "socks5h":
	default:
		return fmt.Errorf("proxy protocol is invalid: %s", item.Protocol)
	}
	if item.Status != "" {
		normalizedStatus := normalizeProxyStatus(item.Status)
		if normalizedStatus != service.StatusActive && normalizedStatus != "inactive" {
			return fmt.Errorf("proxy status is invalid: %s", item.Status)
		}
	}
	return nil
}

func validateDataAccount(item DataAccount) error {
	if strings.TrimSpace(item.Name) == "" {
		return errors.New("account name is required")
	}
	if strings.TrimSpace(item.Platform) == "" {
		return errors.New("account platform is required")
	}
	if strings.TrimSpace(item.Type) == "" {
		return errors.New("account type is required")
	}
	if len(item.Credentials) == 0 {
		return errors.New("account credentials is required")
	}
	platform := strings.ToLower(strings.TrimSpace(item.Platform))
	accountType := strings.ToLower(strings.TrimSpace(item.Type))
	supported := map[string]bool{service.PlatformAnthropic: true, service.PlatformOpenAI: true, service.PlatformGemini: true, service.PlatformAntigravity: true, service.PlatformGrok: true, service.PlatformKimi: true, service.PlatformZhipu: true, service.PlatformDeepseek: true}
	if !supported[platform] {
		return errors.New("account platform is unsupported")
	}
	switch accountType {
	case service.AccountTypeOAuth, service.AccountTypeSetupToken, service.AccountTypeAPIKey, service.AccountTypeUpstream, service.AccountTypeBedrock, service.AccountTypeServiceAccount:
	default:
		return errors.New("account type is invalid")
	}
	if accountType == service.AccountTypeBedrock && platform != service.PlatformAnthropic {
		return errors.New("bedrock accounts require anthropic")
	}
	if accountType == service.AccountTypeServiceAccount && platform != service.PlatformAnthropic && platform != service.PlatformGemini {
		return errors.New("service accounts require anthropic or gemini")
	}
	if accountType == service.AccountTypeSetupToken && platform != service.PlatformAnthropic && platform != service.PlatformOpenAI {
		return errors.New("setup-token accounts require anthropic or openai")
	}
	if accountType == service.AccountTypeUpstream && platform != service.PlatformAntigravity {
		return errors.New("upstream accounts require antigravity")
	}
	if (platform == service.PlatformKimi || platform == service.PlatformZhipu || platform == service.PlatformDeepseek) && accountType != service.AccountTypeAPIKey {
		return errors.New("this platform requires an api key")
	}
	if accountType == service.AccountTypeAPIKey && stringValue(item.Credentials, "api_key") == "" {
		return errors.New("api_key is required")
	}
	if accountType == service.AccountTypeOAuth && stringValue(item.Credentials, "access_token") == "" && stringValue(item.Credentials, "refresh_token") == "" && stringValue(item.Credentials, "token") == "" {
		return errors.New("oauth token is required")
	}
	if accountType == service.AccountTypeSetupToken && stringValue(item.Credentials, "setup_token") == "" && stringValue(item.Credentials, "access_token") == "" && stringValue(item.Credentials, "refresh_token") == "" && stringValue(item.Credentials, "token") == "" {
		return errors.New("setup token is required")
	}
	if accountType == service.AccountTypeServiceAccount {
		if _, err := service.VertexServiceAccountClientEmail(item.Credentials); err != nil {
			return errors.New("service account credential is invalid")
		}
	}
	if accountType == service.AccountTypeBedrock && stringValue(item.Credentials, "api_key") == "" && (stringValue(item.Credentials, "aws_access_key_id") == "" || stringValue(item.Credentials, "aws_secret_access_key") == "") {
		return errors.New("bedrock credentials are required")
	}
	if accountType == service.AccountTypeUpstream {
		baseURL := stringValue(item.Credentials, "base_url")
		parsed, err := url.Parse(baseURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return errors.New("upstream base_url is invalid")
		}
		if stringValue(item.Credentials, "api_key") == "" {
			return errors.New("upstream api_key is required")
		}
	}
	if item.RateMultiplier != nil && *item.RateMultiplier < 0 {
		return errors.New("rate_multiplier must be >= 0")
	}
	if item.Concurrency < 0 {
		return errors.New("concurrency must be >= 0")
	}
	if item.Priority < 0 {
		return errors.New("priority must be >= 0")
	}
	return nil
}

func normalizeDataAccount(item DataAccount) DataAccount {
	item.Platform = strings.ToLower(strings.TrimSpace(item.Platform))
	item.Type = strings.ToLower(strings.TrimSpace(item.Type))
	return item
}

func defaultProxyName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "imported-proxy"
	}
	return name
}

// enrichCredentialsFromIDToken performs best-effort extraction of user info fields
// (email, plan_type, chatgpt_account_id, etc.) from id_token in credentials.
// Only applies to OpenAI OAuth accounts. Skips expired token errors silently.
// Existing credential values are never overwritten — only missing fields are filled.
func enrichCredentialsFromIDToken(item *DataAccount) {
	if item.Credentials == nil {
		return
	}
	// Only enrich OpenAI OAuth accounts
	platform := strings.ToLower(strings.TrimSpace(item.Platform))
	if platform != service.PlatformOpenAI {
		return
	}
	if strings.ToLower(strings.TrimSpace(item.Type)) != service.AccountTypeOAuth {
		return
	}

	idToken, _ := item.Credentials["id_token"].(string)
	if strings.TrimSpace(idToken) == "" {
		return
	}

	// DecodeIDToken skips expiry validation — safe for imported data
	claims, err := openai.DecodeIDToken(idToken)
	if err != nil {
		slog.Debug("import_enrich_id_token_decode_failed", "account", item.Name, "error", err)
		return
	}

	userInfo := claims.GetUserInfo()
	if userInfo == nil {
		return
	}

	// Fill missing fields only (never overwrite existing values)
	setIfMissing := func(key, value string) {
		if value == "" {
			return
		}
		if existing, _ := item.Credentials[key].(string); existing == "" {
			item.Credentials[key] = value
		}
	}

	setIfMissing("email", userInfo.Email)
	setIfMissing("plan_type", userInfo.PlanType)
	setIfMissing("chatgpt_account_id", userInfo.ChatGPTAccountID)
	setIfMissing("chatgpt_user_id", userInfo.ChatGPTUserID)
	setIfMissing("organization_id", userInfo.OrganizationID)
}

func normalizeProxyStatus(status string) string {
	normalized := strings.TrimSpace(strings.ToLower(status))
	switch normalized {
	case "":
		return ""
	case service.StatusActive:
		return service.StatusActive
	case "inactive", service.StatusDisabled:
		return "inactive"
	case "expired":
		// 导入 expired 代理按 inactive 处理，避免导入即触发到期改投逻辑
		return "inactive"
	default:
		return normalized
	}
}
