//go:build unit

package updater

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/updatecontract"
	"github.com/stretchr/testify/require"
)

func TestComposeArgsPreservesOrderedComposeFiles(t *testing.T) {
	for _, count := range []int{1, 2, 3} {
		policy := testPolicy(t.TempDir())
		policy.ComposeFiles = nil
		want := []string{"compose", "--project-directory", policy.DeploymentDirectory}
		for index := range count {
			path := filepath.Join(policy.DeploymentDirectory, "compose-"+string(rune('a'+index))+".yml")
			policy.ComposeFiles = append(policy.ComposeFiles, path)
			want = append(want, "-f", path)
		}
		want = append(want, "--env-file", policy.EnvironmentFile, "config", "--services")

		require.Equal(t, want, (&Service{policy: policy}).composeArgs("config", "--services"))
	}
}

func TestPolicyRejectsUnsafeComposeFileSets(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name   string
		mutate func(*Policy)
	}{
		{"schema v1", func(policy *Policy) { policy.SchemaVersion = 1 }},
		{"empty", func(policy *Policy) { policy.ComposeFiles = nil }},
		{"duplicate", func(policy *Policy) { policy.ComposeFiles = append(policy.ComposeFiles, policy.ComposeFiles[0]) }},
		{"relative", func(policy *Policy) { policy.ComposeFiles = []string{"docker-compose.yml"} }},
		{"outside deployment", func(policy *Policy) { policy.ComposeFiles = []string{filepath.Join(root, "outside.yml")} }},
		{"directive injection", func(policy *Policy) {
			policy.ComposeFiles = []string{policy.DeploymentDirectory + "/compose.yml\nReadWritePaths=/"}
		}},
		{"filesystem root", func(policy *Policy) {
			policy.DeploymentDirectory = "/"
			policy.ComposeFiles = []string{"/docker-compose.yml"}
			policy.EnvironmentFile = "/.env"
		}},
		{"top-level directory", func(policy *Policy) {
			policy.DeploymentDirectory = "/opt"
			policy.ComposeFiles = []string{"/opt/docker-compose.yml"}
			policy.EnvironmentFile = "/opt/.env"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy := testPolicy(root)
			test.mutate(&policy)
			require.Error(t, policy.Validate())
		})
	}
}

func TestManagedComposeFilesRejectMissingAndSymlink(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		policy := testPolicy(t.TempDir())
		writeUpdaterTestDeployment(t, policy)
		policy.ComposeFiles = append(policy.ComposeFiles, filepath.Join(policy.DeploymentDirectory, "missing.yml"))
		require.Error(t, validateManagedPaths(policy))
	})
	t.Run("symlink", func(t *testing.T) {
		policy := testPolicy(t.TempDir())
		writeUpdaterTestDeployment(t, policy)
		link := filepath.Join(policy.DeploymentDirectory, "linked.yml")
		require.NoError(t, os.Symlink(policy.ComposeFiles[0], link))
		policy.ComposeFiles = []string{link}
		require.Error(t, validateManagedPaths(policy))
	})
}

func TestMergedComposeValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*renderedComposeConfig, Policy)
		want   string
	}{
		{"missing service", func(config *renderedComposeConfig, policy Policy) {
			delete(config.Services, policy.RedisService)
		}, "redis service missing"},
		{"image overridden", func(config *renderedComposeConfig, policy Policy) {
			application := config.Services[policy.ApplicationService]
			application.Image = "example.invalid/sub2api:latest"
			config.Services[policy.ApplicationService] = application
		}, "SUB2API_IMAGE"},
		{"image variable prefix", func(config *renderedComposeConfig, policy Policy) {
			application := config.Services[policy.ApplicationService]
			application.Image = "${SUB2API_IMAGE_OVERRIDE:-example.invalid/sub2api:latest}"
			config.Services[policy.ApplicationService] = application
		}, "SUB2API_IMAGE"},
		{"updater access removed", func(config *renderedComposeConfig, policy Policy) {
			application := config.Services[policy.ApplicationService]
			application.GroupAdd = nil
			config.Services[policy.ApplicationService] = application
		}, "socket access"},
		{"docker socket added", func(config *renderedComposeConfig, policy Policy) {
			application := config.Services[policy.ApplicationService]
			application.Volumes[0].Source = "/var/run/docker.sock"
			application.Volumes[0].Target = "/var/run/docker.sock"
			config.Services[policy.ApplicationService] = application
		}, "Docker socket"},
		{"docker socket parent added", func(config *renderedComposeConfig, policy Policy) {
			application := config.Services[policy.ApplicationService]
			application.Volumes[0].Source = "/var/run"
			application.Volumes[0].Target = "/host-run"
			config.Services[policy.ApplicationService] = application
		}, "Docker socket"},
		{"filesystem root added", func(config *renderedComposeConfig, policy Policy) {
			application := config.Services[policy.ApplicationService]
			application.Volumes[0].Source = "/"
			application.Volumes[0].Target = "/host"
			config.Services[policy.ApplicationService] = application
		}, "Docker socket"},
		{"volumes_from added", func(config *renderedComposeConfig, policy Policy) {
			application := config.Services[policy.ApplicationService]
			application.VolumesFrom = []string{"helper"}
			config.Services[policy.ApplicationService] = application
		}, "volumes_from"},
		{"bind-backed named volume added", func(config *renderedComposeConfig, policy Policy) {
			application := config.Services[policy.ApplicationService]
			application.Volumes = append(application.Volumes, renderedComposeMount{
				Type: "volume", Source: "host-run", Target: "/host-run",
			})
			config.Services[policy.ApplicationService] = application
			config.Volumes = map[string]renderedComposeVolume{
				"host-run": {
					Name:       "project_host-run",
					DriverOpts: map[string]string{"type": "none", "o": "bind", "device": "/var/run"},
				},
			}
		}, "Docker socket"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := &fakeRunner{}
			service, policy := newUpdaterTestService(t, runner)
			var config renderedComposeConfig
			require.NoError(t, json.Unmarshal([]byte(runner.composeConfig), &config))
			test.mutate(&config, policy)
			data, err := json.Marshal(config)
			require.NoError(t, err)
			runner.composeConfig = string(data)

			err = service.validateDeploymentFiles(context.Background())

			require.ErrorContains(t, err, test.want)
		})
	}
}

func TestMergedComposeValidationRejectsUnsafeExistingNamedVolume(t *testing.T) {
	runner := &fakeRunner{volumeInspect: `{"Driver":"local","Options":{"type":"none","o":"bind","device":"/var/run"}}`}
	service, policy := newUpdaterTestService(t, runner)
	var config renderedComposeConfig
	require.NoError(t, json.Unmarshal([]byte(runner.composeConfig), &config))
	application := config.Services[policy.ApplicationService]
	application.Volumes = append(application.Volumes, renderedComposeMount{
		Type: "volume", Source: "host-run", Target: "/host-run",
	})
	config.Services[policy.ApplicationService] = application
	config.Volumes = map[string]renderedComposeVolume{"host-run": {Name: "actual-host-run"}}
	data, err := json.Marshal(config)
	require.NoError(t, err)
	runner.composeConfig = string(data)

	err = service.validateDeploymentFiles(context.Background())

	require.ErrorContains(t, err, "Docker socket")
}

func TestMergedComposeValidationUsesInterpolatedRuntimeModel(t *testing.T) {
	runner := &fakeRunner{}
	service, policy := newUpdaterTestService(t, runner)
	var config renderedComposeConfig
	require.NoError(t, json.Unmarshal([]byte(runner.composeConfig), &config))
	application := config.Services[policy.ApplicationService]
	application.Volumes[0].Source = "/var/run"
	application.Volumes[0].Target = "/host-run"
	config.Services[policy.ApplicationService] = application
	data, err := json.Marshal(config)
	require.NoError(t, err)
	runner.runtimeComposeConfig = string(data)

	err = service.validateDeploymentFiles(context.Background())

	require.ErrorContains(t, err, "Docker socket")
}

func TestMergedComposeValidationAllowsSupportedMounts(t *testing.T) {
	runner := &fakeRunner{volumeInspect: `{"Driver":"local","Options":null}`}
	service, policy := newUpdaterTestService(t, runner)
	var config renderedComposeConfig
	require.NoError(t, json.Unmarshal([]byte(runner.composeConfig), &config))
	application := config.Services[policy.ApplicationService]
	volume := application.Volumes[0]
	volume.Source = "/srv/sub2api-data"
	volume.Target = "/app/data"
	application.Volumes = append(application.Volumes, volume)
	application.Volumes = append(application.Volumes, renderedComposeMount{
		Type: "volume", Source: "data", Target: "/app/data",
	})
	config.Services[policy.ApplicationService] = application
	config.Volumes = map[string]renderedComposeVolume{"data": {Name: "project_data"}}
	data, err := json.Marshal(config)
	require.NoError(t, err)
	runner.composeConfig = string(data)

	require.NoError(t, service.validateDeploymentFiles(context.Background()))
}

