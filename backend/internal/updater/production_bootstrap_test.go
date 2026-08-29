//go:build unit

package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/updatecontract"
	"github.com/stretchr/testify/require"
)

type renderedComposeConfig struct {
	Services map[string]renderedComposeService `json:"services"`
	Volumes  map[string]renderedComposeVolume  `json:"volumes"`
}

func TestProductionShapedComposeValidation(t *testing.T) {
	docker, err := exec.LookPath("docker")
	if err != nil {
		t.Skip("Docker Compose is unavailable")
	}
	for _, test := range []struct {
		name             string
		postgresGroupAdd string
	}{
		{name: "production topology"},
		{name: "unrelated service representation", postgresGroupAdd: "    group_add: [70]\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			policy := testPolicy(root)
			policy.DeploymentDirectory = filepath.Join(root, "opt", "sub2api")
			policy.ComposeFiles = []string{
				filepath.Join(policy.DeploymentDirectory, "docker-compose.yml"),
				filepath.Join(policy.DeploymentDirectory, "docker-compose.updater.yml"),
			}
			policy.EnvironmentFile = filepath.Join(policy.DeploymentDirectory, ".env")
			policy.SocketPath = "/run/sub2api-rework-updater/updater.sock"
			policy.DockerBinary = docker
			require.NoError(t, os.MkdirAll(policy.DeploymentDirectory, 0700))

			var compose strings.Builder
			compose.WriteString(`services:
  sub2api:
    image: ${SUB2API_IMAGE}
    volumes:
      - /opt/sub2api/data:/app/data:Z
    environment:
      SYNTH_EMPTY: ${SYNTH_EMPTY:-}
      SYNTH_NUMBER: ${SYNTH_NUMBER:-000985}
      SYNTH_BOOLEAN: ${SYNTH_BOOLEAN:-false}
      SYNTH_SECRET: ${SYNTH_SECRET:?SYNTH_SECRET is required}
`)
			for index := range 144 {
				_, _ = fmt.Fprintf(&compose, "      SYNTHETIC_%03d: ${SYNTHETIC_%03d:-}\n", index, index)
			}
			compose.WriteString(`  postgres:
    image: postgres:18-alpine
`)
			compose.WriteString(test.postgresGroupAdd)
			compose.WriteString(`    volumes:
      - /opt/sub2api/postgres:/var/lib/postgresql/data
    environment:
      PGDATA: /var/lib/postgresql/data
      POSTGRES_USER: ${POSTGRES_USER:-sub2api}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required}
      POSTGRES_DB: ${POSTGRES_DB:-sub2api}
  redis:
    image: redis:8-alpine
    volumes:
      - /opt/sub2api/redis:/data
    environment:
      REDISCLI_AUTH: ${REDIS_PASSWORD:-}
`)
			override := `services:
  sub2api:
    group_add:
      - "${SUB2API_UPDATER_GID:?SUB2API_UPDATER_GID is required}"
    volumes:
      - type: bind
        source: /run/sub2api-rework-updater
        target: /run/sub2api-rework-updater
    environment:
      SUB2API_UPDATER_SOCKET: /run/sub2api-rework-updater/updater.sock
      SUB2API_UPDATER_GID: "${SUB2API_UPDATER_GID:?SUB2API_UPDATER_GID is required}"
`
			var environment strings.Builder
			environment.WriteString(`SUB2API_IMAGE=ghcr.io/firedvl/sub2api-rework:0.1.183-rework.3
SUB2API_UPDATER_GID=985
POSTGRES_USER=sub2api
POSTGRES_PASSWORD=synthetic-postgres-password
POSTGRES_DB=sub2api
REDIS_PASSWORD=
SYNTH_EMPTY=
SYNTH_NUMBER=000985
SYNTH_BOOLEAN=false
SYNTH_SECRET=synthetic_8f13c4a71730b9e13d6f276d65e5d2be7548e15a2ad4aa258c4bf5b0c2a16e71 # gitleaks:allow
`)
			for index := range 144 {
				_, _ = fmt.Fprintf(&environment, "SYNTHETIC_%03d=synthetic_%03d_%s\n", index, index, strings.Repeat("a", 48))
			}
			for path, data := range map[string]string{
				policy.ComposeFiles[0]: compose.String(),
				policy.ComposeFiles[1]: override,
				policy.EnvironmentFile: environment.String(),
			} {
				require.NoError(t, os.WriteFile(path, []byte(data), 0600))
			}

			service := &Service{policy: policy, runner: ExecRunner{Directory: policy.DeploymentDirectory}}
			var outputs []string
			for _, args := range [][]string{
				service.composeArgs("config", "--no-interpolate", "--format", "json"),
				service.composeArgs("config", "--format", "json"),
			} {
				output, err := service.commandOutput(context.Background(), args...)
				require.NoError(t, err)
				require.True(t, json.Valid([]byte(output)))
				require.Greater(t, len(output), 8*1024)
				outputs = append(outputs, output)
			}
			if test.postgresGroupAdd != "" {
				for index, want := range []string{"70", `"70"`} {
					var document struct {
						Services map[string]struct {
							GroupAdd []json.RawMessage `json:"group_add"`
						} `json:"services"`
					}
					require.NoError(t, json.Unmarshal([]byte(outputs[index]), &document))
					require.Equal(t, want, string(document.Services[policy.DatabaseService].GroupAdd[0]))
				}
			}

			require.NoError(t, service.validateDeploymentFiles(context.Background()))
		})
	}
}

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
		want   deploymentValidationError
	}{
		{"missing database service", func(config *renderedComposeConfig, policy Policy) {
			delete(config.Services, policy.DatabaseService)
		}, validationDatabaseService},
		{"missing redis service", func(config *renderedComposeConfig, policy Policy) {
			delete(config.Services, policy.RedisService)
		}, validationRedisService},
		{"image overridden", func(config *renderedComposeConfig, policy Policy) {
			application := config.Services[policy.ApplicationService]
			application.Image = "example.invalid/sub2api:latest"
			config.Services[policy.ApplicationService] = application
		}, validationManagedImage},
		{"image variable prefix", func(config *renderedComposeConfig, policy Policy) {
			application := config.Services[policy.ApplicationService]
			application.Image = "${SUB2API_IMAGE_OVERRIDE:-example.invalid/sub2api:latest}"
			config.Services[policy.ApplicationService] = application
		}, validationManagedImage},
		{"updater socket environment missing", func(config *renderedComposeConfig, policy Policy) {
			application := config.Services[policy.ApplicationService]
			mutateRenderedEnvironment(t, &application, "SUB2API_UPDATER_SOCKET", nil)
			config.Services[policy.ApplicationService] = application
		}, validationUpdaterSocketAccess},
		{"updater socket environment wrong", func(config *renderedComposeConfig, policy Policy) {
			application := config.Services[policy.ApplicationService]
			wrong := "/run/wrong/updater.sock"
			mutateRenderedEnvironment(t, &application, "SUB2API_UPDATER_SOCKET", &wrong)
			config.Services[policy.ApplicationService] = application
		}, validationUpdaterSocketAccess},
		{"updater gid environment missing", func(config *renderedComposeConfig, policy Policy) {
			application := config.Services[policy.ApplicationService]
			mutateRenderedEnvironment(t, &application, "SUB2API_UPDATER_GID", nil)
			config.Services[policy.ApplicationService] = application
		}, validationUpdaterSocketAccess},
		{"updater group missing", func(config *renderedComposeConfig, policy Policy) {
			application := config.Services[policy.ApplicationService]
			application.GroupAdd = nil
			config.Services[policy.ApplicationService] = application
		}, validationUpdaterSocketAccess},
		{"updater socket bind missing", func(config *renderedComposeConfig, policy Policy) {
			application := config.Services[policy.ApplicationService]
			application.Volumes = nil
			config.Services[policy.ApplicationService] = application
		}, validationUpdaterSocketAccess},
		{"docker socket added", func(config *renderedComposeConfig, policy Policy) {
			application := config.Services[policy.ApplicationService]
			application.Volumes[0].Source = "/var/run/docker.sock"
			application.Volumes[0].Target = "/var/run/docker.sock"
			config.Services[policy.ApplicationService] = application
		}, validationDockerSocket},
		{"docker socket parent added", func(config *renderedComposeConfig, policy Policy) {
			application := config.Services[policy.ApplicationService]
			application.Volumes[0].Source = "/var/run"
			application.Volumes[0].Target = "/host-run"
			config.Services[policy.ApplicationService] = application
		}, validationDockerSocket},
		{"filesystem root added", func(config *renderedComposeConfig, policy Policy) {
			application := config.Services[policy.ApplicationService]
			application.Volumes[0].Source = "/"
			application.Volumes[0].Target = "/host"
			config.Services[policy.ApplicationService] = application
		}, validationDockerSocket},
		{"volumes_from added", func(config *renderedComposeConfig, policy Policy) {
			application := config.Services[policy.ApplicationService]
			application.VolumesFrom = []string{"helper"}
			config.Services[policy.ApplicationService] = application
		}, validationVolumesFrom},
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
		}, validationDockerSocket},
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

			require.ErrorIs(t, err, test.want)
		})
	}
}

