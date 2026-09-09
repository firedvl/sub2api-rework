package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// 8 字节 PNG 魔数足以让字节嗅探判定为 image/png。
var b64BackfillPNGBytes = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}

func b64BackfillImageResponse(status int, contentType string, payload []byte) *http.Response {
	header := http.Header{}
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}
	return &http.Response{
		StatusCode: status,
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader(payload)),
	}
}

func b64BackfillAccount(enabled bool) *Account {
	account := &Account{
		ID:       7,
		Name:     "openai-apikey",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://relay.example.com/v1",
		},
	}
	if enabled {
		account.Extra = map[string]any{AccountExtraImagesURLToB64JSON: true}
	}
	return account
}

func TestImagesURLToB64JSONEnabled(t *testing.T) {
	require.False(t, ImagesURLToB64JSONEnabled(nil))
	require.False(t, ImagesURLToB64JSONEnabled(&Account{}))
	require.False(t, ImagesURLToB64JSONEnabled(&Account{Extra: map[string]any{AccountExtraImagesURLToB64JSON: "true"}}))
	require.False(t, ImagesURLToB64JSONEnabled(&Account{Extra: map[string]any{AccountExtraImagesURLToB64JSON: false}}))
	require.True(t, ImagesURLToB64JSONEnabled(&Account{Extra: map[string]any{AccountExtraImagesURLToB64JSON: true}}))
}

func TestBackfillOpenAIImagesB64JSON(t *testing.T) {
	wantB64 := base64.StdEncoding.EncodeToString(b64BackfillPNGBytes)

	tests := []struct {
		name          string
		enabled       bool
		parsed        *OpenAIImagesRequest
		body          string
		upstream      *httpUpstreamRecorder
		wantB64       []string
		wantDownloads int
	}{
		{
			name:          "开关关闭时原样返回",
			enabled:       false,
			body:          `{"created":1,"data":[{"url":"https://cdn.example.com/a.png"}]}`,
			upstream:      &httpUpstreamRecorder{resp: b64BackfillImageResponse(http.StatusOK, "image/png", b64BackfillPNGBytes)},
			wantB64:       []string{""},
			wantDownloads: 0,
		},
		{
			name:          "缺少 b64_json 且有 data url 时本地回填并保留 url",
			enabled:       true,
			body:          `{"created":1,"data":[{"url":"data:image/png;base64,` + wantB64 + `","revised_prompt":"a cat"}]}`,
			upstream:      &httpUpstreamRecorder{},
			wantB64:       []string{wantB64},
			wantDownloads: 0,
		},
		{
			name:          "b64_json 为空串时同样回填",
			enabled:       true,
			body:          `{"created":1,"data":[{"b64_json":"","url":"data:image/png;base64,` + wantB64 + `"}]}`,
			upstream:      &httpUpstreamRecorder{},
			wantB64:       []string{wantB64},
			wantDownloads: 0,
		},
		{
			name:          "b64_json 已有值时不下载",
			enabled:       true,
			body:          `{"created":1,"data":[{"b64_json":"aW1n","url":"https://cdn.example.com/a.png"}]}`,
			upstream:      &httpUpstreamRecorder{resp: b64BackfillImageResponse(http.StatusOK, "image/png", b64BackfillPNGBytes)},
			wantB64:       []string{"aW1n"},
			wantDownloads: 0,
		},
		{
			name:          "多项混合时只回填缺失项",
			enabled:       true,
			body:          `{"created":1,"data":[{"b64_json":"aW1n"},{"url":"data:image/png;base64,` + wantB64 + `"},{"revised_prompt":"none"}]}`,
			upstream:      &httpUpstreamRecorder{},
			wantB64:       []string{"aW1n", wantB64, ""},
			wantDownloads: 0,
		},
		{
			name:          "data URL 直接取载荷不下载",
			enabled:       true,
			body:          `{"created":1,"data":[{"url":"data:image/png;base64,` + wantB64 + `"}]}`,
			upstream:      &httpUpstreamRecorder{},
			wantB64:       []string{wantB64},
			wantDownloads: 0,
		},
		{
			name:          "客户端显式要求 url 时不回填",
			enabled:       true,
			parsed:        &OpenAIImagesRequest{ResponseFormat: "url"},
			body:          `{"created":1,"data":[{"url":"https://cdn.example.com/a.png"}]}`,
			upstream:      &httpUpstreamRecorder{resp: b64BackfillImageResponse(http.StatusOK, "image/png", b64BackfillPNGBytes)},
			wantB64:       []string{""},
			wantDownloads: 0,
		},
		{
			name:          "非 http(s) 协议的 url 不下载",
			enabled:       true,
			body:          `{"created":1,"data":[{"url":"ftp://cdn.example.com/a.png"}]}`,
			upstream:      &httpUpstreamRecorder{resp: b64BackfillImageResponse(http.StatusOK, "image/png", b64BackfillPNGBytes)},
			wantB64:       []string{""},
			wantDownloads: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: tt.upstream}
			got := svc.backfillOpenAIImagesB64JSON(context.Background(), b64BackfillAccount(tt.enabled), tt.parsed, []byte(tt.body))
			require.True(t, gjson.ValidBytes(got))
			items := gjson.GetBytes(got, "data").Array()
			require.Len(t, items, len(tt.wantB64))
			for i, want := range tt.wantB64 {
				require.Equal(t, want, items[i].Get("b64_json").String(), "data.%d.b64_json", i)
			}
			require.Len(t, tt.upstream.requests, tt.wantDownloads)
			// 除 b64_json 外的字段必须原样保留。
			original := gjson.Parse(tt.body)
			original.Get("data").ForEach(func(key, item gjson.Result) bool {
				item.ForEach(func(field, value gjson.Result) bool {
					if field.String() == "b64_json" {
						return true
					}
					require.Equal(t, value.Raw, gjson.GetBytes(got, "data."+key.String()+"."+field.String()).Raw)
					return true
				})
				return true
			})
			require.Equal(t, original.Get("created").Raw, gjson.GetBytes(got, "created").Raw)
		})
	}
}

