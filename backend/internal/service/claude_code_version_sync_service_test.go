package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type claudeCodeVersionSyncSettingRepoStub struct {
	SettingRepository // 嵌入接口，未实现的方法会 panic（不应被调用）

	mu        sync.Mutex
	values    map[string]string
	getErr    error
	getErrKey string
	setErr    error
	updatedAt time.Time
	writes    []string
}

func newClaudeCodeVersionSyncSettingRepoStub(values map[string]string) *claudeCodeVersionSyncSettingRepoStub {
	if values == nil {
		values = map[string]string{}
	}
	return &claudeCodeVersionSyncSettingRepoStub{values: values}
}

func (r *claudeCodeVersionSyncSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getErr != nil && (r.getErrKey == "" || r.getErrKey == key) {
		return "", r.getErr
	}
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (r *claudeCodeVersionSyncSettingRepoStub) Set(_ context.Context, key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.setErr != nil {
		return r.setErr
	}
	r.values[key] = value
	r.writes = append(r.writes, value)
	return nil
}

func (r *claudeCodeVersionSyncSettingRepoStub) syncedWrites() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.writes...)
}

func (r *claudeCodeVersionSyncSettingRepoStub) Get(_ context.Context, key string) (*Setting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getErr != nil {
		return nil, r.getErr
	}
	value, ok := r.values[key]
	if !ok {
		return nil, ErrSettingNotFound
	}
	return &Setting{Key: key, Value: value, UpdatedAt: r.updatedAt}, nil
}

type claudeCodeVersionSyncGitHubStub struct {
	GitHubReleaseClient // 嵌入接口，未实现的方法会 panic（不应被调用）

	latest      *GitHubRelease
	latestErr   error
	latestCalls int

	releases []*GitHubRelease
	err      error
	calls    int
}

func (ginContext *claudeCodeVersionSyncGitHubStub) FetchLatestRelease(_ context.Context, _ string) (*GitHubRelease, error) {
	ginContext.latestCalls++
	if ginContext.latestErr != nil {
		return nil, ginContext.latestErr
	}
	return ginContext.latest, nil
}

func (ginContext *claudeCodeVersionSyncGitHubStub) FetchRecentReleases(_ context.Context, _ string, _ int) ([]*GitHubRelease, error) {
	ginContext.calls++
	if ginContext.err != nil {
		return nil, ginContext.err
	}
	return ginContext.releases, nil
}

func newClaudeCodeVersionSyncService(
	repo SettingRepository,
	github GitHubReleaseClient,
) *ClaudeCodeVersionSyncService {
	return NewClaudeCodeVersionSyncService(repo, &SettingService{}, github, claudeCodeVersionSyncInterval)
}

func TestLatestClaudeCodeStableReleaseVersion(test *testing.T) {
	releases := []*GitHubRelease{
		{TagName: "v2.1.281-beta.1", Prerelease: true},
		{TagName: "v2.1.280"},
		{TagName: "v2.1.279"},
		{TagName: "v2.999.0", Draft: true},
		{TagName: "not-a-tag"},
		nil,
	}

	require.Equal(test, "2.1.280", latestClaudeCodeStableReleaseVersion(releases))
	require.Empty(test, latestClaudeCodeStableReleaseVersion(nil))
	require.Empty(test, latestClaudeCodeStableReleaseVersion([]*GitHubRelease{{TagName: "not-a-tag"}}))
	require.Empty(test, latestClaudeCodeStableReleaseVersion([]*GitHubRelease{{TagName: "v2.1.281-beta.1"}}))
	require.Empty(test, latestClaudeCodeStableReleaseVersion([]*GitHubRelease{{TagName: "v2.1.9"}}))
}

func TestClaudeCodeVersionSyncWritesLatestStableVersion(test *testing.T) {
	repo := newClaudeCodeVersionSyncSettingRepoStub(nil)
	github := &claudeCodeVersionSyncGitHubStub{releases: []*GitHubRelease{
		{TagName: "v2.1.279"},
		{TagName: "v2.1.280"},
	}}

	newClaudeCodeVersionSyncService(repo, github).runOnce()

	require.Equal(test, []string{"2.1.280"}, repo.syncedWrites())
}

func TestClaudeCodeVersionSyncNeverMovesBackwards(test *testing.T) {
	repo := newClaudeCodeVersionSyncSettingRepoStub(map[string]string{
		SettingKeyClaudeCodeClientVersionSynced: "2.1.280",
	})
	github := &claudeCodeVersionSyncGitHubStub{releases: []*GitHubRelease{{TagName: "v2.1.279"}}}

	newClaudeCodeVersionSyncService(repo, github).runOnce()

	require.Empty(test, repo.syncedWrites())
}

