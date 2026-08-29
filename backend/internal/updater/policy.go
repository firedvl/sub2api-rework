package updater

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/Wei-Shaw/sub2api/internal/releaseinfo"
	"github.com/Wei-Shaw/sub2api/internal/updatecontract"
)

const maxPolicyBytes = 64 * 1024

var policyNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

var systemdProtectedPaths = []string{
	"/usr/local/sbin/sub2api-rework-updater",
	"/etc/sub2api-rework/updater.json",
	"/etc/systemd/system/sub2api-rework-updater.service",
	"/etc/systemd/system/sub2api-rework-updater.service.d",
}

type Policy struct {
	SchemaVersion           int      `json:"schema_version"`
	SocketPath              string   `json:"socket_path"`
	StatePath               string   `json:"state_path"`
	AuditPath               string   `json:"audit_path"`
	LockPath                string   `json:"lock_path"`
	BackupDirectory         string   `json:"backup_directory"`
	DeploymentDirectory     string   `json:"deployment_directory"`
	ComposeFiles            []string `json:"compose_files"`
	EnvironmentFile         string   `json:"environment_file"`
	DockerBinary            string   `json:"docker_binary"`
	ApplicationService      string   `json:"application_service"`
	DatabaseService         string   `json:"database_service"`
	RedisService            string   `json:"redis_service"`
	DatabaseUser            string   `json:"database_user"`
	DatabaseName            string   `json:"database_name"`
	TrustedImageRepository  string   `json:"trusted_image_repository"`
	ManifestBaseURL         string   `json:"manifest_base_url"`
	HealthBaseURL           string   `json:"health_base_url"`
	MinimumFreeBytes        uint64   `json:"minimum_free_bytes"`
	OperationTimeoutSeconds int      `json:"operation_timeout_seconds"`
	HealthTimeoutSeconds    int      `json:"health_timeout_seconds"`
	InitialInstalledVersion string   `json:"initial_installed_version"`
	InitialMigration        int      `json:"initial_migration"`
}