func mutateRenderedEnvironment(t *testing.T, service *renderedComposeService, name string, value *string) {
	t.Helper()
	var environment map[string]*string
	require.NoError(t, json.Unmarshal(service.Environment, &environment))
	if value == nil {
		delete(environment, name)
	} else {
		environment[name] = value
	}
	data, err := json.Marshal(environment)
	require.NoError(t, err)
	service.Environment = data
}

func TestDeploymentValidationReportsSafeStages(t *testing.T) {
	t.Run("compose file", func(t *testing.T) {
		service, policy := newUpdaterTestService(t, &fakeRunner{})
		require.NoError(t, os.Chmod(policy.ComposeFiles[0], 0620))
		require.ErrorIs(t, service.validateDeploymentFiles(context.Background()), validationComposeFile)
	})
	t.Run("environment file", func(t *testing.T) {
		service, policy := newUpdaterTestService(t, &fakeRunner{})
		require.NoError(t, os.Chmod(policy.EnvironmentFile, 0644))
		require.ErrorIs(t, service.validateDeploymentFiles(context.Background()), validationEnvironmentFile)
	})
	for _, test := range []struct {
		name   string
		mutate func(*fakeRunner)
		want   deploymentValidationError
	}{
		{"raw compose command", func(runner *fakeRunner) { runner.rawComposeError = "synthetic failure" }, validationRawComposeCommand},
		{"raw compose json", func(runner *fakeRunner) { runner.composeConfig = "{" }, validationRawComposeJSON},
		{"rendered compose command", func(runner *fakeRunner) { runner.runtimeComposeError = "synthetic failure" }, validationRenderedComposeCommand},
		{"rendered compose json", func(runner *fakeRunner) { runner.runtimeComposeConfig = "{" }, validationRenderedComposeJSON},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := &fakeRunner{}
			service, _ := newUpdaterTestService(t, runner)
			test.mutate(runner)
			require.ErrorIs(t, service.validateDeploymentFiles(context.Background()), test.want)
		})
	}
}

