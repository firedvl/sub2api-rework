package updater

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/updatecontract"
)

type ManifestFetcher interface {
	Fetch(ctx context.Context, version string) ([]byte, error)
}

type HTTPManifestFetcher struct {
	baseURL string
	client  *http.Client
}

func NewHTTPManifestFetcher(baseURL string) *HTTPManifestFetcher {
	client := &http.Client{Timeout: 30 * time.Second}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		req.Header.Del("Authorization")
		if !trustedManifestURL(req.URL) || len(via) >= 10 {
			return fmt.Errorf("manifest redirect rejected")
		}
		return nil
	}
	return &HTTPManifestFetcher{baseURL: strings.TrimSuffix(baseURL, "/"), client: client}
}

func trustedManifestURL(value *url.URL) bool {
	if value == nil || value.Scheme != "https" || value.User != nil {
		return false
	}
	switch strings.ToLower(value.Host) {
	case "github.com", "objects.githubusercontent.com", "release-assets.githubusercontent.com":
		return true
	default:
		return false
	}
}

func (f *HTTPManifestFetcher) Fetch(ctx context.Context, version string) ([]byte, error) {
	if !updatecontract.IsReworkVersion(version) {
		return nil, fmt.Errorf("invalid manifest version")
	}
	endpoint := f.baseURL + "/v" + version + "/release-manifest.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil || !trustedManifestURL(req.URL) {
		return nil, fmt.Errorf("invalid manifest URL")
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", "sub2api-rework-updater")
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch release manifest")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release manifest returned status %d", resp.StatusCode)
	}
	if resp.ContentLength > updatecontract.MaxManifestBytes {
		return nil, fmt.Errorf("release manifest is too large")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, updatecontract.MaxManifestBytes+1))
	if err != nil || len(data) > updatecontract.MaxManifestBytes {
		return nil, fmt.Errorf("release manifest is too large or unreadable")
	}
	return data, nil
}
