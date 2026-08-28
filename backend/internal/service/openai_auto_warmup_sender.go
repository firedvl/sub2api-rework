package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	openAIAutoWarmupPrompt         = "Reply with OK only."
	openAIAutoWarmupMaxOutput      = 4
	openAIAutoWarmupRequestLimit   = 1 << 20
	openAIAutoWarmupRequestTimeout = 15 * time.Second
)

func (s *OpenAIGatewayService) SendOpenAIAutoWarmup(ctx context.Context, accountID int64) (*OpenAIAutoWarmupResult, error) {
	if s == nil || s.accountRepo == nil || s.httpUpstream == nil {
		return nil, infraerrors.InternalServer("OPENAI_AUTO_WARMUP_NOT_CONFIGURED", "OpenAI Auto Warm-up is not configured")
	}
	account, err := s.validateOpenAIAutoWarmupDispatch(ctx, accountID)
	if err != nil {
		return nil, err
	}

	accessToken, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "OPENAI_AUTO_WARMUP_AUTH_FAILED", "refresh OpenAI credentials: %v", err)
	}
	model, err := s.resolveOpenAIAutoWarmupModel(ctx, account, accessToken)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(map[string]any{
		"model":               model,
		"input":               openAIAutoWarmupPrompt,
		"tools":               []any{},
		"parallel_tool_calls": false,
		"store":               false,
		"max_output_tokens":   openAIAutoWarmupMaxOutput,
		"stream":              false,
	})
	if err != nil {
		return nil, infraerrors.Newf(http.StatusInternalServerError, "OPENAI_AUTO_WARMUP_REQUEST_FAILED", "encode request: %v", err)
	}

	requestCtx, cancel := context.WithTimeout(ctx, openAIAutoWarmupRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, chatgptCodexURL, bytes.NewReader(body))
	if err != nil {
		return nil, infraerrors.Newf(http.StatusInternalServerError, "OPENAI_AUTO_WARMUP_REQUEST_FAILED", "create request: %v", err)
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	authHeaders, err := s.buildOpenAIAuthenticationHeaders(requestCtx, account, accessToken)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "OPENAI_AUTO_WARMUP_AUTH_FAILED", "build authentication: %v", err)
	}
	for key, values := range authHeaders {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	if err := resolveAndSetOpenAIChatGPTAccountHeaders(requestCtx, s.accountRepo, req.Header, account); err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "OPENAI_AUTO_WARMUP_AUTH_FAILED", "resolve ChatGPT account identity: %v", err)
	}
	req.Host = "chatgpt.com"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("OpenAI-Beta", "responses=experimental")
	enforceCodexIdentityHeadersWithUA(req.Header, s.codexIdentityOverrideUA(account))

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	if _, err := s.validateOpenAIAutoWarmupDispatch(requestCtx, accountID); err != nil {
		return &OpenAIAutoWarmupResult{Model: model}, err
	}
	started := time.Now()
	resp, err := s.doOpenAIUpstream(req, proxyURL, account)
	latency := time.Since(started)
	if err != nil {
		return &OpenAIAutoWarmupResult{Model: model, Latency: latency}, infraerrors.Newf(http.StatusBadGateway, "OPENAI_AUTO_WARMUP_UPSTREAM_FAILED", "send request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, openAIAutoWarmupRequestLimit+1))
	if err != nil {
		return &OpenAIAutoWarmupResult{Model: model, RequestID: strings.TrimSpace(resp.Header.Get("x-request-id")), Latency: latency}, infraerrors.Newf(http.StatusBadGateway, "OPENAI_AUTO_WARMUP_RESPONSE_FAILED", "read response: %v", err)
	}
	if len(responseBody) > openAIAutoWarmupRequestLimit {
		return &OpenAIAutoWarmupResult{Model: model, RequestID: strings.TrimSpace(resp.Header.Get("x-request-id")), Latency: latency}, infraerrors.New(http.StatusBadGateway, "OPENAI_AUTO_WARMUP_RESPONSE_TOO_LARGE", "warm-up response exceeded the read limit")
	}
	if snapshot := ParseCodexRateLimitHeaders(resp.Header); snapshot != nil {
		s.updateCodexUsageSnapshot(requestCtx, account.ID, snapshot)
	}
	result := &OpenAIAutoWarmupResult{
		Model: model, RequestID: strings.TrimSpace(resp.Header.Get("x-request-id")),
		ResponseID: extractOpenAIResponseIDFromJSONBytes(responseBody), Latency: latency,
	}
	if usage, ok := extractOpenAIUsageFromJSONBytes(responseBody); ok {
		result.Usage = usage
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		s.handleOpenAIAccountUpstreamError(requestCtx, account, resp.StatusCode, resp.Header, responseBody, model)
		return result, infraerrors.Newf(http.StatusBadGateway, "OPENAI_AUTO_WARMUP_UPSTREAM_REJECTED", "OpenAI warm-up request returned status %d", resp.StatusCode)
	}
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		return result, infraerrors.New(http.StatusBadGateway, "OPENAI_AUTO_WARMUP_RESPONSE_UNEXPECTED", "OpenAI warm-up returned an unexpected streaming response")
	}
	return result, nil
}