func LoadPolicy(path string) (Policy, error) {
	if !cleanAbsolutePath(path) {
		return Policy{}, fmt.Errorf("updater policy path must be clean and absolute")
	}
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return Policy{}, fmt.Errorf("open updater policy: %w", err)
	}
	file := os.NewFile(uintptr(fd), path)
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return Policy{}, fmt.Errorf("stat updater policy: %w", err)
	}
	if !info.Mode().IsRegular() {
		return Policy{}, fmt.Errorf("updater policy must be a regular file")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return Policy{}, fmt.Errorf("updater policy ownership is unavailable")
	}
	if err := validatePolicyFileSecurity(info.Mode(), stat.Uid); err != nil {
		return Policy{}, err
	}
	if info.Size() <= 0 || info.Size() > maxPolicyBytes {
		return Policy{}, fmt.Errorf("updater policy size is invalid")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxPolicyBytes+1))
	if err != nil {
		return Policy{}, fmt.Errorf("read updater policy: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var policy Policy
	if err := decoder.Decode(&policy); err != nil {
		return Policy{}, fmt.Errorf("decode updater policy: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Policy{}, fmt.Errorf("updater policy contains trailing data")
	}
	if err := policy.Validate(); err != nil {
		return Policy{}, err
	}
	return policy, nil
}

func validatePolicyFileSecurity(mode os.FileMode, ownerUID uint32) error {
	if ownerUID != 0 {
		return fmt.Errorf("updater policy must be owned by root")
	}
	if mode.Perm()&022 != 0 {
		return fmt.Errorf("updater policy must not be group or world writable")
	}
	return nil
}

func (p Policy) Validate() error {
	if p.SchemaVersion != 2 {
		return fmt.Errorf("unsupported updater policy schema_version %d", p.SchemaVersion)
	}
	for name, path := range map[string]string{
		"socket_path": p.SocketPath, "state_path": p.StatePath, "audit_path": p.AuditPath,
		"lock_path": p.LockPath, "backup_directory": p.BackupDirectory,
		"deployment_directory": p.DeploymentDirectory,
		"environment_file":     p.EnvironmentFile, "docker_binary": p.DockerBinary,
	} {
		if !cleanAbsolutePath(path) {
			return fmt.Errorf("%s must be a clean absolute path", name)
		}
	}
	if parent := filepath.Dir(p.DeploymentDirectory); parent == p.DeploymentDirectory || parent == string(filepath.Separator) {
		return fmt.Errorf("deployment_directory must identify a dedicated subdirectory")
	}
	if !pathWithin(p.DeploymentDirectory, p.EnvironmentFile) {
		return fmt.Errorf("environment_file must be inside deployment_directory")
	}
	if len(p.ComposeFiles) == 0 {
		return fmt.Errorf("compose_files must not be empty")
	}
	seenComposeFiles := make(map[string]struct{}, len(p.ComposeFiles))
	for _, path := range p.ComposeFiles {
		if !cleanAbsolutePath(path) {
			return fmt.Errorf("compose_files entries must be clean absolute paths")
		}
		if !pathWithin(p.DeploymentDirectory, path) {
			return fmt.Errorf("compose_files entries must be inside deployment_directory")
		}
		if _, duplicate := seenComposeFiles[path]; duplicate {
			return fmt.Errorf("compose_files must not contain duplicates")
		}
		seenComposeFiles[path] = struct{}{}
	}
	if filepath.Base(p.DockerBinary) != "docker" {
		return fmt.Errorf("docker_binary must name the Docker CLI")
	}
	for name, value := range map[string]string{
		"application_service": p.ApplicationService, "database_service": p.DatabaseService,
		"redis_service": p.RedisService, "database_user": p.DatabaseUser, "database_name": p.DatabaseName,
	} {
		if !policyNamePattern.MatchString(value) {
			return fmt.Errorf("invalid %s", name)
		}
	}
	metadata := releaseinfo.Current()
	if p.TrustedImageRepository != metadata.ArtifactRepository {
		return fmt.Errorf("trusted_image_repository must match embedded release metadata")
	}
	wantManifestBase := "https://github.com/" + metadata.ReworkRepository + "/releases/download"
	if strings.TrimSuffix(p.ManifestBaseURL, "/") != wantManifestBase {
		return fmt.Errorf("manifest_base_url must match the canonical rework repository")
	}
	base, err := url.Parse(p.HealthBaseURL)
	if err != nil || base.Scheme != "http" || base.User != nil || base.RawQuery != "" || base.Fragment != "" ||
		(base.Hostname() != "127.0.0.1" && base.Hostname() != "localhost" && base.Hostname() != "::1") {
		return fmt.Errorf("health_base_url must be a loopback HTTP URL")
	}
	if p.MinimumFreeBytes == 0 || p.OperationTimeoutSeconds < 30 || p.OperationTimeoutSeconds > 3600 ||
		p.HealthTimeoutSeconds < 5 || p.HealthTimeoutSeconds > 300 {
		return fmt.Errorf("invalid updater thresholds or timeouts")
	}
	if !updatecontract.IsReworkVersion(p.InitialInstalledVersion) || p.InitialMigration < 0 {
		return fmt.Errorf("invalid initial installation identity")
	}
	return nil
}

func cleanAbsolutePath(path string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == path && strings.IndexFunc(path, unicode.IsControl) == -1
}

func pathWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func (p Policy) operationTimeout() time.Duration {
	return time.Duration(p.OperationTimeoutSeconds) * time.Second
}

func (p Policy) healthTimeout() time.Duration {
	return time.Duration(p.HealthTimeoutSeconds) * time.Second
}

func SystemdDropIn(policy Policy, runtimeProtectedPaths ...string) ([]byte, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	deploymentPaths := []string{policy.DeploymentDirectory}
	if resolved, err := filepath.EvalSymlinks(policy.DeploymentDirectory); err == nil {
		deploymentPaths = append(deploymentPaths, resolved)
	}
	protectedPaths := append([]string{policy.DockerBinary}, systemdProtectedPaths...)
	protectedPaths = append(protectedPaths, runtimeProtectedPaths...)
	for _, protectedPath := range protectedPaths {
		if !cleanAbsolutePath(protectedPath) {
			return nil, fmt.Errorf("systemd protected path must be clean and absolute")
		}
		pathAliases := []string{protectedPath}
		if resolved, err := filepath.EvalSymlinks(protectedPath); err == nil {
			pathAliases = append(pathAliases, resolved)
		}
		for _, deploymentPath := range deploymentPaths {
			for _, path := range pathAliases {
				if pathWithin(deploymentPath, path) || pathWithin(path, deploymentPath) {
					return nil, fmt.Errorf("deployment_directory must not contain updater installation paths")
				}
			}
		}
	}
	path := strconv.Quote(strings.ReplaceAll(policy.DeploymentDirectory, "%", "%%"))
	return []byte("[Service]\nReadWritePaths=" + path + "\n"), nil
}
