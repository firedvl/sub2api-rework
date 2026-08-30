//go:build staging

package updater

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/updatecontract"
)

const (
	stagingSourceVersion = "0.1.183-rework.3"
	stagingTargetVersion = "0.1.183-rework.8"
	stagingFailedVersion = "0.1.183-rework.9"
	stagingUpdaterHome   = "/var/lib/sub2api-rework-updater"
	stagingUpdaterRun    = "/run/sub2api-rework-updater"
	stagingSourceImage   = "ghcr.io/firedvl/sub2api-rework:" + stagingSourceVersion
	stagingFixtureImage  = "ghcr.io/firedvl/sub2api-rework:0.1.183-rework.4"
	stagingTargetImage   = "ghcr.io/firedvl/sub2api-rework:" + stagingTargetVersion
	stagingFailedImage   = "ghcr.io/firedvl/sub2api-rework:" + stagingFailedVersion
	stagingTargetDigest  = "sha256:4874cbed4a6bd04edb307af49b13725c31f8da942d0c31b19d1e0d18e5c4dad5"
	stagingRedisSecret   = "synthetic-staging-redis-password"
	stagingWrongSecret   = "synthetic-staging-wrong-redis-password"
)

type stagingManifestFetcher struct{}

func (stagingManifestFetcher) Fetch(_ context.Context, version string) ([]byte, error) {
	manifest := updatecontract.Manifest{
		SchemaVersion: 1, ReworkVersion: version, UpstreamVersion: "v0.1.183",
		GitSHA: strings.Repeat(map[string]string{
			stagingTargetVersion: "a", stagingFailedVersion: "b",
		}[version], 40),
		Image: "ghcr.io/firedvl/sub2api-rework:" + version, ImageDigest: stagingTargetDigest,
		MigrationMin: 232, MigrationMax: 232, ReleaseDate: "2026-08-28T12:00:00Z",
		Compatibility: updatecontract.CompatibilityApproved, MinimumUpdaterVersion: Version,
	}
	if manifest.GitSHA == "" {
		return nil, fmt.Errorf("unknown staging release")
	}
	return json.Marshal(manifest)
}

type stagingRunner struct {
	ExecRunner
	mu           sync.Mutex
	failNextUp   bool
	healthFailed bool
}

func (runner *stagingRunner) Run(ctx context.Context, stdin io.Reader, stdout io.Writer, name string, args ...string) error {
	if len(args) == 2 && args[0] == "pull" && (args[1] == stagingTargetImage || args[1] == stagingFailedImage) {
		return nil
	}
	if err := runner.ExecRunner.Run(ctx, stdin, stdout, name, args...); err != nil {
		return err
	}
	if strings.Contains(strings.Join(args, " "), " up -d --no-deps sub2api") {
		runner.mu.Lock()
		if runner.failNextUp {
			runner.failNextUp = false
			runner.healthFailed = true
		} else if runner.healthFailed {
			runner.healthFailed = false
		}
		runner.mu.Unlock()
	}
	return nil
}

func (runner *stagingRunner) failNextApplicationHealth() {
	runner.mu.Lock()
	runner.failNextUp = true
	runner.mu.Unlock()
}

func (runner *stagingRunner) applicationHealthFailed() bool {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	return runner.healthFailed
}

type stagingHealthTransport struct {
	runner *stagingRunner
}

func (transport stagingHealthTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if transport.runner.applicationHealthFailed() {
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("staging health failure")),
			Request:    request,
		}, nil
	}
	return http.DefaultTransport.RoundTrip(request)
}

func TestProductionBootstrapPreservesUpdaterAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("staging test requires Docker")
	}
	if runtime.GOOS != "linux" {
		t.Skip("staging test requires Linux Unix-socket bind semantics")
	}
	if os.Getenv("INVOCATION_ID") == "" {
		t.Fatal("staging qualification must run under systemd")
	}
	docker, err := exec.LookPath("docker")
	if err != nil {
		t.Fatal("Docker is unavailable under the updater sandbox")
	}
	if home := os.Getenv("HOME"); home != stagingUpdaterHome {
		t.Fatalf("updater HOME is %q, want %q", home, stagingUpdaterHome)
	}
	if _, err := os.ReadDir("/root"); err == nil {
		t.Fatal("ProtectHome did not make /root inaccessible")
	}
	root, err := os.MkdirTemp(stagingUpdaterHome, "staging-")
	if err != nil {
		t.Fatalf("private updater HOME is not writable: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })

	deploymentDirectory := filepath.Join(root, "opt", "sub2api")
	socketDirectory := stagingUpdaterRun
	if err := os.MkdirAll(deploymentDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	assertStagingManagedPath(t, socketDirectory, 0750)
	unsafeAuditDirectory := filepath.Join(root, "var", "log", "sub2api-rework-updater")
	configureStagingVarLog(t, filepath.Dir(unsafeAuditDirectory), unsafeAuditDirectory)
	port := freeStagingPort(t)
	project := "sub2api-updater-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	baseCompose := filepath.Join(deploymentDirectory, "docker-compose.yml")
	updaterCompose := filepath.Join(deploymentDirectory, "docker-compose.updater.yml")
	environmentFile := filepath.Join(deploymentDirectory, ".env")
	writeStagingDeployment(t, project, port, socketDirectory, baseCompose, updaterCompose, environmentFile)

	compose := []string{
		"compose", "--project-directory", deploymentDirectory,
		"-f", baseCompose, "-f", updaterCompose, "--env-file", environmentFile,
	}
	if output, err := runStagingCommand(docker, "version"); err != nil {
		t.Fatalf("Docker failed under updater sandbox: %v: %s", err, output)
	}
	if output, err := runStagingCommand(docker, "compose", "version"); err != nil {
		t.Fatalf("Compose plugin discovery failed under updater sandbox: %v: %s", err, output)
	}
	pluginPath, err := runStagingCommand(docker, "info", "--format", `{{range .ClientInfo.Plugins}}{{if eq .Name "compose"}}{{.Path}}{{end}}{{end}}`)
	pluginPath = strings.TrimSpace(pluginPath)
	if err != nil || !filepath.IsAbs(pluginPath) || pathWithin(stagingUpdaterHome, pluginPath) || pathWithin("/root", pluginPath) {
		t.Fatalf("Compose plugin is not installed system-wide: %q: %v", pluginPath, err)
	}
	if output, err := runStagingCommand(pluginPath, "docker-cli-plugin-metadata"); err != nil || !json.Valid([]byte(output)) {
		t.Fatalf("direct Compose plugin metadata failed: %v: %s", err, output)
	}
	for _, config := range []struct {
		name string
		args []string
	}{
		{name: "raw", args: []string{"config", "--no-interpolate", "--format", "json"}},
		{name: "rendered", args: []string{"config", "--format", "json"}},
	} {
		output, err := runStagingCommand(docker, append(append([]string(nil), compose...), config.args...)...)
		if err != nil || !json.Valid([]byte(output)) {
			t.Fatalf("%s Compose config failed under updater sandbox: %v: %s", config.name, err, output)
		}
	}
	t.Cleanup(func() {
		_, _ = runStagingCommand(docker, append(compose, "down", "-v", "--remove-orphans")...)
		_, _ = runStagingCommand(docker, "image", "rm", stagingTargetImage)
		_, _ = runStagingCommand(docker, "image", "rm", stagingFailedImage)
	})
	// The unpublished target uses qualified fixture bytes under a local-only tag.
	if output, err := runStagingCommand(docker, "pull", stagingFixtureImage); err != nil {
		t.Fatalf("pull staging fixture image: %v: %s", err, output)
	}
	if output, err := runStagingCommand(docker, "tag", stagingFixtureImage, stagingTargetImage); err != nil {
		t.Fatalf("tag staging target image: %v: %s", err, output)
	}
	stateDirectory := filepath.Join(root, "var", "lib", "sub2api-rework-updater")
	policy := Policy{
		SchemaVersion:       2,
		SocketPath:          filepath.Join(socketDirectory, "updater.sock"),
		StatePath:           filepath.Join(stateDirectory, "state.json"),
		AuditPath:           filepath.Join(stateDirectory, "audit.jsonl"),
		LockPath:            filepath.Join(socketDirectory, "operation.lock"),
		BackupDirectory:     filepath.Join(stateDirectory, "backups"),
		DeploymentDirectory: deploymentDirectory,
		ComposeFiles:        []string{baseCompose, updaterCompose}, EnvironmentFile: environmentFile,
		DockerBinary: docker, ApplicationService: "sub2api", DatabaseService: "postgres", RedisService: "redis",
		DatabaseUser: "sub2api", DatabaseName: "sub2api",
		TrustedImageRepository: "ghcr.io/firedvl/sub2api-rework",
		ManifestBaseURL:        "https://github.com/firedvl/sub2api-rework/releases/download",
		HealthBaseURL:          "http://127.0.0.1:" + strconv.Itoa(port),
		MinimumFreeBytes:       1, OperationTimeoutSeconds: 300, HealthTimeoutSeconds: 5,
		InitialInstalledVersion: stagingSourceVersion, InitialMigration: 232,
	}
	runner := &stagingRunner{ExecRunner: ExecRunner{Directory: deploymentDirectory}}
	unsafePolicy := policy
	unsafePolicy.AuditPath = filepath.Join(unsafeAuditDirectory, "audit.jsonl")
	if _, err := NewService(unsafePolicy, runner, stagingManifestFetcher{}); err == nil || !strings.Contains(err.Error(), "unsafe parent") {
		t.Fatalf("unsafe /var/log-style audit path was not rejected: %v", err)
	}
	service, err := NewService(policy, runner, stagingManifestFetcher{})
	if err != nil {
		t.Fatal(err)
	}
	status, err := service.Status()
	if err != nil || status.UpdaterVersion != Version || status.InstalledVersion != stagingSourceVersion || status.CurrentMigration != 232 {
		t.Fatalf("unexpected staging bootstrap status: %+v, %v", status, err)
	}
	assertStagingManagedPath(t, stateDirectory, 0700)
	assertStagingManagedPath(t, policy.StatePath, 0600)
	assertStagingManagedPath(t, policy.BackupDirectory, 0700)
	service.healthHTTP.Transport = stagingHealthTransport{runner: runner}

	var stopServer context.CancelFunc
	var serverError chan error
	startUpdater := func() {
		serverContext, cancel := context.WithCancel(context.Background())
		stopServer = cancel
		serverError = make(chan error, 1)
		go func(handler http.Handler, result chan<- error) {
			result <- ServeUnix(serverContext, policy, handler)
		}(service.Handler(), serverError)
		waitForStagingSocket(t, policy.SocketPath)
	}
	stopUpdater := func() {
		if stopServer == nil {
			return
		}
		stopServer()
		select {
		case err := <-serverError:
			if err != nil {
				t.Errorf("stop updater socket: %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Error("updater socket did not stop")
		}
		stopServer = nil
		serverError = nil
	}
	startUpdater()
	t.Cleanup(stopUpdater)
	if output, err := runStagingCommand(docker, append(compose, "up", "-d", "--wait", "--wait-timeout", "180")...); err != nil {
		logs, _ := runStagingCommand(docker, append(compose, "logs", "--no-color", "--tail", "100")...)
		t.Fatalf("start staging deployment: %v: %s\n%s", err, output, logs)
	}

	assertStagingApplicationAccess(t, docker, compose, policy, strconv.Itoa(os.Getgid()))
	runtimeDirectoryBefore, err := os.Stat(socketDirectory)
	if err != nil {
		t.Fatal(err)
	}
	containerIDsBefore := stagingContainerIDs(t, docker, compose)
	stopUpdater()
	service, err = NewService(policy, runner, stagingManifestFetcher{})
	if err != nil {
		t.Fatal(err)
	}
	service.healthHTTP.Transport = stagingHealthTransport{runner: runner}
	startUpdater()
	runtimeDirectoryAfter, err := os.Stat(socketDirectory)
	if err != nil || !os.SameFile(runtimeDirectoryBefore, runtimeDirectoryAfter) {
		t.Fatalf("updater restart replaced its runtime directory: %v", err)
	}
	containerIDsAfter := stagingContainerIDs(t, docker, compose)
	for name, before := range containerIDsBefore {
		if after := containerIDsAfter[name]; after != before {
			t.Fatalf("updater restart recreated %s container: before=%s after=%s", name, before, after)
		}
	}
	status, err = service.Status()
	if err != nil || status.CurrentMigration != 232 {
		t.Fatalf("updater restart changed migration 232: %+v, %v", status, err)
	}
	assertStagingApplicationAccess(t, docker, compose, policy, strconv.Itoa(os.Getgid()))
	assertStagingRedisAuthModes(t, docker, compose, service, policy)
	assertStagingSafeDeploymentReason(t, docker, compose, service, policy, updaterCompose)
	assertStagingManagedPath(t, policy.AuditPath, 0600)
	requestStagingOperation(t, docker, compose, policy.SocketPath, updatecontract.OperationPrepare, stagingTargetVersion)
	waitForStagingOperation(t, service, updatecontract.UpdaterStatePrepared, 3*time.Minute)
	requestStagingOperation(t, docker, compose, policy.SocketPath, updatecontract.OperationInstall, stagingTargetVersion)
	waitForStagingOperation(t, service, updatecontract.UpdaterStateSucceeded, 5*time.Minute)
	assertStagingApplicationAccess(t, docker, compose, policy, strconv.Itoa(os.Getgid()))

	if output, err := runStagingCommand(docker, "tag", stagingTargetImage, stagingFailedImage); err != nil {
		t.Fatalf("tag staging failure image: %v: %s", err, output)
	}
	requestStagingOperation(t, docker, compose, policy.SocketPath, updatecontract.OperationPrepare, stagingFailedVersion)
	waitForStagingOperation(t, service, updatecontract.UpdaterStatePrepared, 3*time.Minute)
	runner.failNextApplicationHealth()
	requestStagingOperation(t, docker, compose, policy.SocketPath, updatecontract.OperationInstall, stagingFailedVersion)
	status = waitForStagingOperation(t, service, updatecontract.UpdaterStateFailed, 5*time.Minute)
	if status.InstalledVersion != stagingTargetVersion || status.LastAttempt == nil || status.LastAttempt.RollbackResult != "succeeded" {
		t.Fatalf("automatic rollback did not restore %s: %+v", stagingTargetVersion, status)
	}
	assertStagingApplicationAccess(t, docker, compose, policy, strconv.Itoa(os.Getgid()))
	if dropIn, err := SystemdDropIn(policy); err != nil || !strings.Contains(string(dropIn), deploymentDirectory) || strings.Contains(string(dropIn), "/opt/sub2api-rework/deploy") {
		t.Fatalf("deployment-specific systemd drop-in is invalid: %q, %v", dropIn, err)
	}
}

func stagingContainerIDs(t *testing.T, docker string, compose []string) map[string]string {
	t.Helper()
	ids := make(map[string]string, 3)
	for _, name := range []string{"sub2api", "postgres", "redis"} {
		output, err := runStagingCommand(docker, append(append([]string(nil), compose...), "ps", "-q", name)...)
		if err != nil || strings.TrimSpace(output) == "" {
			t.Fatalf("find staging %s container: %v: %s", name, err, output)
		}
		ids[name] = strings.TrimSpace(output)
	}
	return ids
}

func configureStagingVarLog(t *testing.T, parent, child string) {
	t.Helper()
	if err := os.MkdirAll(child, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0775); err != nil {
		t.Fatal(err)
	}
	if os.Geteuid() == 0 && os.Getgid() == 0 {
		if err := os.Chown(parent, 0, 123); err != nil {
			t.Fatal(err)
		}
	}
	info, err := os.Stat(parent)
	if err != nil {
		t.Fatal(err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || info.Mode().Perm() != 0775 || stat.Gid == 0 || (os.Geteuid() == 0 && stat.Uid != 0) {
		t.Fatalf("staging /var/log model is invalid: mode=%o stat=%+v", info.Mode().Perm(), stat)
	}
}

func assertStagingManagedPath(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != mode || stat.Uid != uint32(os.Geteuid()) {
		t.Fatalf("invalid managed path %s: mode=%v stat=%+v", path, info.Mode(), stat)
	}
}

func writeStagingDeployment(t *testing.T, project string, port int, socketDirectory, basePath, overridePath, environmentPath string) {
	t.Helper()
	sourcePath := os.Getenv("SUB2API_STAGING_COMPOSE")
	if sourcePath == "" {
		sourcePath = "../../../deploy/docker-compose.yml"
	}
	base, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	base = bytes.ReplaceAll(base, []byte("container_name: sub2api-postgres"), []byte("container_name: "+project+"-postgres"))
	base = bytes.ReplaceAll(base, []byte("container_name: sub2api-redis"), []byte("container_name: "+project+"-redis"))
	base = bytes.ReplaceAll(base, []byte("container_name: sub2api\n"), []byte("container_name: "+project+"-application\n"))
	for source, replacement := range map[string]string{
		"- sub2api_data:/app/data":                 "- " + filepath.Join(filepath.Dir(basePath), "data") + ":/app/data:Z",
		"- postgres_data:/var/lib/postgresql/data": "- " + filepath.Join(filepath.Dir(basePath), "postgres") + ":/var/lib/postgresql/data",
		"- redis_data:/data":                       "- " + filepath.Join(filepath.Dir(basePath), "redis") + ":/data",
	} {
		if bytes.Count(base, []byte(source)) != 1 {
			t.Fatalf("staging source mount %q changed", source)
		}
		base = bytes.Replace(base, []byte(source), []byte(replacement), 1)
	}
	volumeStart := bytes.Index(base, []byte("\n# =============================================================================\n# Volumes\n"))
	networkStart := bytes.Index(base, []byte("\n# =============================================================================\n# Networks\n"))
	if volumeStart < 0 || networkStart <= volumeStart {
		t.Fatal("staging top-level volume block changed")
	}
	base = append(base[:volumeStart], base[networkStart:]...)
	for _, directory := range []string{"data", "postgres", "redis"} {
		path := filepath.Join(filepath.Dir(basePath), directory)
		if err := os.Mkdir(path, 0777); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0777); err != nil {
			t.Fatal(err)
		}
	}
	override := fmt.Sprintf(`services:
  sub2api:
    group_add:
      - "%d"
    volumes:
      - type: bind
        source: %s
        target: %s
    environment:
      - SUB2API_UPDATER_SOCKET=%s/updater.sock
      - SUB2API_UPDATER_GID=%d

  redis:
    command: ["redis-server", "--save", "60", "1", "--appendonly", "yes", "--appendfsync", "everysec"]
    environment:
      - REDISCLI_AUTH=${STAGING_REDIS_CLIENT_PASSWORD:?STAGING_REDIS_CLIENT_PASSWORD is required}
    healthcheck:
      test: ["CMD-SHELL", "test \"$$(env -u REDISCLI_AUTH redis-cli --raw ping 2>/dev/null)\" = PONG || test \"$$(redis-cli --raw ping 2>/dev/null)\" = PONG"]
      interval: 1s
      timeout: 1s
      retries: 2
      start_period: 1s
`, os.Getgid(), socketDirectory, socketDirectory, socketDirectory, os.Getgid())
	environment := fmt.Sprintf(`BIND_HOST=127.0.0.1
SERVER_PORT=%d
SUB2API_IMAGE=%s
STAGING_REDIS_CLIENT_PASSWORD=%s
POSTGRES_USER=sub2api
POSTGRES_PASSWORD=synthetic-staging-password
POSTGRES_DB=sub2api
ADMIN_EMAIL=admin@sub2api.local
ADMIN_PASSWORD=synthetic-staging-admin-password
JWT_SECRET=synthetic-staging-jwt-secret-with-at-least-32-bytes
TOTP_ENCRYPTION_KEY=0000000000000000000000000000000000000000000000000000000000000000
RUN_MODE=simple
`, port, stagingSourceImage, stagingRedisSecret)
	for path, data := range map[string][]byte{
		basePath: base, overridePath: []byte(override), environmentPath: []byte(environment),
	} {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func assertStagingRedisAuthModes(t *testing.T, docker string, compose []string, service *Service, policy Policy) {
	t.Helper()
	ordinary := append(append([]string(nil), compose...), "exec", "-T", "redis", "redis-cli", "--raw", "ping")
	output, err := runStagingCommandCombined(docker, ordinary...)
	if err != nil || !strings.Contains(output, "AUTH failed") || !strings.Contains(output, "PONG") {
		t.Fatalf("ordinary redis-cli did not reproduce its zero-exit stale-auth diagnostic: %v: %s", err, output)
	}
	if err := service.checkRedis(context.Background()); err != nil {
		t.Fatalf("updater rejected no-auth Redis with stale client auth: %v", err)
	}

	setStagingRedisPassword(t, docker, compose, stagingRedisSecret, true)
	waitForStagingRedisHealth(t, docker, compose, "healthy")
	if err := service.checkRedis(context.Background()); err != nil {
		t.Fatalf("updater rejected authenticated Redis with correct credentials: %v", err)
	}

	setStagingRedisPassword(t, docker, compose, stagingWrongSecret, false)
	waitForStagingRedisHealth(t, docker, compose, "unhealthy")
	output, err = runStagingCommandCombined(docker, ordinary...)
	if err != nil || !strings.Contains(output, "AUTH failed") || !strings.Contains(output, "NOAUTH") {
		t.Fatalf("ordinary redis-cli did not reproduce its zero-exit wrong-auth response: %v: %s", err, output)
	}
	if err := service.checkRedis(context.Background()); err == nil {
		t.Fatal("updater accepted Redis with wrong credentials")
	} else if strings.Contains(err.Error(), stagingRedisSecret) || strings.Contains(err.Error(), stagingWrongSecret) {
		t.Fatalf("Redis health error exposed a secret: %v", err)
	}
	requestStagingOperation(t, docker, compose, policy.SocketPath, updatecontract.OperationPrepare, stagingTargetVersion)
	status := waitForStagingOperation(t, service, updatecontract.UpdaterStateFailed, time.Minute)
	audit, err := os.ReadFile(policy.AuditPath)
	if err != nil {
		t.Fatal(err)
	}
	visible := status.LastError + "\n" + string(audit)
	if strings.Contains(visible, stagingRedisSecret) || strings.Contains(visible, stagingWrongSecret) {
		t.Fatal("Redis credentials appeared in updater status or audit")
	}

	if output, err := runStagingCommand(docker, append(append([]string(nil), compose...), "stop", "redis")...); err != nil {
		t.Fatalf("stop staging Redis: %v: %s", err, output)
	}
	if err := service.checkRedis(context.Background()); err == nil {
		t.Fatal("updater accepted stopped Redis")
	}
	if output, err := runStagingCommand(docker, append(append([]string(nil), compose...),
		"up", "-d", "--force-recreate", "--wait", "--wait-timeout", "60", "redis")...); err != nil {
		t.Fatalf("recreate staging Redis: %v: %s", err, output)
	}
	if err := service.checkRedis(context.Background()); err != nil {
		t.Fatalf("updater did not recover after Redis recreate: %v", err)
	}
}

func assertStagingSafeDeploymentReason(t *testing.T, docker string, compose []string, service *Service, policy Policy, overridePath string) {
	t.Helper()
	original, err := os.ReadFile(overridePath)
	if err != nil {
		t.Fatal(err)
	}
	socketDirectory := filepath.Dir(policy.SocketPath)
	broken := bytes.Replace(original,
		[]byte("target: "+socketDirectory), []byte("target: /var/run/docker.sock"), 1)
	if bytes.Equal(broken, original) {
		t.Fatal("staging updater socket target was not found")
	}
	if err := os.WriteFile(overridePath, broken, 0600); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.WriteFile(overridePath, original, 0600); err != nil {
			t.Errorf("restore staging updater override: %v", err)
		}
	}()

	requestStagingOperation(t, docker, compose, policy.SocketPath, updatecontract.OperationPrepare, stagingTargetVersion)
	status := waitForStagingOperation(t, service, updatecontract.UpdaterStateFailed, time.Minute)
	if status.LastError != "deployment structure check failed: docker-socket" {
		t.Fatalf("unsafe staging topology returned %q", status.LastError)
	}
	audit, err := os.ReadFile(policy.AuditPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(status.LastError+string(audit), stagingRedisSecret) {
		t.Fatal("Compose credential appeared in updater status or audit")
	}
}

func setStagingRedisPassword(t *testing.T, docker string, compose []string, password string, withoutClientAuth bool) {
	t.Helper()
	args := append(append([]string(nil), compose...), "exec", "-T", "redis")
	if withoutClientAuth {
		args = append(args, "env", "-u", "REDISCLI_AUTH")
	}
	args = append(args, "redis-cli", "--raw", "-x", "CONFIG", "SET", "requirepass")
	output, err := runStagingCommandInput(password, docker, args...)
	if err != nil || strings.TrimSpace(output) != "OK" {
		t.Fatalf("set staging Redis password: %v: %s", err, output)
	}
}

func waitForStagingRedisHealth(t *testing.T, docker string, compose []string, wanted string) {
	t.Helper()
	containerID, err := runStagingCommand(docker, append(append([]string(nil), compose...), "ps", "-q", "redis")...)
	if err != nil || strings.TrimSpace(containerID) == "" {
		t.Fatalf("find staging Redis container: %v: %s", err, containerID)
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		status, _ := runStagingCommand(docker, "inspect", "--format", "{{.State.Health.Status}}", strings.TrimSpace(containerID))
		if strings.TrimSpace(status) == wanted {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("staging Redis did not become %s", wanted)
}

func requestStagingOperation(t *testing.T, docker string, compose []string, socketPath string, operation updatecontract.Operation, version string) {
	t.Helper()
	request := updatecontract.OperationRequest{Version: version, Actor: "admin:1"}
	if operation == updatecontract.OperationInstall {
		request.Confirmation = "INSTALL " + version
	}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	args := append(append([]string(nil), compose...),
		"exec", "-T", "-u", "root", "sub2api", "su-exec", "sub2api",
		"curl", "--silent", "--show-error", "--fail", "--unix-socket", socketPath,
		"--header", "Content-Type: application/json", "--data", string(data), "http://updater/v1/"+string(operation),
	)
	if output, err := runStagingCommand(docker, args...); err != nil {
		t.Fatalf("request staging %s: %v: %s", operation, err, output)
	}
}

func assertStagingApplicationAccess(t *testing.T, docker string, compose []string, policy Policy, updaterGID string) {
	t.Helper()
	statusArgs := append(append([]string(nil), compose...),
		"exec", "-T", "-u", "root", "sub2api", "su-exec", "sub2api",
		"curl", "--silent", "--show-error", "--fail", "--unix-socket", policy.SocketPath,
		"http://updater/v1/status",
	)
	output, err := runStagingCommand(docker, statusArgs...)
	if err != nil || !strings.Contains(output, `"updater_version":"`+Version+`"`) {
		t.Fatalf("application cannot reach updater status: %v: %s", err, output)
	}
	containerArgs := append(append([]string(nil), compose...), "ps", "-q", "sub2api")
	containerID, err := runStagingCommand(docker, containerArgs...)
	if err != nil || strings.TrimSpace(containerID) == "" {
		t.Fatalf("find staging application container: %v: %s", err, containerID)
	}
	inspect, err := runStagingCommand(docker, "inspect", strings.TrimSpace(containerID))
	if err != nil {
		t.Fatal(err)
	}
	var containers []struct {
		HostConfig struct {
			GroupAdd []string
		}
		Mounts []struct {
			Type, Source, Destination string
		}
	}
	if err := json.Unmarshal([]byte(inspect), &containers); err != nil || len(containers) != 1 {
		t.Fatalf("decode staging container inspection: %v", err)
	}
	if !containsString(containers[0].HostConfig.GroupAdd, updaterGID) {
		t.Fatalf("application is missing updater GID %s: %v", updaterGID, containers[0].HostConfig.GroupAdd)
	}
	hasUpdaterSocket := false
	for _, mount := range containers[0].Mounts {
		if filepath.Base(filepath.Clean(mount.Source)) == "docker.sock" || filepath.Base(filepath.Clean(mount.Destination)) == "docker.sock" {
			t.Fatalf("application has Docker socket mount: %+v", mount)
		}
		if mount.Type == "bind" && mount.Destination == filepath.Dir(policy.SocketPath) {
			hasUpdaterSocket = true
		}
	}
	if !hasUpdaterSocket {
		t.Fatal("application is missing updater socket bind mount")
	}
}

func waitForStagingOperation(t *testing.T, service *Service, state updatecontract.UpdaterState, timeout time.Duration) updatecontract.UpdaterStatus {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status, err := service.Status()
		if err != nil {
			t.Fatal(err)
		}
		if !status.Busy {
			if status.State != state {
				t.Fatalf("updater reached %s instead of %s: %s", status.State, state, status.LastError)
			}
			return status
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("staging updater operation timed out")
	return updatecontract.UpdaterStatus{}
}

func waitForStagingSocket(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(path); err == nil && info.Mode()&os.ModeSocket != 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("updater socket did not start")
}

func freeStagingPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	return listener.Addr().(*net.TCPAddr).Port
}

func TestStagingCommandOutputModes(t *testing.T) {
	output, err := runStagingCommand("sh", "-c", "printf container-id; printf warning >&2")
	if err != nil || output != "container-id" {
		t.Fatalf("successful stderr polluted stdout: %v: %q", err, output)
	}
	output, err = runStagingCommandCombined("sh", "-c", "printf PONG; printf 'AUTH failed' >&2")
	if err != nil || output != "PONGAUTH failed" {
		t.Fatalf("combined diagnostic output was lost: %v: %q", err, output)
	}
}

func runStagingCommand(name string, args ...string) (string, error) {
	return runStagingCommandInput("", name, args...)
}

func runStagingCommandCombined(name string, args ...string) (string, error) {
	command := exec.Command(name, args...)
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	err := command.Run()
	return output.String(), err
}

func runStagingCommandInput(input, name string, args ...string) (string, error) {
	command := exec.Command(name, args...)
	command.Stdin = strings.NewReader(input)
	output, err := command.Output()
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		output = append(output, exitError.Stderr...)
	}
	return string(output), err
}
