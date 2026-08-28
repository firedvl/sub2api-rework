package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/updatecontract"
	"gopkg.in/yaml.v3"
)

const (
	defaultDeploymentImage = "weishaw/sub2api:latest"
	migrationQuery         = "SELECT COALESCE(MAX((regexp_match(filename, '^([0-9]+)'))[1]::int), 0) FROM schema_migrations"
)

func (s *Service) preflight(ctx context.Context, manifest *updatecontract.Manifest, requireApplicationHealth bool) (int, error) {
	if err := s.validateDeploymentFiles(); err != nil {
		return 0, fmt.Errorf("deployment structure check failed")
	}
	if err := s.checkDiskSpace(); err != nil {
		return 0, err
	}
	if err := s.checkBackupWritable(); err != nil {
		return 0, fmt.Errorf("backup destination is not writable")
	}
	if err := s.runDocker(ctx, nil, io.Discard, "version", "--format", "{{.Server.Version}}"); err != nil {
		return 0, fmt.Errorf("docker is unavailable")
	}
	services, err := s.commandOutput(ctx, s.composeArgs("config", "--services")...)
	if err != nil || !containsLine(services, s.policy.ApplicationService) ||
		!containsLine(services, s.policy.DatabaseService) || !containsLine(services, s.policy.RedisService) {
		return 0, fmt.Errorf("compose deployment services are unavailable")
	}
	if requireApplicationHealth {
		if err := s.checkHTTP(ctx, "/health"); err != nil {
			return 0, fmt.Errorf("current application is not healthy enough to update")
		}
	}
	if err := s.checkDatabase(ctx); err != nil {
		return 0, fmt.Errorf("database is unavailable")
	}
	if err := s.checkRedis(ctx); err != nil {
		return 0, fmt.Errorf("redis is unavailable")
	}
	migration, err := s.currentMigration(ctx)
	if err != nil {
		return 0, err
	}
	if manifest != nil && (migration < manifest.MigrationMin || migration > manifest.MigrationMax) {
		return 0, fmt.Errorf("migration compatibility preflight failed")
	}
	return migration, nil
}

func (s *Service) preflightRollback(ctx context.Context) (int, error) {
	return s.preflight(ctx, nil, false)
}

