package repository

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type githubReleaseRoundTripFunc func(*http.Request) (*http.Response, error)

func (f githubReleaseRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func response(req *http.Request, status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req}
}

func newTestGitHubReleaseClient(transport http.RoundTripper) *githubReleaseClient {
	return &githubReleaseClient{
		apiClient: &http.Client{Transport: transport}, assetClient: &http.Client{Transport: transport},
	}
}

func TestGitHubReleaseClientScopesAuthorizationToAPI(t *testing.T) {
	client := newTestGitHubReleaseClient(nil)
	client.updateGitHubToken = "update-secret"
	req, err := client.newAPIRequest(context.Background(), "https://api.github.com/repos/test/repo")
	require.NoError(t, err)
	require.Equal(t, "Bearer update-secret", req.Header.Get("Authorization"))

	_, err = client.newAPIRequest(context.Background(), "https://example.com/repos/test/repo")
	require.Error(t, err)

	redirect, err := http.NewRequest(http.MethodGet, "https://example.com/asset", nil)
	require.NoError(t, err)
	redirect.Header.Set("Authorization", "Bearer update-secret")
	require.Error(t, githubAssetCheckRedirect(nil)(redirect, nil))
	require.Empty(t, redirect.Header.Get("Authorization"))
}

func TestGitHubReleaseClientFetchesReleaseMetadata(t *testing.T) {
	transport := githubReleaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "api.github.com", req.URL.Host)
		require.Equal(t, "Sub2API-Release-Watcher", req.Header.Get("User-Agent"))
		return response(req, http.StatusOK, `[{"tag_name":"v1.0.0"}]`), nil
	})
	client := newTestGitHubReleaseClient(transport)
	releases, err := client.FetchRecentReleases(context.Background(), "test/repo", 200)
	require.NoError(t, err)
	require.Len(t, releases, 1)
	require.Equal(t, "v1.0.0", releases[0].TagName)
}

func TestGitHubReleaseClientRejectsRepositoryInjection(t *testing.T) {
	client := newTestGitHubReleaseClient(githubReleaseRoundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("network must not be called")
		return nil, nil
	}))
	_, err := client.FetchLatestRelease(context.Background(), "owner/repo?x=/latest")
	require.Error(t, err)
}

func TestGitHubReleaseClientFetchesBoundedTrustedAsset(t *testing.T) {
	transport := githubReleaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "github.com", req.URL.Host)
		require.Empty(t, req.Header.Get("Authorization"))
		return response(req, http.StatusOK, "manifest"), nil
	})
	client := newTestGitHubReleaseClient(transport)
	data, err := client.FetchReleaseAsset(context.Background(), "https://github.com/test/repo/releases/download/v1/release-manifest.json", 8)
	require.NoError(t, err)
	require.Equal(t, "manifest", string(data))

	_, err = client.FetchReleaseAsset(context.Background(), "https://example.com/manifest.json", 8)
	require.Error(t, err)
	_, err = client.FetchReleaseAsset(context.Background(), "https://github.com/test/manifest.json", 7)
	require.ErrorContains(t, err, "exceeds")
}
