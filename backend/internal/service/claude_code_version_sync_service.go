package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const (
	claudeCodeVersionSyncInterval = time.Hour
	claudeCodeVersionSyncTimeout  = 30 * time.Second
	claudeCodeVersionSyncRepo     = "anthropics/claude-code"
	claudeCodeVersionSyncPerPage  = 30
	claudeCodeVersionTagPrefix    = "v"
)

type ClaudeCodeVersionSyncService struct {
	settingRepo    SettingRepository
	settingService *SettingService
	githubClient   GitHubReleaseClient
	interval       time.Duration
	stopCh         chan struct{}
	stopOnce       sync.Once
	wg             sync.WaitGroup
}

func NewClaudeCodeVersionSyncService(
	settingRepo SettingRepository,
	settingService *SettingService,
	githubClient GitHubReleaseClient,
	interval time.Duration,
) *ClaudeCodeVersionSyncService {
	return &ClaudeCodeVersionSyncService{
		settingRepo:    settingRepo,
		settingService: settingService,
		githubClient:   githubClient,
		interval:       interval,
		stopCh:         make(chan struct{}),
	}
}

func (svc *ClaudeCodeVersionSyncService) Start() {
	if svc == nil || svc.settingRepo == nil || svc.githubClient == nil || svc.interval <= 0 {
		return
	}
	svc.wg.Add(1)
	go func() {
		defer svc.wg.Done()
		ticker := time.NewTicker(svc.interval)
		defer ticker.Stop()

		svc.runInitial()
		for {
			select {
			case <-ticker.C:
				svc.runOnce()
			case <-svc.stopCh:
				return
			}
		}
	}()
}

func (svc *ClaudeCodeVersionSyncService) Stop() {
	if svc == nil {
		return
	}
	svc.stopOnce.Do(func() {
		close(svc.stopCh)
	})
	svc.wg.Wait()
}

func (svc *ClaudeCodeVersionSyncService) runInitial() {
	if svc.syncedWithinInterval() {
		return
	}
	svc.runOnce()
}

func (svc *ClaudeCodeVersionSyncService) syncedWithinInterval() bool {
	if svc.interval <= 0 {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), claudeCodeVersionSyncTimeout)
	defer cancel()

	setting, err := svc.settingRepo.Get(ctx, SettingKeyClaudeCodeClientVersionSynced)
	if err != nil || setting == nil || setting.UpdatedAt.IsZero() {
		return false
	}
	if NormalizeClaudeCodeClientVersion(setting.Value) == "" {
		return false
	}
	return time.Since(setting.UpdatedAt) < svc.interval
}

func (svc *ClaudeCodeVersionSyncService) runOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), claudeCodeVersionSyncTimeout)
	defer cancel()

	if !svc.autoSyncEnabled(ctx) {
		return
	}

	latest := svc.fetchLatestStableVersion(ctx)
	if latest == "" {
		return
	}

	current, err := svc.settingRepo.GetValue(ctx, SettingKeyClaudeCodeClientVersionSynced)
	if err != nil && !errors.Is(err, ErrSettingNotFound) {
		slog.Warn("claude_code_version_sync_current_read_failed", "error", err)
		return
	}
	current = NormalizeClaudeCodeClientVersion(current)
	if current != "" && CompareVersions(latest, current) <= 0 {
		return
	}
	if err := svc.settingRepo.Set(ctx, SettingKeyClaudeCodeClientVersionSynced, latest); err != nil {
		slog.Warn("claude_code_version_sync_persist_failed", "version", latest, "error", err)
		return
	}
	svc.settingService.InvalidateClaudeCodeClientVersionCache()
	slog.Info("claude_code_version_synced", "previous", current, "version", latest)
}

func (svc *ClaudeCodeVersionSyncService) fetchLatestStableVersion(ctx context.Context) string {
	release, err := svc.githubClient.FetchLatestRelease(ctx, claudeCodeVersionSyncRepo)
	if err != nil {
		slog.Warn("claude_code_version_sync_latest_fetch_failed", "error", err)
	} else if version := latestClaudeCodeStableReleaseVersion([]*GitHubRelease{release}); version != "" {
		return version
	}

	releases, err := svc.githubClient.FetchRecentReleases(ctx, claudeCodeVersionSyncRepo, claudeCodeVersionSyncPerPage)
	if err != nil {
		slog.Warn("claude_code_version_sync_fetch_failed", "error", err)
		return ""
	}
	version := latestClaudeCodeStableReleaseVersion(releases)
	if version == "" {
		slog.Warn("claude_code_version_sync_no_stable_release", "repo", claudeCodeVersionSyncRepo)
	}
	return version
}

func (svc *ClaudeCodeVersionSyncService) autoSyncEnabled(ctx context.Context) bool {
	value, err := svc.settingRepo.GetValue(ctx, SettingKeyClaudeCodeVersionAutoSyncEnabled)
	if err != nil {
		return true
	}
	if strings.TrimSpace(value) == "" {
		return true
	}
	return strings.TrimSpace(value) == "true"
}

func latestClaudeCodeStableReleaseVersion(releases []*GitHubRelease) string {
	best := ""
	for _, release := range releases {
		if release == nil || release.Draft || release.Prerelease {
			continue
		}
		tag := strings.TrimSpace(release.TagName)
		if !strings.HasPrefix(tag, claudeCodeVersionTagPrefix) {
			continue
		}
		version := NormalizeClaudeCodeClientVersion(strings.TrimPrefix(tag, claudeCodeVersionTagPrefix))
		if version == "" || strings.Contains(version, "-") {
			continue
		}
		if best == "" || CompareVersions(version, best) > 0 {
			best = version
		}
	}
	return best
}
