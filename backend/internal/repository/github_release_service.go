package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

var githubRepositoryPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

type githubReleaseClient struct {
	apiClient         *http.Client
	assetClient       *http.Client
	updateGitHubToken string
}

type githubReleaseClientError struct{ err error }

func NewGitHubReleaseClient(proxyURL string, allowDirectOnProxyError bool) service.GitHubReleaseClient {
	shared, err := httpclient.GetClient(httpclient.Options{Timeout: 30 * time.Second, ProxyURL: proxyURL})
	if err != nil {
		if strings.TrimSpace(proxyURL) != "" && !allowDirectOnProxyError {
			slog.Warn("proxy client init failed, release checks will fail", "service", "github_release", "error", err)
			return &githubReleaseClientError{err: fmt.Errorf("proxy client init failed and direct fallback is disabled: %w", err)}
		}
		shared = &http.Client{Timeout: 30 * time.Second}
	}
	apiClient := cloneHTTPClient(shared)
	apiClient.CheckRedirect = githubAPICheckRedirect(apiClient.CheckRedirect)
	assetClient := cloneHTTPClient(shared)
	assetClient.CheckRedirect = githubAssetCheckRedirect(assetClient.CheckRedirect)
	return &githubReleaseClient{apiClient: apiClient, assetClient: assetClient, updateGitHubToken: os.Getenv("UPDATE_GITHUB_TOKEN")}
}

func cloneHTTPClient(client *http.Client) *http.Client {
	cloned := *client
	return &cloned
}

func isGitHubAPIURL(value *url.URL) bool {
	return value != nil && strings.EqualFold(value.Scheme, "https") && value.User == nil &&
		strings.EqualFold(value.Host, "api.github.com")
}

func isGitHubAssetURL(value *url.URL) bool {
	if value == nil || !strings.EqualFold(value.Scheme, "https") || value.User != nil {
		return false
	}
	switch strings.ToLower(value.Host) {
	case "github.com", "objects.githubusercontent.com", "release-assets.githubusercontent.com":
		return true
	default:
		return false
	}
}

func githubAPICheckRedirect(previous func(*http.Request, []*http.Request) error) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if !isGitHubAPIURL(req.URL) {
			return fmt.Errorf("GitHub API redirect rejected")
		}
		if len(via) >= 10 {
			return fmt.Errorf("too many GitHub API redirects")
		}
		if previous != nil {
			return previous(req, via)
		}
		return nil
	}
}

func githubAssetCheckRedirect(previous func(*http.Request, []*http.Request) error) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		req.Header.Del("Authorization")
		if !isGitHubAssetURL(req.URL) {
			return fmt.Errorf("GitHub release asset redirect rejected")
		}
		if len(via) >= 10 {
			return fmt.Errorf("too many GitHub release asset redirects")
		}
		if previous != nil {
			return previous(req, via)
		}
		return nil
	}
}

func (c *githubReleaseClient) newAPIRequest(ctx context.Context, endpoint string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if !isGitHubAPIURL(req.URL) {
		return nil, fmt.Errorf("invalid GitHub API URL")
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Sub2API-Release-Watcher")
	if c.updateGitHubToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.updateGitHubToken)
	}
	return req, nil
}

func (c *githubReleaseClientError) FetchLatestRelease(context.Context, string) (*service.GitHubRelease, error) {
	return nil, c.err
}
func (c *githubReleaseClientError) FetchRecentReleases(context.Context, string, int) ([]*service.GitHubRelease, error) {
	return nil, c.err
}
func (c *githubReleaseClientError) FetchReleaseAsset(context.Context, string, int64) ([]byte, error) {
	return nil, c.err
}

func (c *githubReleaseClient) FetchLatestRelease(ctx context.Context, repository string) (*service.GitHubRelease, error) {
	if !githubRepositoryPattern.MatchString(repository) {
		return nil, fmt.Errorf("invalid GitHub repository")
	}
	var release service.GitHubRelease
	if err := c.fetchAPIJSON(ctx, "https://api.github.com/repos/"+repository+"/releases/latest", &release); err != nil {
		return nil, err
	}
	return &release, nil
}

func (c *githubReleaseClient) FetchRecentReleases(ctx context.Context, repository string, perPage int) ([]*service.GitHubRelease, error) {
	if !githubRepositoryPattern.MatchString(repository) {
		return nil, fmt.Errorf("invalid GitHub repository")
	}
	if perPage <= 0 {
		perPage = 10
	} else if perPage > 100 {
		perPage = 100
	}
	var releases []*service.GitHubRelease
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=%d", repository, perPage)
	if err := c.fetchAPIJSON(ctx, endpoint, &releases); err != nil {
		return nil, err
	}
	return releases, nil
}

func (c *githubReleaseClient) fetchAPIJSON(ctx context.Context, endpoint string, target any) error {
	req, err := c.newAPIRequest(ctx, endpoint)
	if err != nil {
		return err
	}
	resp, err := c.apiClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 2*1024*1024)).Decode(target)
}

func (c *githubReleaseClient) FetchReleaseAsset(ctx context.Context, rawURL string, maxSize int64) ([]byte, error) {
	if maxSize <= 0 {
		return nil, fmt.Errorf("invalid release asset size limit")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil || !isGitHubAssetURL(req.URL) {
		return nil, fmt.Errorf("invalid GitHub release asset URL")
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", "Sub2API-Release-Watcher")
	resp, err := c.assetClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub release asset returned %d", resp.StatusCode)
	}
	if resp.ContentLength > maxSize {
		return nil, fmt.Errorf("GitHub release asset exceeds %d bytes", maxSize)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxSize {
		return nil, fmt.Errorf("GitHub release asset exceeds %d bytes", maxSize)
	}
	return data, nil
}
