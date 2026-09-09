package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// AccountExtraImagesURLToB64JSON 是账户 extra 中的开关键。开启后，Images 端点的非流式响应里
// b64_json 缺失或为空但 url 非空的图片项，由网关下载 url 内容并以 base64 回填到 b64_json。
const AccountExtraImagesURLToB64JSON = "images_url_to_b64_json"

// openAIImageURLDownloadTimeout 是单张图片 url 下载的超时上限。
const openAIImageURLDownloadTimeout = 60 * time.Second

// ImagesURLToB64JSONEnabled 返回账户是否开启了 url 转 b64_json 回填。
func ImagesURLToB64JSONEnabled(account *Account) bool {
	return account != nil && account.getExtraBool(AccountExtraImagesURLToB64JSON)
}

// backfillOpenAIImagesB64JSON 对 Images 端点的非流式响应做 url 转 b64_json 回填。
//
// 只处理 data[i].b64_json 缺失或为空且 url 非空的项；url 字段原样保留。
// 单项失败仅记日志并保留该项原样，响应整体照常返回。
// 客户端显式要求 response_format=url 时不做回填。
func (s *OpenAIGatewayService) backfillOpenAIImagesB64JSON(
	ctx context.Context,
	account *Account,
	parsed *OpenAIImagesRequest,
	body []byte,
) []byte {
	if !ImagesURLToB64JSONEnabled(account) {
		return body
	}
	if parsed != nil && parsed.ResponseFormat == "url" {
		return body
	}
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return body
	}
	items := gjson.GetBytes(body, "data")
	if !items.IsArray() {
		return body
	}
	for index, item := range items.Array() {
		if !item.IsObject() {
			continue
		}
		if strings.TrimSpace(item.Get("b64_json").String()) != "" {
			continue
		}
		rawURL := strings.TrimSpace(item.Get("url").String())
		if rawURL == "" {
			continue
		}
		encoded, err := s.fetchOpenAIImageURLBase64(ctx, account, rawURL)
		if err != nil {
			logger.LegacyPrintf(
				"service.openai_gateway",
				"[OpenAI] Images b64_json backfill skipped account_id=%d index=%d err=%s",
				account.ID,
				index,
				sanitizeUpstreamErrorMessage(err.Error()),
			)
			continue
		}
		updated, err := sjson.SetBytes(body, fmt.Sprintf("data.%d.b64_json", index), encoded)
		if err != nil {
			logger.LegacyPrintf(
				"service.openai_gateway",
				"[OpenAI] Images b64_json backfill skipped account_id=%d index=%d err=%s",
				account.ID,
				index,
				sanitizeUpstreamErrorMessage(err.Error()),
			)
			continue
		}
		body = updated
	}
	return body
}

// fetchOpenAIImageURLBase64 取得图片 url 内容的标准 base64 编码。
// data: 形式只在本地解码；HTTP(S) 只允许直连，并在实际拨号处把经校验的公网 IP 固定下来。
// 大小上限与 OAuth 路径的单图下载一致，且前 512 字节须嗅探为 png/jpeg/webp/gif。
func (s *OpenAIGatewayService) fetchOpenAIImageURLBase64(ctx context.Context, account *Account, rawURL string) (string, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	return s.fetchOpenAIImageURLBase64WithNetwork(ctx, account, rawURL, openAIImageBackfillNetwork{
		resolver: net.DefaultResolver,
		dial:     dialer.DialContext,
	})
}

func (s *OpenAIGatewayService) fetchOpenAIImageURLBase64WithNetwork(
	ctx context.Context,
	account *Account,
	rawURL string,
	network openAIImageBackfillNetwork,
) (string, error) {
	if strings.HasPrefix(strings.ToLower(rawURL), "data:") {
		return validateOpenAIImageDataURL(rawURL)
	}
	if account == nil {
		return "", errors.New("image backfill account is not configured")
	}
	if account.ProxyID != nil || account.Proxy != nil {
		return "", errors.New("account proxy configured; direct image backfill disabled")
	}
	if s == nil {
		return "", errors.New("image backfill service is not configured")
	}
	downloadURL, err := s.validateOpenAIImageBackfillURL(rawURL)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, openAIImageURLDownloadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", errors.New("image URL request is invalid")
	}
	req.Header.Set("Accept", "image/*,*/*;q=0.8")
	client := newOpenAIImageBackfillHTTPClient(network, s.validateOpenAIImageBackfillURL)
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.New("image download request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("download image: unexpected status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, openAIImageMaxDownloadBytes+1))
	if err != nil {
		return "", errors.New("image response body could not be read")
	}
	if int64(len(data)) > openAIImageMaxDownloadBytes {
		return "", fmt.Errorf("downloaded image exceeds %d bytes", openAIImageMaxDownloadBytes)
	}
	if len(data) == 0 {
		return "", errors.New("download image: empty body")
	}
	if !isBackfillImageContent(data) {
		return "", errors.New("download image: content is not an allowed image format")
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

func validateOpenAIImageDataURL(raw string) (string, error) {
	comma := strings.IndexByte(raw, ',')
	if comma < 0 {
		return "", errors.New("image data URL is invalid")
	}
	metadata := strings.Split(strings.ToLower(raw[len("data:"):comma]), ";")
	if len(metadata) != 2 || metadata[1] != "base64" {
		return "", errors.New("image data URL is invalid")
	}
	if _, ok := openAIImageBackfillContentTypes[metadata[0]]; !ok {
		return "", errors.New("image data URL content type is not allowed")
	}
	payload := strings.TrimSpace(raw[comma+1:])
	maxEncoded := ((openAIImageMaxDownloadBytes + 2) / 3) * 4
	if len(payload) > maxEncoded {
		return "", fmt.Errorf("image data exceeds %d bytes", openAIImageMaxDownloadBytes)
	}
	if payload == "" {
		return "", errors.New("image data URL is invalid")
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil || int64(len(data)) > openAIImageMaxDownloadBytes {
		return "", errors.New("image data URL is invalid")
	}
	if !isBackfillImageContent(data) {
		return "", errors.New("image data URL content is not an allowed image format")
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// openAIImageBackfillContentTypes 是允许回填的图片格式，以字节嗅探结果为准，响应头不作为依据。
var openAIImageBackfillContentTypes = map[string]struct{}{
	"image/png":  {},
	"image/jpeg": {},
	"image/webp": {},
	"image/gif":  {},
}

// isBackfillImageContent 报告 data 的前 512 字节是否嗅探为允许回填的图片格式。
func isBackfillImageContent(data []byte) bool {
	_, ok := openAIImageBackfillContentTypes[detectedImageContentType(data)]
	return ok
}