func TestClaudeCodeVersionSyncSkippedWhenDisabled(test *testing.T) {
	repo := newClaudeCodeVersionSyncSettingRepoStub(map[string]string{
		SettingKeyClaudeCodeVersionAutoSyncEnabled: "false",
	})
	github := &claudeCodeVersionSyncGitHubStub{releases: []*GitHubRelease{{TagName: "v2.1.280"}}}

	newClaudeCodeVersionSyncService(repo, github).runOnce()

	require.Zero(test, github.latestCalls, "关闭自动同步后不应请求上游")
	require.Zero(test, github.calls, "关闭自动同步后不应请求上游")
	require.Empty(test, repo.syncedWrites())
}

func TestClaudeCodeVersionSyncEnabledByDefaultOrOnError(test *testing.T) {
	for _, tt := range []struct {
		name   string
		getErr error
		value  string
	}{
		{name: "缺失"},
		{name: "空值", value: ""},
		{name: "显式开启", value: "true"},
		{name: "读取失败", getErr: errors.New("db down")},
	} {
		test.Run(tt.name, func(test *testing.T) {
			repo := newClaudeCodeVersionSyncSettingRepoStub(map[string]string{
				SettingKeyClaudeCodeVersionAutoSyncEnabled: tt.value,
			})
			repo.getErr = tt.getErr
			repo.getErrKey = SettingKeyClaudeCodeVersionAutoSyncEnabled
			github := &claudeCodeVersionSyncGitHubStub{releases: []*GitHubRelease{{TagName: "v2.1.280"}}}

			newClaudeCodeVersionSyncService(repo, github).runOnce()

			require.Equal(test, []string{"2.1.280"}, repo.syncedWrites())
		})
	}
}

func TestClaudeCodeVersionSyncKeepsValueOnCurrentVersionReadError(test *testing.T) {
	repo := newClaudeCodeVersionSyncSettingRepoStub(map[string]string{
		SettingKeyClaudeCodeClientVersionSynced: "2.1.281",
	})
	repo.getErr = errors.New("读取已有版本失败")
	repo.getErrKey = SettingKeyClaudeCodeClientVersionSynced
	github := &claudeCodeVersionSyncGitHubStub{latest: &GitHubRelease{TagName: "v2.1.280"}}

	newClaudeCodeVersionSyncService(repo, github).runOnce()

	require.Empty(test, repo.syncedWrites(), "无法确认已有版本时不得覆盖同步值")
	require.Equal(test, "2.1.281", repo.values[SettingKeyClaudeCodeClientVersionSynced])
}

func TestClaudeCodeVersionSyncKeepsValueOnFetchError(test *testing.T) {
	repo := newClaudeCodeVersionSyncSettingRepoStub(map[string]string{
		SettingKeyClaudeCodeClientVersionSynced: "2.1.280",
	})
	github := &claudeCodeVersionSyncGitHubStub{
		latestErr: errors.New("network down"),
		err:       errors.New("network down"),
	}

	newClaudeCodeVersionSyncService(repo, github).runOnce()

	require.Equal(test, 1, github.latestCalls)
	require.Equal(test, 1, github.calls)
	require.Empty(test, repo.syncedWrites())
	value, err := repo.GetValue(context.Background(), SettingKeyClaudeCodeClientVersionSynced)
	require.NoError(test, err)
	require.Equal(test, "2.1.280", value)
}

func TestClaudeCodeVersionSyncUsesLatestReleaseEndpoint(test *testing.T) {
	repo := newClaudeCodeVersionSyncSettingRepoStub(nil)
	github := &claudeCodeVersionSyncGitHubStub{
		latest:   &GitHubRelease{TagName: "v2.1.280"},
		releases: []*GitHubRelease{{TagName: "v2.1.279"}},
	}

	newClaudeCodeVersionSyncService(repo, github).runOnce()

	require.Equal(test, 1, github.latestCalls)
	require.Zero(test, github.calls, "主路径可用时不应再拉列表页")
	require.Equal(test, []string{"2.1.280"}, repo.syncedWrites())
}