func TestEveryOperationalComposeCommandUsesCompleteFileSet(t *testing.T) {
	runner := &fakeRunner{}
	service, policy := newUpdaterTestServiceWithPolicy(t, runner, func(policy *Policy) {
		policy.ComposeFiles = append(policy.ComposeFiles,
			filepath.Join(policy.DeploymentDirectory, "docker-compose.updater.yml"),
			filepath.Join(policy.DeploymentDirectory, "docker-compose.site.yml"),
		)
	})
	prepareUpdater(t, service)
	_, err := service.Start(updatecontract.OperationInstall, installRequest())
	require.NoError(t, err)
	require.Equal(t, updatecontract.UpdaterStateSucceeded, waitForUpdater(t, service, 5*time.Second).State)

	prefix := strings.Join((&Service{policy: policy}).composeArgs(), " ")
	required := []string{
		"config --no-interpolate --format json", "config --format json", "config --services", "pg_isready", "redis-cli ping",
		" psql ", " pg_dump ", " stop sub2api", "/app/sub2api --migrate", " up -d --no-deps sub2api",
	}
	for _, fragment := range required {
		require.True(t, runner.hasCall(fragment), fragment)
	}
	for _, call := range runner.callsSnapshot() {
		if strings.HasPrefix(call, "compose ") {
			require.True(t, strings.HasPrefix(call, prefix), call)
		}
	}
}

func TestBackupRecordsAndValidatesCompleteComposeSet(t *testing.T) {
	runner := &fakeRunner{}
	service, policy := newUpdaterTestServiceWithPolicy(t, runner, func(policy *Policy) {
		policy.ComposeFiles = append(policy.ComposeFiles, filepath.Join(policy.DeploymentDirectory, "docker-compose.updater.yml"))
	})
	prepareUpdater(t, service)
	_, err := service.Start(updatecontract.OperationInstall, installRequest())
	require.NoError(t, err)
	require.Equal(t, updatecontract.UpdaterStateSucceeded, waitForUpdater(t, service, 5*time.Second).State)
	state, err := service.store.load(policy.InitialInstalledVersion, policy.InitialMigration, Version)
	require.NoError(t, err)
	require.Len(t, state.Backup.ComposeFiles, 2)
	for index, file := range state.Backup.ComposeFiles {
		require.Equal(t, policy.ComposeFiles[index], file.OriginalPath)
		require.Equal(t, filepath.Join(state.Backup.Directory, "compose-00"+string(rune('0'+index))+".yaml"), file.BackupPath)
		require.True(t, strings.HasPrefix(file.SHA256, "sha256:"))
		info, err := os.Stat(file.BackupPath)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	}
	require.NoError(t, service.validateBackupMetadata(state.Backup))

	first := state.Backup.ComposeFiles[0]
	original, err := os.ReadFile(first.BackupPath)
	require.NoError(t, err)
	require.NoError(t, os.Remove(first.BackupPath))
	require.Error(t, service.validateBackupMetadata(state.Backup))
	require.NoError(t, os.WriteFile(first.BackupPath, original, 0600))
	require.NoError(t, os.WriteFile(first.BackupPath, []byte("tampered\n"), 0600))
	require.ErrorContains(t, service.validateBackupMetadata(state.Backup), "checksum")
}

func TestSystemdDropInUsesValidatedDeploymentDirectory(t *testing.T) {
	tests := []string{
		"/opt/sub2api",
		"/opt/sub2api-rework/deploy",
		`/srv/Sub2API deployments/100% "blue"`,
	}
	for _, deploymentDirectory := range tests {
		t.Run(deploymentDirectory, func(t *testing.T) {
			policy := testPolicy(t.TempDir())
			policy.DeploymentDirectory = deploymentDirectory
			policy.ComposeFiles = []string{filepath.Join(deploymentDirectory, "docker-compose.yml")}
			policy.EnvironmentFile = filepath.Join(deploymentDirectory, ".env")

			dropIn, err := SystemdDropIn(policy)

			require.NoError(t, err)
			require.Contains(t, string(dropIn), "[Service]\n")
			require.Contains(t, string(dropIn), "ReadWritePaths=")
			require.NotContains(t, string(dropIn), "\nReadWritePaths=/\n")
			if strings.Contains(deploymentDirectory, "%") {
				require.Equal(t, "[Service]\nReadWritePaths=\"/srv/Sub2API deployments/100%% \\\"blue\\\"\"\n", string(dropIn))
			}
		})
	}
}

