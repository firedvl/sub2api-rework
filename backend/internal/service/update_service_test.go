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
		SchemaVersion: 1, ReworkVersion: "0.2.0-rework.1", UpstreamVersion: "v0.2.0",
		GitSHA: strings.Repeat("a", 40), Image: "ghcr.io/firedvl/sub2api-rework:0.2.0-rework.1",
		ImageDigest: "sha256:" + strings.Repeat("b", 64), MigrationMin: 235, MigrationMax: 239,
		ReleaseDate: "2026-09-04T12:00:00Z", Compatibility: compatibility,
		MinimumUpdaterVersion: "1.1.3", ReleaseNotes: updatecontract.ReleaseNotes{Rework: "Qualified changes"},
	}
	data, err := json.Marshal(manifest)
	require.NoError(t, err)
	return data
}

func newWatcher(client *updateServiceGitHubStub) *UpdateService {
	svc := NewUpdateService(&updateServiceCacheStub{}, client, BuildInfo{
		Version: "0.1.184-rework.8", Commit: "abc", Date: "2026-09-04T12:00:00Z", BuildType: "release",
	})
	svc.now = func() time.Time { return time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC) }
	return svc
}

func upstreamRelease(version string) *GitHubRelease {
	return &GitHubRelease{TagName: version, HTMLURL: "https://github.com/Wei-Shaw/sub2api/releases/tag/" + version}
}

func reworkRelease(tag string) *GitHubRelease {
	return &GitHubRelease{TagName: tag, Prerelease: true, Assets: []GitHubAsset{{
		Name: manifestAssetName, Size: 1024, BrowserDownloadURL: "https://github.com/firedvl/sub2api-rework/releases/download/" + tag + "/release-manifest.json",
	}}}
}

func TestUpdateWatcherUpToDateWithoutNewRework(t *testing.T) {
	info, err := newWatcher(&updateServiceGitHubStub{upstream: upstreamRelease("v0.2.0")}).CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, ReleaseStateUpToDate, info.State)
	require.False(t, info.Installable)
}

func TestUpdateWatcherNewUpstreamIsCompatibilityPending(t *testing.T) {
	info, err := newWatcher(&updateServiceGitHubStub{upstream: upstreamRelease("v0.2.1")}).CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, ReleaseStateCompatibilityPending, info.State)
	require.Empty(t, info.LatestCompatibleRework)
	require.False(t, info.Installable)
}

func TestUpdateWatcherPrereleaseUpstreamIsIgnored(t *testing.T) {
	upstream := upstreamRelease("v0.2.1-beta.1")
	upstream.Prerelease = true
	info, err := newWatcher(&updateServiceGitHubStub{upstream: upstream}).CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, ReleaseStateUpdateFailed, info.State)
	require.False(t, info.Installable)
}

func TestUpdateWatcherApprovedManifestIsReady(t *testing.T) {
	info, err := newWatcher(&updateServiceGitHubStub{
		upstream: upstreamRelease("v0.2.0"), rework: []*GitHubRelease{reworkRelease("v0.2.0-rework.1")},
		manifest: watcherManifest(t, updatecontract.CompatibilityApproved),
	}).CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, ReleaseStateUpdateReady, info.State)
	require.Equal(t, "0.2.0-rework.1", info.LatestCompatibleRework)
	require.True(t, info.Installable)
}

func TestUpdateWatcherPendingBuildIsNotInstallable(t *testing.T) {
	info, err := newWatcher(&updateServiceGitHubStub{
		upstream: upstreamRelease("v0.2.0"), rework: []*GitHubRelease{reworkRelease("v0.2.0-rework.1")},
		manifest: watcherManifest(t, updatecontract.CompatibilityPending),
	}).CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, ReleaseStateReworkBuildAvailable, info.State)
	require.False(t, info.Installable)
}

func TestUpdateWatcherBlockedBuildIsNotInstallable(t *testing.T) {
	info, err := newWatcher(&updateServiceGitHubStub{
		upstream: upstreamRelease("v0.2.0"), rework: []*GitHubRelease{reworkRelease("v0.2.0-rework.1")},
		manifest: watcherManifest(t, updatecontract.CompatibilityBlocked),
	}).CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, ReleaseStateUpdateBlocked, info.State)
	require.False(t, info.Installable)
}