func (s *OpenAIGatewayService) validateOpenAIAutoWarmupDispatch(ctx context.Context, accountID int64) (*Account, error) {
	if s.settingService == nil {
		return nil, infraerrors.Conflict("OPENAI_AUTO_WARMUP_DISABLED", "OpenAI Auto Warm-up is disabled")
	}
	settings, err := s.settingService.GetAllSettings(ctx)
	if err != nil || settings == nil || !settings.OpenAIAutoWarmupEnabled {
		return nil, infraerrors.Conflict("OPENAI_AUTO_WARMUP_DISABLED", "OpenAI Auto Warm-up is disabled")
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil {
		return nil, firstError(err, ErrAccountNotFound)
	}
	if err := validateOpenAIAutoWarmupAccount(account); err != nil {
		return nil, err
	}
	return account, nil
}

func validateOpenAIAutoWarmupAccount(account *Account) error {
	used5h, hasUsed5h := resolveAccountExtraNumber(account.Extra, "codex_5h_used_percent")
	if !ResolveOpenAIAutoWarmupEnabled(account) || !account.IsSchedulable() || account.RateLimitedAt != nil || account.RateLimitResetAt != nil ||
		!hasUsed5h || math.IsNaN(used5h) || math.IsInf(used5h, 0) || used5h < 0 || used5h >= 100 {
		return infraerrors.Conflict("OPENAI_AUTO_WARMUP_ACCOUNT_UNSAFE", "account is not eligible for Auto Warm-up")
	}
	return nil
}

func (s *OpenAIGatewayService) resolveOpenAIAutoWarmupModel(ctx context.Context, account *Account, accessToken string) (string, error) {
	refreshed := *account
	refreshed.Credentials = cloneOpenAIAutoResetExtra(account.Credentials)
	if refreshed.Credentials == nil {
		refreshed.Credentials = make(map[string]any)
	}
	if strings.TrimSpace(accessToken) != "" {
		refreshed.Credentials["access_token"] = accessToken
	}
	manifest, err := s.FetchCodexModelsManifest(ctx, &refreshed, "", "")
	if err != nil {
		return "", infraerrors.Newf(http.StatusBadGateway, "OPENAI_AUTO_WARMUP_MODEL_RESOLUTION_FAILED", "fetch Codex models: %v", err)
	}
	var envelope struct {
		Models []struct {
			Slug string `json:"slug"`
		} `json:"models"`
	}
	if manifest == nil || json.Unmarshal(manifest.Body, &envelope) != nil {
		return "", infraerrors.New(http.StatusBadGateway, "OPENAI_AUTO_WARMUP_MODEL_RESOLUTION_FAILED", "Codex models manifest is invalid")
	}
	for _, model := range envelope.Models {
		if slug := strings.TrimSpace(model.Slug); slug != "" {
			return slug, nil
		}
	}
	return "", infraerrors.New(http.StatusBadGateway, "OPENAI_AUTO_WARMUP_MODEL_UNAVAILABLE", "Codex models manifest has no usable model")
}

func firstError(primary, fallback error) error {
	if primary != nil {
		return primary
	}
	return fallback
}