func (s *Service) validateDeploymentFiles() error {
	data, err := readManagedFile(s.policy.ComposeFile, 2*1024*1024, false)
	if err != nil {
		return fmt.Errorf("invalid deployment file")
	}
	if _, err := validateManagedRegularFile(s.policy.EnvironmentFile, 2*1024*1024, true); err != nil {
		return fmt.Errorf("invalid deployment file")
	}
	var compose struct {
		Services map[string]struct {
			Image string `yaml:"image"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return err
	}
	application, ok := compose.Services[s.policy.ApplicationService]
	if !ok || !strings.Contains(application.Image, "${SUB2API_IMAGE") {
		return fmt.Errorf("application image must use SUB2API_IMAGE")
	}
	if _, ok := compose.Services[s.policy.DatabaseService]; !ok {
		return fmt.Errorf("database service missing")
	}
	if _, ok := compose.Services[s.policy.RedisService]; !ok {
		return fmt.Errorf("redis service missing")
	}
	return nil
}

func (s *Service) checkDiskSpace() error {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(s.policy.DeploymentDirectory, &stat); err != nil {
		return fmt.Errorf("read free disk space")
	}
	available := uint64(stat.Bavail) * uint64(stat.Bsize)
	if available < s.policy.MinimumFreeBytes {
		return fmt.Errorf("insufficient free disk space")
	}
	return nil
}

func (s *Service) checkBackupWritable() error {
	if err := ensureManagedDirectory(s.policy.BackupDirectory, 0700, true); err != nil {
		return err
	}
	file, err := os.CreateTemp(s.policy.BackupDirectory, ".preflight-*")
	if err != nil {
		return err
	}
	path := file.Name()
	if err := file.Chmod(0600); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return os.Remove(path)
}

func (s *Service) pullAndVerify(ctx context.Context, manifest *updatecontract.Manifest) error {
	immutable := manifest.ImmutableImage()
	if err := s.runDocker(ctx, nil, io.Discard, "pull", immutable); err != nil {
		return fmt.Errorf("approved image pull failed")
	}
	digests, err := s.imageDigests(ctx, immutable)
	if err != nil {
		return fmt.Errorf("approved image digest inspection failed")
	}
	want := imageRepository(manifest.Image) + "@" + manifest.ImageDigest
	for _, digest := range digests {
		if digest == want {
			return nil
		}
	}
	return fmt.Errorf("pulled image digest does not match the approved manifest")
}

func (s *Service) createBackup(
	ctx context.Context,
	updateID, sourceVersion, targetVersion string,
	sourceMigration int,
) (_ *backupMetadata, resultErr error) {
	if err := ensureManagedDirectory(s.policy.BackupDirectory, 0700, true); err != nil {
		return nil, err
	}
	directory := filepath.Join(s.policy.BackupDirectory, s.now().UTC().Format("20060102T150405Z")+"-"+updateID)
	if err := os.Mkdir(directory, 0700); err != nil {
		return nil, err
	}
	if err := validateManagedDirectory(directory, true); err != nil {
		return nil, err
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.RemoveAll(directory)
		}
	}()

	environmentCopy := filepath.Join(directory, "deployment.env")
	composeCopy := filepath.Join(directory, "docker-compose.yml")
	databaseBackup := filepath.Join(directory, "postgres.dump")
	if err := copyRegularFile(s.policy.EnvironmentFile, environmentCopy, 0600, true); err != nil {
		return nil, err
	}
	if err := copyRegularFile(s.policy.ComposeFile, composeCopy, 0600, false); err != nil {
		return nil, err
	}
	sourceImage, err := deploymentImageFromEnvironment(s.policy.EnvironmentFile)
	if err != nil {
		return nil, err
	}
	digests, err := s.imageDigests(ctx, sourceImage)
	if err != nil || len(digests) == 0 {
		return nil, fmt.Errorf("current image digest is unavailable")
	}
	sourceDigest := selectImageDigest(sourceImage, digests)
	if sourceDigest == "" {
		return nil, fmt.Errorf("current image digest is unavailable")
	}

	dump, err := os.OpenFile(databaseBackup, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	if info, statErr := dump.Stat(); statErr != nil || validateManagedFileInfo(info, true) != nil {
		_ = dump.Close()
		return nil, fmt.Errorf("invalid database backup file")
	}
	dumpErr := s.runDocker(ctx, nil, dump, s.composeArgs(
		"exec", "-T", s.policy.DatabaseService, "pg_dump", "-U", s.policy.DatabaseUser,
		"-d", s.policy.DatabaseName, "--format=custom", "--no-owner", "--no-privileges",
	)...)
	closeErr := dump.Close()
	if dumpErr != nil || closeErr != nil {
		return nil, fmt.Errorf("postgresql backup failed")
	}

	metadata := &backupMetadata{
		UpdateID: updateID, Directory: directory, DatabaseBackup: databaseBackup,
		EnvironmentCopy: environmentCopy, ComposeCopy: composeCopy,
		SourceVersion: sourceVersion, TargetVersion: targetVersion,
		SourceImage: sourceImage, SourceDigest: sourceDigest, SourceMigration: sourceMigration,
		CreatedAt: s.now().UTC(),
	}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil || atomicWriteFile(filepath.Join(directory, "metadata.json"), append(data, '\n'), 0600) != nil {
		return nil, fmt.Errorf("backup metadata failed")
	}
	if err := s.validateBackupMetadata(metadata); err != nil {
		return nil, err
	}
	complete = true
	return metadata, nil
}

func (s *Service) runMigrations(ctx context.Context) error {
	return s.runDocker(ctx, nil, io.Discard, s.composeArgs(
		"run", "--rm", "--no-deps", s.policy.ApplicationService, "/app/sub2api", "--migrate",
	)...)
}

func (s *Service) stopApplication(ctx context.Context) error {
	return s.runDocker(ctx, nil, io.Discard, s.composeArgs("stop", s.policy.ApplicationService)...)
}

func (s *Service) startApplication(ctx context.Context) error {
	return s.runDocker(ctx, nil, io.Discard, s.composeArgs("up", "-d", "--no-deps", s.policy.ApplicationService)...)
}

func (s *Service) validateDeployment(ctx context.Context, expectedMigration int) error {
	deadline := time.Now().Add(s.policy.healthTimeout())
	var lastErr error
	for {
		lastErr = s.validateDeploymentOnce(ctx, expectedMigration)
		if lastErr == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("post-update health validation failed")
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("post-update health validation canceled")
		case <-time.After(time.Second):
		}
	}
}

func (s *Service) validateDeploymentOnce(ctx context.Context, expectedMigration int) error {
	for _, path := range []string{"/health", "/", "/api/v1/settings/public"} {
		if err := s.checkHTTP(ctx, path); err != nil {
			return err
		}
	}
	if err := s.checkDatabase(ctx); err != nil {
		return err
	}
	if err := s.checkRedis(ctx); err != nil {
		return err
	}
	migration, err := s.currentMigration(ctx)
	if err != nil || migration != expectedMigration {
		return fmt.Errorf("migration target validation failed")
	}
	return nil
}

func (s *Service) restoreImmediate(ctx context.Context, backup *backupMetadata, forceDatabaseRestore bool) error {
	if err := s.validateBackupMetadata(backup); err != nil {
		return fmt.Errorf("rollback backup validation failed")
	}
	if err := s.stopApplication(ctx); err != nil {
		return fmt.Errorf("stop application before restore failed")
	}
	restoreDatabase := forceDatabaseRestore
	if !restoreDatabase {
		currentMigration, err := s.currentMigration(ctx)
		if err != nil {
			return err
		}
		if currentMigration > backup.SourceMigration {
			restoreDatabase = true
		} else if currentMigration < backup.SourceMigration {
			return fmt.Errorf("database migration state moved backwards unexpectedly")
		}
	}
	if restoreDatabase {
		dump, err := openManagedFile(backup.DatabaseBackup, 0, true)
		if err != nil {
			return err
		}
		restoreErr := s.runDocker(ctx, dump, io.Discard, s.composeArgs(
			"exec", "-T", s.policy.DatabaseService, "pg_restore", "-U", s.policy.DatabaseUser,
			"-d", s.policy.DatabaseName, "--clean", "--if-exists", "--no-owner", "--no-privileges",
		)...)
		_ = dump.Close()
		if restoreErr != nil {
			return fmt.Errorf("database restore failed")
		}
	}
	if err := s.restoreApplication(ctx, backup); err != nil {
		return err
	}
	return s.validateDeployment(ctx, backup.SourceMigration)
}

func (s *Service) restoreApplication(ctx context.Context, backup *backupMetadata) error {
	if err := s.runDocker(ctx, nil, io.Discard, "pull", backup.SourceDigest); err != nil {
		return fmt.Errorf("previous image pull failed")
	}
	if err := copyRegularFile(backup.EnvironmentCopy, s.policy.EnvironmentFile, 0600, true); err != nil {
		return fmt.Errorf("restore deployment environment failed")
	}
	if err := rewriteEnvironmentImage(s.policy.EnvironmentFile, backup.SourceDigest); err != nil {
		return fmt.Errorf("pin previous image failed")
	}
	return s.startApplication(ctx)
}

func (s *Service) checkHTTP(ctx context.Context, path string) error {
	endpoint := strings.TrimSuffix(s.policy.HealthBaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := s.healthHTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("health endpoint returned status %d", resp.StatusCode)
	}
	return nil
}

func (s *Service) checkDatabase(ctx context.Context) error {
	return s.runDocker(ctx, nil, io.Discard, s.composeArgs(
		"exec", "-T", s.policy.DatabaseService, "pg_isready", "-U", s.policy.DatabaseUser, "-d", s.policy.DatabaseName,
	)...)
}

func (s *Service) checkRedis(ctx context.Context) error {
	return s.runDocker(ctx, nil, io.Discard, s.composeArgs(
		"exec", "-T", s.policy.RedisService, "redis-cli", "ping",
	)...)
}

func (s *Service) currentMigration(ctx context.Context) (int, error) {
	output, err := s.commandOutput(ctx, s.composeArgs(
		"exec", "-T", s.policy.DatabaseService, "psql", "-U", s.policy.DatabaseUser,
		"-d", s.policy.DatabaseName, "-Atc", migrationQuery,
	)...)
	if err != nil {
		return 0, fmt.Errorf("read migration state failed")
	}
	return parseMigrationOutput(output)
}

func (s *Service) imageDigests(ctx context.Context, image string) ([]string, error) {
	output, err := s.commandOutput(ctx, "image", "inspect", "--format", "{{json .RepoDigests}}", image)
	if err != nil {
		return nil, err
	}
	var digests []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &digests); err != nil {
		return nil, fmt.Errorf("invalid Docker digest output")
	}
	return digests, nil
}

func (s *Service) composeArgs(args ...string) []string {
	prefix := []string{
		"compose", "--project-directory", s.policy.DeploymentDirectory,
		"-f", s.policy.ComposeFile, "--env-file", s.policy.EnvironmentFile,
	}
	return append(prefix, args...)
}

func (s *Service) runDocker(ctx context.Context, stdin io.Reader, stdout io.Writer, args ...string) error {
	return s.runner.Run(ctx, stdin, stdout, s.policy.DockerBinary, args...)
}

func (s *Service) commandOutput(ctx context.Context, args ...string) (string, error) {
	var output boundedBuffer
	if err := s.runDocker(ctx, nil, &output, args...); err != nil {
		return "", err
	}
	return output.String(), nil
}

func containsLine(output, wanted string) bool {
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == wanted {
			return true
		}
	}
	return false
}

func deploymentImageFromEnvironment(path string) (string, error) {
	data, err := readManagedFile(path, 2*1024*1024, true)
	if err != nil {
		return "", err
	}
	image := ""
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "SUB2API_IMAGE=") {
			if image != "" {
				return "", fmt.Errorf("duplicate SUB2API_IMAGE entries")
			}
			image = strings.TrimSpace(strings.TrimPrefix(line, "SUB2API_IMAGE="))
		}
	}
	if image == "" {
		image = defaultDeploymentImage
	}
	if strings.ContainsAny(image, "\r\n\x00") {
		return "", fmt.Errorf("invalid SUB2API_IMAGE value")
	}
	return image, nil
}

func rewriteEnvironmentImage(path, image string) error {
	if strings.TrimSpace(image) != image || strings.ContainsAny(image, "\r\n\x00") {
		return fmt.Errorf("invalid image value")
	}
	info, err := validateManagedRegularFile(path, 2*1024*1024, true)
	if err != nil {
		return fmt.Errorf("invalid environment file")
	}
	data, err := readManagedFile(path, 2*1024*1024, true)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	found := false
	for index, line := range lines {
		if strings.HasPrefix(line, "SUB2API_IMAGE=") {
			if found {
				return fmt.Errorf("duplicate SUB2API_IMAGE entries")
			}
			lines[index] = "SUB2API_IMAGE=" + image
			found = true
		}
	}
	if !found {
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
		lines = append(lines, "SUB2API_IMAGE="+image, "")
	}
	return atomicWriteFile(path, []byte(strings.Join(lines, "\n")), info.Mode().Perm())
}

func copyRegularFile(source, destination string, mode os.FileMode, sourcePrivate bool) error {
	data, err := readManagedFile(source, 16*1024*1024, sourcePrivate)
	if err != nil {
		return fmt.Errorf("invalid backup source")
	}
	return atomicWriteFile(destination, data, mode)
}

func (s *Service) validateBackupMetadata(backup *backupMetadata) error {
	if backup == nil || backup.Directory == s.policy.BackupDirectory || !pathWithin(s.policy.BackupDirectory, backup.Directory) {
		return fmt.Errorf("invalid backup directory")
	}
	if err := validateManagedDirectory(backup.Directory, true); err != nil {
		return err
	}
	expected := map[string]string{
		backup.DatabaseBackup:  filepath.Join(backup.Directory, "postgres.dump"),
		backup.EnvironmentCopy: filepath.Join(backup.Directory, "deployment.env"),
		backup.ComposeCopy:     filepath.Join(backup.Directory, "docker-compose.yml"),
	}
	for path, wanted := range expected {
		if path != wanted {
			return fmt.Errorf("invalid backup path")
		}
		if _, err := validateManagedRegularFile(path, 0, true); err != nil {
			return err
		}
	}
	if _, err := validateManagedRegularFile(filepath.Join(backup.Directory, "metadata.json"), maxStateBytes, true); err != nil {
		return err
	}
	return nil
}

func selectImageDigest(image string, digests []string) string {
	repository := imageRepository(image)
	for _, digest := range digests {
		if strings.HasPrefix(digest, repository+"@sha256:") {
			return digest
		}
	}
	return ""
}

func imageRepository(image string) string {
	image = strings.SplitN(image, "@", 2)[0]
	lastSlash := strings.LastIndex(image, "/")
	if colon := strings.LastIndex(image, ":"); colon > lastSlash {
		image = image[:colon]
	}
	return image
}
