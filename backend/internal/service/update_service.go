package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/releaseinfo"
	"github.com/Wei-Shaw/sub2api/internal/updatecontract"
	"golang.org/x/mod/semver"
)

const (
	updateCacheTTL     = 20 * time.Minute
	reworkReleaseLimit = 20
	manifestAssetName  = "release-manifest.json"
)

type ReleaseState string

const (
	ReleaseStateUpToDate             ReleaseState = "up_to_date"
	ReleaseStateUpstreamAvailable    ReleaseState = "upstream_available"
	ReleaseStateCompatibilityPending ReleaseState = "compatibility_pending"
	ReleaseStateReworkBuildAvailable ReleaseState = "rework_build_available"
	ReleaseStateUpdateReady          ReleaseState = "update_ready"
	ReleaseStateUpdateBlocked        ReleaseState = "update_blocked"
	ReleaseStateUpdateFailed         ReleaseState = "update_failed"
)

type UpdateCache interface {
	GetUpdateInfo(ctx context.Context) (string, error)
	SetUpdateInfo(ctx context.Context, data string, ttl time.Duration) error
}

// GitHubReleaseClient is deliberately read-only. It cannot write files or replace binaries.
type GitHubReleaseClient interface {
	FetchLatestRelease(ctx context.Context, repo string) (*GitHubRelease, error)
	FetchRecentReleases(ctx context.Context, repo string, perPage int) ([]*GitHubRelease, error)
	FetchReleaseAsset(ctx context.Context, url string, maxSize int64) ([]byte, error)
}

type GitHubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	PublishedAt string        `json:"published_at"`
	HTMLURL     string        `json:"html_url"`
	Draft       bool          `json:"draft"`
	Prerelease  bool          `json:"prerelease"`
	Assets      []GitHubAsset `json:"assets"`
}

type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

type UpdateService struct {
	cache          UpdateCache
	githubClient   GitHubReleaseClient
	metadata       releaseinfo.Metadata
	currentVersion string
	commit         string
	buildDate      string
	buildType      string
	now            func() time.Time
}

type UpdateInfo struct {
	SchemaVersion          int                          `json:"schema_version"`
	CurrentVersion         string                       `json:"current_version"`
	CurrentGitCommit       string                       `json:"current_git_commit"`
	BuildDate              string                       `json:"build_date"`
	BuildType              string                       `json:"build_type"`
	UpdateChannel          string                       `json:"update_channel"`
	UpdatePolicy           string                       `json:"update_policy"`
	UpstreamBaseline       string                       `json:"upstream_baseline"`
	UpstreamBaselineSHA    string                       `json:"upstream_baseline_sha"`
	LatestUpstream         string                       `json:"latest_upstream"`
	LatestUpstreamURL      string                       `json:"latest_upstream_url,omitempty"`
	LatestReworkVersion    string                       `json:"latest_rework_version,omitempty"`
	LatestCompatibleRework string                       `json:"latest_compatible_rework,omitempty"`
	State                  ReleaseState                 `json:"state"`
	Installable            bool                         `json:"installable"`
	Compatibility          updatecontract.Compatibility `json:"compatibility,omitempty"`
	ReleaseDate            string                       `json:"release_date,omitempty"`
	MigrationMin           int                          `json:"migration_min,omitempty"`
	MigrationMax           int                          `json:"migration_max,omitempty"`
	MinimumUpdaterVersion  string                       `json:"minimum_updater_version,omitempty"`
	ReleaseNotes           *updatecontract.ReleaseNotes `json:"release_notes,omitempty"`
	CheckedAt              time.Time                    `json:"checked_at"`
	Cached                 bool                         `json:"cached"`
	Warning                string                       `json:"warning,omitempty"`
}

func NewUpdateService(cache UpdateCache, githubClient GitHubReleaseClient, buildInfo BuildInfo) *UpdateService {
	metadata := releaseinfo.Current()
	version := strings.TrimSpace(buildInfo.Version)
	if version == "" {
		version = metadata.ReworkVersion
	}
	return &UpdateService{
		cache: cache, githubClient: githubClient, metadata: metadata,
		currentVersion: version, commit: buildInfo.Commit, buildDate: buildInfo.Date,
		buildType: buildInfo.BuildType, now: time.Now,
	}
}

func (s *UpdateService) CheckUpdate(ctx context.Context, force bool) (*UpdateInfo, error) {
	if !force {
		if cached, err := s.getFromCache(ctx); err == nil {
			cached.Cached = true
			return cached, nil
		}
	}

	info := s.baseInfo()
	if err := s.refresh(ctx, info); err != nil {
		if cached, cacheErr := s.getFromCache(ctx); cacheErr == nil {
			cached.Cached = true
			cached.Warning = safeWatcherWarning(err)
			return cached, nil
		}
		info.State = ReleaseStateUpdateFailed
		info.Warning = safeWatcherWarning(err)
		return info, nil
	}
	s.saveToCache(ctx, info)
	return info, nil
}

func (s *UpdateService) baseInfo() *UpdateInfo {
	return &UpdateInfo{
		SchemaVersion: 1, CurrentVersion: s.currentVersion, CurrentGitCommit: s.commit,
		BuildDate: s.buildDate, BuildType: s.buildType, UpdateChannel: s.metadata.UpdateChannel,
		UpdatePolicy: s.metadata.DefaultPolicy, UpstreamBaseline: s.metadata.UpstreamBaseline,
		UpstreamBaselineSHA: s.metadata.UpstreamBaselineSHA, LatestUpstream: s.metadata.UpstreamBaseline,
		State: ReleaseStateUpToDate, CheckedAt: s.now().UTC(),
	}
}