func TestSystemdDropInRejectsDirectiveInjection(t *testing.T) {
	for _, unsafe := range []string{"\nReadWritePaths=/", "\rReadWritePaths=/", "\t/escape", "\x00escape"} {
		policy := testPolicy(t.TempDir())
		policy.DeploymentDirectory = "/opt/sub2api" + unsafe
		policy.ComposeFiles = []string{policy.DeploymentDirectory + "/docker-compose.yml"}
		policy.EnvironmentFile = policy.DeploymentDirectory + "/.env"
		_, err := SystemdDropIn(policy)
		require.Error(t, err)
	}
}

func TestSystemdDropInRejectsProtectedInstallationTrees(t *testing.T) {
	tests := []struct {
		deploymentDirectory string
		protectedPaths      []string
	}{
		{deploymentDirectory: "/usr/local"},
		{deploymentDirectory: "/etc/sub2api-rework", protectedPaths: []string{"/etc/sub2api-rework/custom-policy.json"}},
		{deploymentDirectory: "/etc/systemd"},
		{deploymentDirectory: "/etc/systemd/system/sub2api-rework-updater.service.d/deployment"},
	}
	for _, test := range tests {
		policy := testPolicy(t.TempDir())
		policy.DeploymentDirectory = test.deploymentDirectory
		policy.ComposeFiles = []string{filepath.Join(test.deploymentDirectory, "docker-compose.yml")}
		policy.EnvironmentFile = filepath.Join(test.deploymentDirectory, ".env")

		_, err := SystemdDropIn(policy, test.protectedPaths...)
		require.ErrorContains(t, err, "must not contain")
	}
}

func TestSystemdDropInRejectsProtectedPathSymlinkAliases(t *testing.T) {
	t.Run("deployment alias", func(t *testing.T) {
		root := t.TempDir()
		protectedRoot := filepath.Join(root, "protected")
		require.NoError(t, os.Mkdir(protectedRoot, 0700))
		protectedPath := filepath.Join(protectedRoot, "updater")
		require.NoError(t, os.WriteFile(protectedPath, []byte("test"), 0600))
		deployment := filepath.Join(root, "deployment")
		require.NoError(t, os.Symlink(protectedRoot, deployment))
		policy := testPolicy(root)
		policy.DeploymentDirectory = deployment
		policy.ComposeFiles = []string{filepath.Join(deployment, "docker-compose.yml")}
		policy.EnvironmentFile = filepath.Join(deployment, ".env")

		_, err := SystemdDropIn(policy, protectedPath)
		require.ErrorContains(t, err, "must not contain")
	})

	t.Run("protected path alias", func(t *testing.T) {
		root := t.TempDir()
		deployment := filepath.Join(root, "deployment")
		require.NoError(t, os.Mkdir(deployment, 0700))
		protectedTarget := filepath.Join(deployment, "updater")
		require.NoError(t, os.WriteFile(protectedTarget, []byte("test"), 0600))
		protectedAlias := filepath.Join(root, "updater-link")
		require.NoError(t, os.Symlink(protectedTarget, protectedAlias))
		policy := testPolicy(root)
		policy.DeploymentDirectory = deployment
		policy.ComposeFiles = []string{filepath.Join(deployment, "docker-compose.yml")}
		policy.EnvironmentFile = filepath.Join(deployment, ".env")

		_, err := SystemdDropIn(policy, protectedAlias)
		require.ErrorContains(t, err, "must not contain")
	})
}

func TestSystemdDropInAllowsDedicatedSystemSubdirectory(t *testing.T) {
	policy := testPolicy(t.TempDir())
	policy.DeploymentDirectory = "/usr/local/share/sub2api"
	policy.ComposeFiles = []string{"/usr/local/share/sub2api/docker-compose.yml"}
	policy.EnvironmentFile = "/usr/local/share/sub2api/.env"

	_, err := SystemdDropIn(policy, "/etc/sub2api-rework/custom-policy.json")
	require.NoError(t, err)
}
