//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/stretchr/testify/require"
)

func TestGetClaudeCodeClientVersionPriority(test *testing.T) {
	for _, tt := range []struct {
		name   string
		manual string
		synced string
		want   string
	}{
		{name: "手动值优先", manual: "2.1.280", synced: "2.1.281", want: "2.1.280"},
		{name: "归一化手动值", manual: " v2.1.280 ", synced: "2.1.281", want: "2.1.280"},
		{name: "跟随同步值", synced: "2.1.281", want: "2.1.281"},
		{name: "无效手动值回退", manual: "invalid", synced: "2.1.281", want: "2.1.281"},
		{name: "无效同步值回退", synced: "2.1.9", want: claude.CLIVersion()},
		{name: "未配置时回退", want: claude.CLIVersion()},
	} {
		test.Run(tt.name, func(test *testing.T) {
			repo := &authSourceDefaultsRepoStub{values: map[string]string{
				SettingKeyClaudeCodeClientVersion:       tt.manual,
				SettingKeyClaudeCodeClientVersionSynced: tt.synced,
			}}
			svc := NewSettingService(repo, &config.Config{})
			require.Equal(test, tt.want, svc.GetClaudeCodeClientVersion(context.Background()))
		})
	}
}

func TestUpdateSettingsClaudeCodeVersionTakesEffectImmediately(test *testing.T) {
	ctx := context.Background()
	repo := &authSourceDefaultsRepoStub{values: map[string]string{
		SettingKeyClaudeCodeClientVersionSynced: "2.1.281",
	}}
	svc := NewSettingService(repo, &config.Config{})
	resetGatewayForwardingSettingsCacheForTest(test)
	defer svc.refreshCachedSettings(&SystemSettings{})
	require.Equal(test, "2.1.281", svc.GetClaudeCodeClientVersion(ctx))

	settings := &SystemSettings{
		ClaudeCodeClientVersion:          "2.1.280",
		ClaudeCodeVersionAutoSyncEnabled: true,
	}
	require.NoError(test, svc.UpdateSettings(ctx, settings))
	require.Equal(test, "2.1.280", svc.GetClaudeCodeClientVersion(ctx), "保存手动版本后应立即生效")
	require.NotContains(test, repo.updates, SettingKeyClaudeCodeClientVersionSynced)
	require.Equal(test, "2.1.281", repo.values[SettingKeyClaudeCodeClientVersionSynced])

	settings.ClaudeCodeClientVersion = ""
	settings.ClaudeCodeVersionAutoSyncEnabled = false
	require.NoError(test, svc.UpdateSettings(ctx, settings))
	require.Equal(test, "2.1.281", svc.GetClaudeCodeClientVersion(ctx))
	require.Equal(test, "false", repo.values[SettingKeyClaudeCodeVersionAutoSyncEnabled])
}