func (s *UpdateService) refresh(ctx context.Context, info *UpdateInfo) error {
	upstream, err := s.githubClient.FetchLatestRelease(ctx, s.metadata.UpstreamRepository)
	if err != nil {
		return fmt.Errorf("upstream release check failed")
	}
	if upstream == nil || upstream.Draft || upstream.Prerelease || !validUpstreamVersion(upstream.TagName) {
		return fmt.Errorf("upstream returned no valid stable release")
	}
	info.LatestUpstream = upstream.TagName
	info.LatestUpstreamURL = upstream.HTMLURL

	releases, err := s.githubClient.FetchRecentReleases(ctx, s.metadata.ReworkRepository, reworkReleaseLimit)
	if err != nil {
		return fmt.Errorf("rework release check failed")
	}
	candidate := newestReworkRelease(releases, s.currentVersion)
	if candidate == nil {
		if semver.Compare(info.LatestUpstream, s.metadata.UpstreamBaseline) > 0 {
			info.State = ReleaseStateCompatibilityPending
		}
		return nil
	}

	version := strings.TrimPrefix(candidate.TagName, "v")
	info.LatestReworkVersion = version
	assetURL := manifestAssetURL(candidate.Assets)
	if assetURL == "" {
		info.State = ReleaseStateUpdateBlocked
		info.Warning = "The newest rework release has no approved release manifest."
		return nil
	}
	data, err := s.githubClient.FetchReleaseAsset(ctx, assetURL, updatecontract.MaxManifestBytes)
	if err != nil {
		return fmt.Errorf("rework manifest fetch failed")
	}
	manifest, err := updatecontract.Parse(data, s.metadata.ArtifactRepository)
	if err != nil {
		info.State = ReleaseStateUpdateBlocked
		info.Warning = "The newest rework release manifest failed validation."
		return nil
	}
	if manifest.ReworkVersion != version || semver.Compare(manifest.UpstreamVersion, info.LatestUpstream) > 0 {
		info.State = ReleaseStateUpdateBlocked
		info.Warning = "The newest rework release manifest does not match its release identity."
		return nil
	}
	info.Compatibility = manifest.Compatibility
	info.ReleaseDate = manifest.ReleaseDate
	info.MigrationMin = manifest.MigrationMin
	info.MigrationMax = manifest.MigrationMax
	info.MinimumUpdaterVersion = manifest.MinimumUpdaterVersion
	info.ReleaseNotes = &manifest.ReleaseNotes

	if s.metadata.MigrationMax < manifest.MigrationMin || s.metadata.MigrationMax > manifest.MigrationMax {
		info.State = ReleaseStateUpdateBlocked
		info.Warning = "The release migration range does not include the installed schema."
		return nil
	}
	switch manifest.Compatibility {
	case updatecontract.CompatibilityApproved:
		info.State = ReleaseStateUpdateReady
		info.Installable = true
		info.LatestCompatibleRework = manifest.ReworkVersion
	case updatecontract.CompatibilityPending:
		info.State = ReleaseStateReworkBuildAvailable
	case updatecontract.CompatibilityBlocked:
		info.State = ReleaseStateUpdateBlocked
	}
	return nil
}

func newestReworkRelease(releases []*GitHubRelease, currentVersion string) *GitHubRelease {
	candidates := make([]*GitHubRelease, 0, len(releases))
	for _, release := range releases {
		if release == nil || release.Draft || release.Prerelease {
			continue
		}
		version := strings.TrimPrefix(release.TagName, "v")
		if updatecontract.IsReworkVersion(version) && updatecontract.CompareRework(version, currentVersion) > 0 {
			candidates = append(candidates, release)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return updatecontract.CompareRework(strings.TrimPrefix(candidates[i].TagName, "v"), strings.TrimPrefix(candidates[j].TagName, "v")) > 0
	})
	if len(candidates) == 0 {
		return nil
	}
	return candidates[0]
}

func manifestAssetURL(assets []GitHubAsset) string {
	for _, asset := range assets {
		if asset.Name == manifestAssetName && asset.Size > 0 && asset.Size <= updatecontract.MaxManifestBytes {
			return asset.BrowserDownloadURL
		}
	}
	return ""
}

func validUpstreamVersion(version string) bool {
	return strings.HasPrefix(version, "v") && semver.IsValid(version) && semver.Canonical(version) == version
}

func safeWatcherWarning(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if len(message) > 160 {
		message = message[:160]
	}
	return message
}

func (s *UpdateService) getFromCache(ctx context.Context) (*UpdateInfo, error) {
	data, err := s.cache.GetUpdateInfo(ctx)
	if err != nil {
		return nil, err
	}
	var info UpdateInfo
	if err := json.Unmarshal([]byte(data), &info); err != nil {
		return nil, err
	}
	if info.SchemaVersion != 1 || info.CurrentVersion != s.currentVersion {
		return nil, fmt.Errorf("cached release status is stale")
	}
	return &info, nil
}

func (s *UpdateService) saveToCache(ctx context.Context, info *UpdateInfo) {
	data, err := json.Marshal(info)
	if err == nil {
		_ = s.cache.SetUpdateInfo(ctx, string(data), updateCacheTTL)
	}
}