func TestRenderedComposeRequiresApplicationService(t *testing.T) {
	runner := &fakeRunner{}
	service, policy := newUpdaterTestService(t, runner)
	var config renderedComposeConfig
	require.NoError(t, json.Unmarshal([]byte(runner.composeConfig), &config))
	delete(config.Services, policy.ApplicationService)
	data, err := json.Marshal(config)
	require.NoError(t, err)
	runner.runtimeComposeConfig = string(data)

	require.ErrorIs(t, service.validateDeploymentFiles(context.Background()), validationApplicationService)
}

func TestMergedComposeValidationRejectsUnsupportedNamedVolume(t *testing.T) {
	runner := &fakeRunner{volumeInspect: `{"Driver":"nfs","Options":null}`}
	service, policy := newUpdaterTestService(t, runner)
	var config renderedComposeConfig
	require.NoError(t, json.Unmarshal([]byte(runner.composeConfig), &config))
	application := config.Services[policy.ApplicationService]
	application.Volumes = append(application.Volumes, renderedComposeMount{
		Type: "volume", Source: "data", Target: "/app/data",
	})
	config.Services[policy.ApplicationService] = application
	config.Volumes = map[string]renderedComposeVolume{"data": {Name: "project_data"}}
	data, err := json.Marshal(config)
	require.NoError(t, err)
	runner.composeConfig = string(data)

	require.ErrorIs(t, service.validateDeploymentFiles(context.Background()), validationNamedVolume)
}

func TestMergedComposeValidationRejectsDockerSocketSymlinkAlias(t *testing.T) {
	runner := &fakeRunner{}
	service, policy := newUpdaterTestService(t, runner)
	target := filepath.Join(t.TempDir(), "docker.sock")
	require.NoError(t, os.WriteFile(target, nil, 0600))
	alias := filepath.Join(t.TempDir(), "innocent-source")
	require.NoError(t, os.Symlink(target, alias))
	var config renderedComposeConfig
	require.NoError(t, json.Unmarshal([]byte(runner.composeConfig), &config))
	application := config.Services[policy.ApplicationService]
	application.Volumes[0] = renderedComposeMount{Type: "bind", Source: alias, Target: "/host/run"}
	config.Services[policy.ApplicationService] = application
	data, err := json.Marshal(config)
	require.NoError(t, err)
	runner.composeConfig = string(data)

	require.ErrorIs(t, service.validateDeploymentFiles(context.Background()), validationDockerSocket)
}