func TestUpdateWatcherMissingManifestIsBlocked(t *testing.T) {
	release := reworkRelease("v0.2.0-rework.1")
	release.Assets = nil
	info, err := newWatcher(&updateServiceGitHubStub{
		upstream: upstreamRelease("v0.2.0"), rework: []*GitHubRelease{release},
	}).CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, ReleaseStateUpdateBlocked, info.State)
	require.False(t, info.Installable)
}

func TestUpdateWatcherMalformedManifestIsBlocked(t *testing.T) {
	info, err := newWatcher(&updateServiceGitHubStub{
		upstream: upstreamRelease("v0.2.0"), rework: []*GitHubRelease{reworkRelease("v0.2.0-rework.1")}, manifest: []byte(`{"image":"attacker"}`),
	}).CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, ReleaseStateUpdateBlocked, info.State)
	require.False(t, info.Installable)
}

func TestUpdateWatcherIncompatibleMigrationIsBlocked(t *testing.T) {
	var manifest updatecontract.Manifest
	require.NoError(t, json.Unmarshal(watcherManifest(t, updatecontract.CompatibilityApproved), &manifest))
	manifest.MigrationMin = 240
	manifest.MigrationMax = 240
	data, err := json.Marshal(manifest)
	require.NoError(t, err)
	info, err := newWatcher(&updateServiceGitHubStub{
		upstream: upstreamRelease("v0.2.0"), rework: []*GitHubRelease{reworkRelease("v0.2.0-rework.1")}, manifest: data,
	}).CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, ReleaseStateUpdateBlocked, info.State)
	require.False(t, info.Installable)
}

func TestUpdateWatcherCurrentReworkVersionIsUpToDate(t *testing.T) {
	info, err := newWatcher(&updateServiceGitHubStub{
		upstream: upstreamRelease("v0.2.0"), rework: []*GitHubRelease{reworkRelease("v0.1.184-rework.8")},
	}).CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, ReleaseStateUpToDate, info.State)
	require.False(t, info.Installable)
}

func TestNewestReworkReleaseFiltersAndOrdersCandidates(t *testing.T) {
	draft := reworkRelease("v0.2.0-rework.1")
	draft.Draft = true
	tests := []struct {
		name     string
		releases []*GitHubRelease
		want     string
	}{
		{name: "draft", releases: []*GitHubRelease{draft}},
		{name: "lower release", releases: []*GitHubRelease{reworkRelease("v0.1.184-rework.7")}},
		{name: "arbitrary prereleases", releases: []*GitHubRelease{
			{TagName: "v0.2.0-beta.1", Prerelease: true},
			{TagName: "v0.2.0-rc.1", Prerelease: true},
			{TagName: "random-test", Prerelease: true},
			{TagName: "nightly", Prerelease: true},
		}},
		{name: "newest qualified release", releases: []*GitHubRelease{
			reworkRelease("v0.1.184-rework.7"),
			reworkRelease("v0.2.0-rework.1"),
		}, want: "v0.2.0-rework.1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := newestReworkRelease(test.releases, "0.1.184-rework.8")
			if test.want == "" {
				require.Nil(t, candidate)
				return
			}
			require.NotNil(t, candidate)
			require.Equal(t, test.want, candidate.TagName)
		})
	}
}

func TestUpdateWatcherUsesCachedStatusWhenGitHubFails(t *testing.T) {
	cache := &updateServiceCacheStub{}
	good := NewUpdateService(cache, &updateServiceGitHubStub{upstream: upstreamRelease("v0.2.0")}, BuildInfo{Version: "0.1.184-rework.8"})
	_, err := good.CheckUpdate(context.Background(), true)
	require.NoError(t, err)

	failing := NewUpdateService(cache, &updateServiceGitHubStub{latestErr: errors.New("token in unsafe detail")}, BuildInfo{Version: "0.1.184-rework.8"})
	info, err := failing.CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.True(t, info.Cached)
	require.Equal(t, "upstream release check failed", info.Warning)
}