func TestBackfillOpenAIImagesB64JSON_ProxySkipsOptionalDownload(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: b64BackfillImageResponse(http.StatusOK, "image/png", b64BackfillPNGBytes)}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := b64BackfillAccount(true)
	proxyID := int64(3)
	account.ProxyID = &proxyID
	account.Proxy = &Proxy{Protocol: "http", Host: "127.0.0.1", Port: 7890}

	body := []byte(`{"created":1,"data":[{"url":"https://cdn.example.com/a.png?sig=abc"}]}`)
	got := svc.backfillOpenAIImagesB64JSON(context.Background(), account, nil, body)
	require.Equal(t, body, got)
	require.Empty(t, upstream.requests)
}

func TestBackfillOpenAIImagesB64JSON_RejectsPrivateHosts(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: b64BackfillImageResponse(http.StatusOK, "image/png", b64BackfillPNGBytes)}
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream}
	account := b64BackfillAccount(true)

	for _, rawURL := range []string{
		"http://localhost:8080/api/v1/admin/accounts",
		"http://Foo.LocalHost/a.png",
		"http://127.0.0.1:8080/a.png",
		"https://127.0.0.1/a.png",
		"http://[::1]:8080/a.png",
		"http://10.0.0.5/a.png",
		"http://172.16.0.9:9000/bucket/a.png",
		"http://192.168.1.10:9000/bucket/a.png",
		"http://169.254.169.254/latest/meta-data/",
		"http://0.0.0.0:8080/a.png",
		"http://[fe80::1]/a.png",
		"http://[::ffff:127.0.0.1]/a.png",
	} {
		body := `{"created":1,"data":[{"url":"` + rawURL + `"}]}`
		got := svc.backfillOpenAIImagesB64JSON(context.Background(), account, nil, []byte(body))
		require.Equal(t, body, string(got), "url=%s", rawURL)
	}
	require.Empty(t, upstream.requests, "private destinations must never be requested")

}