func TestSafeDeploymentValidationReasons(t *testing.T) {
	reasons := []deploymentValidationError{
		validationComposeFile,
		validationEnvironmentFile,
		validationRawComposeCommand,
		validationRawComposeJSON,
		validationManagedImage,
		validationRenderedComposeCommand,
		validationRenderedComposeJSON,
		validationApplicationService,
		validationDatabaseService,
		validationRedisService,
		validationVolumesFrom,
		validationDockerSocket,
		validationNamedVolume,
		validationUpdaterSocketAccess,
		validationInternal,
	}
	for _, reason := range reasons {
		t.Run(string(reason), func(t *testing.T) {
			require.EqualError(t, safeDeploymentValidationError(reason), "deployment structure check failed: "+string(reason))
		})
	}
	const secret = "synthetic-secret-that-must-not-leak"
	err := safeDeploymentValidationError(fmt.Errorf("unexpected validator failure: %s", secret))
	require.EqualError(t, err, "deployment structure check failed: internal")
	require.NotContains(t, err.Error(), secret)
}

func TestDeploymentValidationDiagnosticsDoNotLeakCommandErrors(t *testing.T) {
	const secret = "synthetic-compose-secret-that-must-not-leak"
	runner := &fakeRunner{rawComposeError: secret}
	service, policy := newUpdaterTestService(t, runner)

	_, err := service.Start(updatecontract.OperationPrepare, updatecontract.OperationRequest{
		Version: "0.1.184-rework.1", Actor: "admin:1",
	})
	require.NoError(t, err)
	status := waitForUpdater(t, service, 3*time.Second)
	require.Equal(t, updatecontract.UpdaterStateFailed, status.State)
	require.Equal(t, "deployment structure check failed: raw-compose-command", status.LastError)
	audit, err := os.ReadFile(policy.AuditPath)
	require.NoError(t, err)
	require.NotContains(t, status.LastError+string(audit), secret)
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

	require.ErrorIs(t, err, validationDockerSocket)
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

	require.ErrorIs(t, err, validationDockerSocket)
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

func TestMergedComposeValidationIgnoresUnrelatedEnvironmentTypes(t *testing.T) {
	runner := &fakeRunner{}
	service, policy := newUpdaterTestService(t, runner)
	var config renderedComposeConfig
	require.NoError(t, json.Unmarshal([]byte(runner.composeConfig), &config))
	application := config.Services[policy.ApplicationService]
	application.Environment = json.RawMessage(`{
  "SUB2API_UPDATER_SOCKET": "` + policy.SocketPath + `",
  "SUB2API_UPDATER_GID": "1234",
  "EMPTY": null,
  "NUMBER": 985,
  "BOOLEAN": false
}`)
	config.Services[policy.ApplicationService] = application
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
		"config --no-interpolate --format json", "config --format json", "config --services", "pg_isready",
		"env -u REDISCLI_AUTH redis-cli --raw ping",
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

func TestExamplePolicyKeepsTrustedStateInPrivateStorage(t *testing.T) {
	data, err := os.ReadFile("../../../deploy/updater/updater.example.json")
	require.NoError(t, err)
	var policy Policy
	require.NoError(t, json.Unmarshal(data, &policy))

	const stateDirectory = "/var/lib/sub2api-rework-updater"
	require.Equal(t, 2, policy.SchemaVersion)
	require.Equal(t, filepath.Join(stateDirectory, "state.json"), policy.StatePath)
	require.Equal(t, filepath.Join(stateDirectory, "audit.jsonl"), policy.AuditPath)
	require.Equal(t, filepath.Join(stateDirectory, "backups"), policy.BackupDirectory)
	require.Equal(t, "0.1.183-rework.3", policy.InitialInstalledVersion)
	require.Equal(t, 232, policy.InitialMigration)
}

func TestBaseSystemdUnitUsesPrivateStateAndRuntimeStorage(t *testing.T) {
	data, err := os.ReadFile("../../../deploy/updater/sub2api-rework-updater.service")
	require.NoError(t, err)
	unit := string(data)

	require.Contains(t, unit, "ProtectSystem=strict")
	require.Contains(t, unit, "StateDirectory=sub2api-rework-updater")
	require.Contains(t, unit, "StateDirectoryMode=0700")
	require.Contains(t, unit, "RuntimeDirectory=sub2api-rework-updater")
	require.Contains(t, unit, "RuntimeDirectoryMode=0750")
	require.Contains(t, unit, "ReadWritePaths=/var/lib/sub2api-rework-updater")
	require.Contains(t, unit, "ReadWritePaths=/run/sub2api-rework-updater")
	require.Contains(t, unit, "ReadWritePaths=/var/run/docker.sock")
	require.NotContains(t, unit, "LogsDirectory=")
	require.NotContains(t, unit, "/var/log/sub2api-rework-updater")
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
