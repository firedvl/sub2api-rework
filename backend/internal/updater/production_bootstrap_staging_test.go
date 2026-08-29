//go:build staging

package updater

import (
	"bytes"
	"context"
	"encoding/json"
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
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/updatecontract"
)

const (
	stagingSourceVersion = "0.1.183-rework.1"
	stagingTargetVersion = "0.1.183-rework.2"
	stagingFailedVersion = "0.1.183-rework.3"
	stagingSourceImage   = "ghcr.io/firedvl/sub2api-rework:" + stagingSourceVersion
	stagingTargetImage   = "ghcr.io/firedvl/sub2api-rework:" + stagingTargetVersion
	stagingFailedImage   = "ghcr.io/firedvl/sub2api-rework:" + stagingFailedVersion
	stagingTargetDigest  = "sha256:978365d39e93ce9dcad460496664e145b4ee98de3041a718f0702e716ff6332f"
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
		Compatibility: updatecontract.CompatibilityApproved, MinimumUpdaterVersion: "1.1.0",
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
	if len(args) == 2 && args[0] == "pull" && args[1] == stagingFailedImage {
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
	docker, err := exec.LookPath("docker")
	if err != nil {
		t.Skip("docker is unavailable")
	}
	root, err := os.MkdirTemp("/tmp", "sub2api-updater-staging-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })

	deploymentDirectory := filepath.Join(root, "opt", "sub2api")
	socketDirectory := filepath.Join(root, "run", "sub2api-rework-updater")
	if err := os.MkdirAll(deploymentDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(socketDirectory, 0750); err != nil {
		t.Fatal(err)
	}
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
	t.Cleanup(func() {
		_, _ = runStagingCommand(docker, append(compose, "down", "-v", "--remove-orphans")...)
		_, _ = runStagingCommand(docker, "image", "rm", stagingFailedImage)
	})
	policy := Policy{
		SchemaVersion:       2,
		SocketPath:          filepath.Join(socketDirectory, "updater.sock"),
		StatePath:           filepath.Join(root, "var", "lib", "sub2api-rework-updater", "state.json"),
		AuditPath:           filepath.Join(root, "var", "log", "sub2api-rework-updater", "audit.jsonl"),
		LockPath:            filepath.Join(root, "run", "sub2api-rework-updater", "operation.lock"),
		BackupDirectory:     filepath.Join(root, "var", "lib", "sub2api-rework-updater", "backups"),
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
	service, err := NewService(policy, runner, stagingManifestFetcher{})
	if err != nil {
		t.Fatal(err)
	}
	service.healthHTTP.Transport = stagingHealthTransport{runner: runner}

	serverContext, stopServer := context.WithCancel(context.Background())
	serverError := make(chan error, 1)
	go func() { serverError <- ServeUnix(serverContext, policy, service.Handler()) }()
	t.Cleanup(func() {
		stopServer()
		select {
		case err := <-serverError:
			if err != nil {
				t.Errorf("stop updater socket: %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Error("updater socket did not stop")
		}
	})
	waitForStagingSocket(t, policy.SocketPath)
	if output, err := runStagingCommand(docker, append(compose, "up", "-d", "--wait", "--wait-timeout", "180")...); err != nil {
		logs, _ := runStagingCommand(docker, append(compose, "logs", "--no-color", "--tail", "100")...)
		t.Fatalf("start staging deployment: %v: %s\n%s", err, output, logs)
	}

	assertStagingApplicationAccess(t, docker, compose, policy, strconv.Itoa(os.Getgid()))
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
	status := waitForStagingOperation(t, service, updatecontract.UpdaterStateFailed, 5*time.Minute)
	if status.InstalledVersion != stagingTargetVersion || status.LastAttempt == nil || status.LastAttempt.RollbackResult != "succeeded" {
		t.Fatalf("automatic rollback did not restore %s: %+v", stagingTargetVersion, status)
	}
	assertStagingApplicationAccess(t, docker, compose, policy, strconv.Itoa(os.Getgid()))
	if dropIn, err := SystemdDropIn(policy); err != nil || !strings.Contains(string(dropIn), deploymentDirectory) || strings.Contains(string(dropIn), "/opt/sub2api-rework/deploy") {
		t.Fatalf("deployment-specific systemd drop-in is invalid: %q, %v", dropIn, err)
	}
}

func writeStagingDeployment(t *testing.T, project string, port int, socketDirectory, basePath, overridePath, environmentPath string) {
	t.Helper()
	base, err := os.ReadFile("../../../deploy/docker-compose.yml")
	if err != nil {
		t.Fatal(err)
	}
	base = bytes.ReplaceAll(base, []byte("container_name: sub2api-postgres"), []byte("container_name: "+project+"-postgres"))
	base = bytes.ReplaceAll(base, []byte("container_name: sub2api-redis"), []byte("container_name: "+project+"-redis"))
	base = bytes.ReplaceAll(base, []byte("container_name: sub2api\n"), []byte("container_name: "+project+"-application\n"))
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
`, os.Getgid(), socketDirectory, socketDirectory, socketDirectory, os.Getgid())
	environment := fmt.Sprintf(`BIND_HOST=127.0.0.1
SERVER_PORT=%d
SUB2API_IMAGE=%s
POSTGRES_USER=sub2api
POSTGRES_PASSWORD=synthetic-staging-password
POSTGRES_DB=sub2api
ADMIN_EMAIL=admin@sub2api.local
ADMIN_PASSWORD=synthetic-staging-admin-password
JWT_SECRET=synthetic-staging-jwt-secret-with-at-least-32-bytes
TOTP_ENCRYPTION_KEY=0000000000000000000000000000000000000000000000000000000000000000
RUN_MODE=simple
`, port, stagingSourceImage)
	for path, data := range map[string][]byte{
		basePath: base, overridePath: []byte(override), environmentPath: []byte(environment),
	} {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
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
	if err != nil || !strings.Contains(output, `"updater_version":"1.1.0"`) {
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

func runStagingCommand(name string, args ...string) (string, error) {
	command := exec.Command(name, args...)
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	err := command.Run()
	return output.String(), err
}