func TestClaudeCodeVersionSyncFallsBackToReleaseList(test *testing.T) {
	tests := []struct {
		name      string
		latest    *GitHubRelease
		latestErr error
	}{
		{name: "latest 前缀不符", latest: &GitHubRelease{TagName: "cli-2.1.280"}},
		{name: "latest 是预发布", latest: &GitHubRelease{TagName: "v2.1.281-beta.1", Prerelease: true}},
		{name: "latest 是草稿", latest: &GitHubRelease{TagName: "v2.1.281", Draft: true}},
		{name: "latest 抓取失败", latestErr: errors.New("network down")},
		{name: "latest 为空", latest: nil},
	}

	for _, tt := range tests {
		test.Run(tt.name, func(test *testing.T) {
			repo := newClaudeCodeVersionSyncSettingRepoStub(nil)
			github := &claudeCodeVersionSyncGitHubStub{
				latest:    tt.latest,
				latestErr: tt.latestErr,
				releases: []*GitHubRelease{
					{TagName: "v2.1.281-beta.1", Prerelease: true},
					{TagName: "v2.1.280"},
					{TagName: "v2.1.279"},
				},
			}

			newClaudeCodeVersionSyncService(repo, github).runOnce()

			require.Equal(test, 1, github.latestCalls)
			require.Equal(test, 1, github.calls)
			require.Equal(test, []string{"2.1.280"}, repo.syncedWrites())
		})
	}
}

func TestClaudeCodeVersionSyncLatestSharesFiltering(test *testing.T) {
	repo := newClaudeCodeVersionSyncSettingRepoStub(nil)
	github := &claudeCodeVersionSyncGitHubStub{
		latest:   &GitHubRelease{TagName: "v2.1.9"},
		releases: []*GitHubRelease{{TagName: "v2.1.280"}},
	}

	newClaudeCodeVersionSyncService(repo, github).runOnce()

	require.Equal(test, []string{"2.1.280"}, repo.syncedWrites())
}

func TestClaudeCodeVersionSyncStartRequiresDependencies(test *testing.T) {
	require.NotPanics(test, func() {
		svc := NewClaudeCodeVersionSyncService(nil, nil, nil, claudeCodeVersionSyncInterval)
		svc.Start()
		svc.Stop()
	})
}

func TestClaudeCodeVersionSyncInitialSkipsWhenRecentlySynced(test *testing.T) {
	repo := newClaudeCodeVersionSyncSettingRepoStub(map[string]string{
		SettingKeyClaudeCodeClientVersionSynced: "2.1.280",
	})
	repo.updatedAt = time.Now().Add(-time.Minute)
	github := &claudeCodeVersionSyncGitHubStub{releases: []*GitHubRelease{{TagName: "v2.1.281"}}}

	newClaudeCodeVersionSyncService(repo, github).runInitial()

	require.Zero(test, github.calls, "同步值仍在周期内时不应请求上游")
	require.Empty(test, repo.syncedWrites())
}

func TestClaudeCodeVersionSyncInitialRunsWhenStaleOrMissing(test *testing.T) {
	test.Run("同步值已过期", func(test *testing.T) {
		repo := newClaudeCodeVersionSyncSettingRepoStub(map[string]string{
			SettingKeyClaudeCodeClientVersionSynced: "2.1.280",
		})
		repo.updatedAt = time.Now().Add(-2 * time.Hour)
		github := &claudeCodeVersionSyncGitHubStub{releases: []*GitHubRelease{{TagName: "v2.1.281"}}}

		newClaudeCodeVersionSyncService(repo, github).runInitial()

		require.Equal(test, 1, github.calls)
		require.Equal(test, []string{"2.1.281"}, repo.syncedWrites())
	})

	test.Run("尚无同步值", func(test *testing.T) {
		repo := newClaudeCodeVersionSyncSettingRepoStub(nil)
		repo.updatedAt = time.Now()
		github := &claudeCodeVersionSyncGitHubStub{releases: []*GitHubRelease{{TagName: "v2.1.280"}}}

		newClaudeCodeVersionSyncService(repo, github).runInitial()

		require.Equal(test, 1, github.calls)
		require.Equal(test, []string{"2.1.280"}, repo.syncedWrites())
	})
}

func TestClaudeCodeVersionComparisonIsNumericNotLexical(test *testing.T) {
	require.Greater(test, CompareVersions("2.1.280", "2.1.9"), 0)

	require.Equal(test, "2.1.280", latestClaudeCodeStableReleaseVersion([]*GitHubRelease{
		{TagName: "v2.1.9"},
		{TagName: "v2.1.280"},
	}))

	repo := newClaudeCodeVersionSyncSettingRepoStub(map[string]string{
		SettingKeyClaudeCodeClientVersionSynced: "2.1.280",
	})
	github := &claudeCodeVersionSyncGitHubStub{releases: []*GitHubRelease{{TagName: "v2.1.258"}}}

	newClaudeCodeVersionSyncService(repo, github).runOnce()

	require.Empty(test, repo.syncedWrites(), "更低的版本号不得写入")
}
