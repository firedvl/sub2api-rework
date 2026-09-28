//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type updatingProxyRepoStub struct {
	*proxyRepoStub
	proxy       *Proxy
	updateCalls int
}

func (s *updatingProxyRepoStub) GetByID(context.Context, int64) (*Proxy, error) {
	copy := *s.proxy
	return &copy, nil
}

func (s *updatingProxyRepoStub) Update(_ context.Context, proxy *Proxy) error {
	s.updateCalls++
	copy := *proxy
	s.proxy = &copy
	return nil
}

func TestBothProxyUpdateServicesUseRepositoryUpdateBoundary(t *testing.T) {
	t.Run("ProxyService", func(t *testing.T) {
		repo := &updatingProxyRepoStub{
			proxyRepoStub: &proxyRepoStub{},
			proxy:         &Proxy{ID: 9, Protocol: "http", Host: "old.example", Port: 8080, Status: StatusActive},
		}
		svc := NewProxyService(repo)
		host := "new.example"

		_, err := svc.Update(context.Background(), 9, UpdateProxyRequest{Host: &host})

		require.NoError(t, err)
		require.Equal(t, 1, repo.updateCalls)
		require.Equal(t, host, repo.proxy.Host)
	})

	t.Run("adminService", func(t *testing.T) {
		repo := &updatingProxyRepoStub{
			proxyRepoStub: &proxyRepoStub{},
			proxy: &Proxy{
				ID:             9,
				Protocol:       "http",
				Host:           "old.example",
				Port:           8080,
				Status:         StatusActive,
				FallbackMode:   FallbackModeNone,
				ExpiryWarnDays: 7,
			},
		}
		svc := &adminServiceImpl{proxyRepo: repo}
		warnDays := 7

		_, err := svc.UpdateProxy(context.Background(), 9, &UpdateProxyInput{
			Host:           "new.example",
			FallbackMode:   FallbackModeNone,
			ExpiryWarnDays: &warnDays,
		})

		require.NoError(t, err)
		require.Equal(t, 1, repo.updateCalls)
		require.Equal(t, "new.example", repo.proxy.Host)
	})
}

func TestAdminProxyUpdateCanClearCredentials(t *testing.T) {
	for _, clear := range []bool{false, true} {
		repo := &updatingProxyRepoStub{
			proxyRepoStub: &proxyRepoStub{},
			proxy: &Proxy{ID: 9, Protocol: "http", Host: "proxy.example", Port: 8080,
				Username: "old-user", Password: "old-pass", Status: StatusActive},
		}
		input := &UpdateProxyInput{FallbackMode: FallbackModeNone}
		if clear {
			empty := ""
			input.Username, input.Password = &empty, &empty
		}
		_, err := (&adminServiceImpl{proxyRepo: repo}).UpdateProxy(context.Background(), 9, input)
		require.NoError(t, err)
		if clear {
			require.Empty(t, repo.proxy.Username)
			require.Empty(t, repo.proxy.Password)
		} else {
			require.Equal(t, "old-user", repo.proxy.Username)
			require.Equal(t, "old-pass", repo.proxy.Password)
		}
	}
}

func TestAdminProxyPartialUpdatePreservesOmittedSettings(t *testing.T) {
	expires := time.Now().Add(time.Hour)
	backup := int64(10)
	repo := &updatingProxyRepoStub{proxyRepoStub: &proxyRepoStub{},
		proxy: &Proxy{ID: 9, ExpiresAt: &expires, FallbackMode: FallbackModeProxy, BackupProxyID: &backup, ExpiryWarnDays: 7}}
	svc := &adminServiceImpl{proxyRepo: repo}
	got, err := svc.UpdateProxy(context.Background(), 9, &UpdateProxyInput{Status: "inactive"})
	require.NoError(t, err)
	require.Equal(t, &expires, got.ExpiresAt)
	require.Equal(t, FallbackModeProxy, got.FallbackMode)
	require.Equal(t, &backup, got.BackupProxyID)
	require.Equal(t, 7, got.ExpiryWarnDays)
	_, err = svc.UpdateProxy(context.Background(), 9, &UpdateProxyInput{ClearBackupID: true})
	require.Error(t, err, "cannot clear the backup while proxy fallback remains selected")
	zero := 0
	got, err = svc.UpdateProxy(context.Background(), 9, &UpdateProxyInput{
		ClearExpiresAt: true, FallbackMode: FallbackModeNone, ClearBackupID: true, ExpiryWarnDays: &zero})
	require.NoError(t, err)
	require.Nil(t, got.ExpiresAt)
	require.Nil(t, got.BackupProxyID)
	require.Equal(t, 0, got.ExpiryWarnDays)
}
