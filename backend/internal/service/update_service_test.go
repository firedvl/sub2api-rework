//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/updatecontract"
	"github.com/stretchr/testify/require"
)

type updateServiceCacheStub struct{ data string }

func (s *updateServiceCacheStub) GetUpdateInfo(context.Context) (string, error) {
	if s.data == "" {
		return "", errors.New("cache miss")
	}
	return s.data, nil
}
func (s *updateServiceCacheStub) SetUpdateInfo(_ context.Context, data string, _ time.Duration) error {
	s.data = data
	return nil
}

type updateServiceGitHubStub struct {
	upstream  *GitHubRelease
	rework    []*GitHubRelease
	manifest  []byte
	latestErr error
}

func (s *updateServiceGitHubStub) FetchLatestRelease(context.Context, string) (*GitHubRelease, error) {
	return s.upstream, s.latestErr
}
func (s *updateServiceGitHubStub) FetchRecentReleases(context.Context, string, int) ([]*GitHubRelease, error) {
	return s.rework, nil
}
func (s *updateServiceGitHubStub) FetchReleaseAsset(context.Context, string, int64) ([]byte, error) {
	return s.manifest, nil
}

func watcherManifest(t *testing.T, compatibility updatecontract.Compatibility) []byte {
	t.Helper()
	manifest := updatecontract.Manifest{
		SchemaVersion: 1, ReworkVersion: "0.1.184-rework.1", UpstreamVersion: "v0.1.184",
		GitSHA: strings.Repeat("a", 40), Image: "ghcr.io/firedvl/sub2api-rework:0.1.184-rework.1",
		ImageDigest: "sha256:" + strings.Repeat("b", 64), MigrationMin: 232, MigrationMax: 233,
		ReleaseDate: "2026-08-28T12:00:00Z", Compatibility: compatibility,
		MinimumUpdaterVersion: "1.0.0", ReleaseNotes: updatecontract.ReleaseNotes{Rework: "Qualified changes"},
	}
	data, err := json.Marshal(manifest)
	require.NoError(t, err)
	return data
}

func newWatcher(client *updateServiceGitHubStub) *UpdateService {
	svc := NewUpdateService(&updateServiceCacheStub{}, client, BuildInfo{
		Version: "0.1.183-rework.1", Commit: "abc", Date: "2026-08-28T12:00:00Z", BuildType: "release",
	})
	svc.now = func() time.Time { return time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC) }
	return svc
}

func upstreamRelease(version string) *GitHubRelease {
	return &GitHubRelease{TagName: version, HTMLURL: "https://github.com/Wei-Shaw/sub2api/releases/tag/" + version}
}

func reworkRelease() *GitHubRelease {
	return &GitHubRelease{TagName: "v0.1.184-rework.1", Assets: []GitHubAsset{{
		Name: manifestAssetName, Size: 1024, BrowserDownloadURL: "https://github.com/firedvl/sub2api-rework/releases/download/v0.1.184-rework.1/release-manifest.json",
	}}}
}

func TestUpdateWatcherUpToDateWithoutNewRework(t *testing.T) {
	info, err := newWatcher(&updateServiceGitHubStub{upstream: upstreamRelease("v0.1.183")}).CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, ReleaseStateUpToDate, info.State)
	require.False(t, info.Installable)
}

func TestUpdateWatcherNewUpstreamIsCompatibilityPending(t *testing.T) {
	info, err := newWatcher(&updateServiceGitHubStub{upstream: upstreamRelease("v0.1.184")}).CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, ReleaseStateCompatibilityPending, info.State)
	require.Empty(t, info.LatestCompatibleRework)
	require.False(t, info.Installable)
}

func TestUpdateWatcherApprovedManifestIsReady(t *testing.T) {
	info, err := newWatcher(&updateServiceGitHubStub{
		upstream: upstreamRelease("v0.1.184"), rework: []*GitHubRelease{reworkRelease()},
		manifest: watcherManifest(t, updatecontract.CompatibilityApproved),
	}).CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, ReleaseStateUpdateReady, info.State)
	require.Equal(t, "0.1.184-rework.1", info.LatestCompatibleRework)
	require.True(t, info.Installable)
}

func TestUpdateWatcherPendingBuildIsNotInstallable(t *testing.T) {
	info, err := newWatcher(&updateServiceGitHubStub{
		upstream: upstreamRelease("v0.1.184"), rework: []*GitHubRelease{reworkRelease()},
		manifest: watcherManifest(t, updatecontract.CompatibilityPending),
	}).CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, ReleaseStateReworkBuildAvailable, info.State)
	require.False(t, info.Installable)
}

func TestUpdateWatcherMalformedManifestIsBlocked(t *testing.T) {
	info, err := newWatcher(&updateServiceGitHubStub{
		upstream: upstreamRelease("v0.1.184"), rework: []*GitHubRelease{reworkRelease()}, manifest: []byte(`{"image":"attacker"}`),
	}).CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, ReleaseStateUpdateBlocked, info.State)
	require.False(t, info.Installable)
}

func TestUpdateWatcherUsesCachedStatusWhenGitHubFails(t *testing.T) {
	cache := &updateServiceCacheStub{}
	good := NewUpdateService(cache, &updateServiceGitHubStub{upstream: upstreamRelease("v0.1.183")}, BuildInfo{Version: "0.1.183-rework.1"})
	_, err := good.CheckUpdate(context.Background(), true)
	require.NoError(t, err)

	failing := NewUpdateService(cache, &updateServiceGitHubStub{latestErr: errors.New("token in unsafe detail")}, BuildInfo{Version: "0.1.183-rework.1"})
	info, err := failing.CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.True(t, info.Cached)
	require.Equal(t, "upstream release check failed", info.Warning)
}