func TestIsBackfillImageContent(t *testing.T) {
	webp := append([]byte("RIFF\x24\x00\x00\x00WEBPVP8 "), make([]byte, 8)...)
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{name: "png", data: b64BackfillPNGBytes, want: true},
		{name: "jpeg", data: []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}, want: true},
		{name: "webp", data: webp, want: true},
		{name: "gif", data: []byte("GIF89a\x01\x00\x01\x00"), want: true},
		{name: "bmp 不在允许列表", data: []byte("BM\x00\x00\x00\x00\x00\x00\x00\x00"), want: false},
		{name: "ico 不在允许列表", data: []byte{0x00, 0x00, 0x01, 0x00, 0x01, 0x00}, want: false},
		{name: "svg 文本", data: []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`), want: false},
		{name: "html", data: []byte("<html>login required</html>"), want: false},
		{name: "json", data: []byte(`{"error":"unauthorized"}`), want: false},
		{name: "空", data: nil, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isBackfillImageContent(tt.data))
		})
	}
}

func TestOpenAIImageBackfillDataURLValidation(t *testing.T) {
	want := base64.StdEncoding.EncodeToString(b64BackfillPNGBytes)
	paddedBytes := append(append([]byte(nil), b64BackfillPNGBytes...), 0)
	padded := base64.StdEncoding.EncodeToString(paddedBytes)
	for _, tt := range []struct {
		name string
		raw  string
		want string
	}{
		{name: "valid", raw: "data:image/png;base64," + want, want: want},
		{name: "valid padded", raw: "data:image/png;base64," + padded, want: padded},
		{name: "missing base64 marker", raw: "data:image/png," + want},
		{name: "malformed base64 marker", raw: "data:image/png;base64junk," + want},
		{name: "extra metadata", raw: "data:image/png;charset=utf-8;base64," + want},
		{name: "disallowed mime", raw: "data:image/svg+xml;base64," + want},
		{name: "invalid payload", raw: "data:image/png;base64,%%%"},
		{name: "mime bytes mismatch", raw: "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("not an image"))},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateOpenAIImageDataURL(tt.raw)
			if tt.want == "" {
				require.Error(t, err)
				require.Empty(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}

	oversized := "data:image/png;base64," + strings.Repeat("A", ((openAIImageMaxDownloadBytes+2)/3)*4+1)
	got, err := validateOpenAIImageDataURL(oversized)
	require.Error(t, err)
	require.Empty(t, got)
}

func TestOpenAIImageBackfillResponseControls(t *testing.T) {
	readFailure := &passthroughErrReadCloser{err: errors.New("signed-url-secret")}
	tests := []struct {
		name     string
		response func() *http.Response
		want     string
	}{
		{name: "success", response: func() *http.Response {
			return b64BackfillImageResponse(http.StatusOK, "text/plain", b64BackfillPNGBytes)
		}, want: base64.StdEncoding.EncodeToString(b64BackfillPNGBytes)},
		{name: "non 2xx", response: func() *http.Response {
			return b64BackfillImageResponse(http.StatusNotFound, "image/png", b64BackfillPNGBytes)
		}},
		{name: "non image", response: func() *http.Response {
			return b64BackfillImageResponse(http.StatusOK, "image/png", []byte("not an image"))
		}},
		{name: "oversized", response: func() *http.Response {
			return b64BackfillImageResponse(http.StatusOK, "image/png", append(b64BackfillPNGBytes, make([]byte, openAIImageMaxDownloadBytes)...))
		}},
		{name: "read failure", response: func() *http.Response {
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: readFailure}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			network, _, _ := imageDestinationNetwork(t, map[string][]net.IPAddr{"public.example": {{IP: net.ParseIP("8.8.8.8")}}}, func(*http.Request) *http.Response { return tt.response() })
			cfg := &config.Config{}
			cfg.Security.URLAllowlist.AllowInsecureHTTP = true
			service := &OpenAIGatewayService{cfg: cfg}
			got, err := service.fetchOpenAIImageURLBase64WithNetwork(context.Background(), b64BackfillAccount(true), "http://public.example/image", network)
			if tt.want == "" {
				require.Error(t, err)
				require.Empty(t, got)
				if tt.name == "read failure" {
					require.NotContains(t, err.Error(), "signed-url-secret")
				}
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestOpenAIImageBackfillHonorsCancellation(t *testing.T) {
	dials := 0
	network := openAIImageBackfillNetwork{
		resolver: &imageDestinationResolver{answers: map[string][]net.IPAddr{"public.example": {{IP: net.ParseIP("8.8.8.8")}}}},
		dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			dials++
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service := &OpenAIGatewayService{cfg: &config.Config{}}
	got, err := service.fetchOpenAIImageURLBase64WithNetwork(ctx, b64BackfillAccount(true), "https://public.example/image", network)
	require.Error(t, err)
	require.Empty(t, got)
	require.LessOrEqual(t, dials, 1)
}

func TestBackfillOpenAIImagesB64JSON_NonObjectBodies(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: b64BackfillImageResponse(http.StatusOK, "image/png", b64BackfillPNGBytes)}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := b64BackfillAccount(true)

	for _, body := range []string{``, `not json`, `{"created":1}`, `{"created":1,"data":{}}`, `{"created":1,"data":[]}`, `{"created":1,"data":["x"]}`} {
		got := svc.backfillOpenAIImagesB64JSON(context.Background(), account, nil, []byte(body))
		require.Equal(t, body, string(got), "body=%q", body)
	}
	require.Empty(t, upstream.requests)
}

func TestOpenAIGatewayServiceForwardImages_APIKeyBackfillsB64JSONFromDataURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","size":"1024x1024"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Set("api_key", &APIKey{ID: 42})

	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)

	wantB64 := base64.StdEncoding.EncodeToString(b64BackfillPNGBytes)
	dataURL := "data:image/png;base64," + wantB64
	upstream := &httpUpstreamRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{"application/json"},
					"X-Request-Id": []string{"req_img_url"},
				},
				Body: io.NopCloser(strings.NewReader(
					`{"created":1710000000,"data":[{"url":"` + dataURL + `","revised_prompt":"a cat"}],"usage":{"input_tokens":10,"output_tokens":20}}`,
				)),
			},
		},
	}
	svc.httpUpstream = upstream

	result, err := svc.ForwardImages(context.Background(), c, b64BackfillAccount(true), body, parsed, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, result.ImageCount)
	require.False(t, result.Stream)

	require.Len(t, upstream.requests, 1)
	require.Equal(t, http.MethodPost, upstream.requests[0].Method)
	require.Equal(t, "https://relay.example.com/v1/images/generations", upstream.requests[0].URL.String())

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, wantB64, gjson.Get(rec.Body.String(), "data.0.b64_json").String())
	require.Equal(t, dataURL, gjson.Get(rec.Body.String(), "data.0.url").String())
	require.Equal(t, "a cat", gjson.Get(rec.Body.String(), "data.0.revised_prompt").String())
	require.Equal(t, int64(1710000000), gjson.Get(rec.Body.String(), "created").Int())
}

func TestOpenAIGatewayServiceForwardImages_APIKeyLeavesURLOnlyResponseWhenDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","size":"1024x1024"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Set("api_key", &APIKey{ID: 42})

	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)

	upstreamBody := `{"created":1710000000,"data":[{"url":"https://cdn.example.com/cat.png"}]}`
	upstream := &httpUpstreamRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(upstreamBody)),
			},
			b64BackfillImageResponse(http.StatusOK, "image/png", b64BackfillPNGBytes),
		},
	}
	svc.httpUpstream = upstream

	result, err := svc.ForwardImages(context.Background(), c, b64BackfillAccount(false), body, parsed, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, result.ImageCount)

	require.Len(t, upstream.requests, 1)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, upstreamBody, rec.Body.String())
}

func TestOpenAIGatewayServiceForwardImages_ProxySkipsBackfillAndPreservesResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","size":"1024x1024"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Set("api_key", &APIKey{ID: 42})

	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)
	upstreamBody := `{"created":1710000000,"data":[{"url":"https://cdn.example.com/cat.png?sig=synthetic"}]}`
	upstream := &httpUpstreamRecorder{responses: []*http.Response{{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}}
	svc.httpUpstream = upstream
	account := b64BackfillAccount(true)
	proxyID := int64(3)
	account.ProxyID = &proxyID
	account.Proxy = &Proxy{Protocol: "http", Host: "proxy.example", Port: 8080}

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, upstreamBody, rec.Body.String())
	require.Len(t, upstream.requests, 1)
	require.Equal(t, http.MethodPost, upstream.requests[0].Method)
}
