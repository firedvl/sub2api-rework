package updater

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/updatecontract"
)

const (
	defaultDeploymentImage = "weishaw/sub2api:latest"
	migrationQuery         = "SELECT COALESCE(MAX((regexp_match(filename, '^([0-9]+)'))[1]::int), 0) FROM schema_migrations"
)

type renderedComposeConfig struct {
	Services map[string]renderedComposeService `json:"services"`
	Volumes  map[string]renderedComposeVolume  `json:"volumes"`
}

type renderedComposeService struct {
	Image       string                 `json:"image"`
	Environment json.RawMessage        `json:"environment"`
	GroupAdd    []string               `json:"group_add"`
	Volumes     []renderedComposeMount `json:"volumes"`
	VolumesFrom []string               `json:"volumes_from"`
}

type renderedComposeMount struct {
	Type   string `json:"type"`
	Source string `json:"source"`
	Target string `json:"target"`
}

type renderedComposeVolume struct {
	Name       string            `json:"name"`
	DriverOpts map[string]string `json:"driver_opts"`
}

type inspectedDockerVolume struct {
	Driver  string            `json:"Driver"`
	Options map[string]string `json:"Options"`
}

func (s *Service) preflight(ctx context.Context, manifest *updatecontract.Manifest, requireApplicationHealth bool) (int, error) {
	if err := s.validateDeploymentFiles(ctx); err != nil {
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

func (s *Service) validateDeploymentFiles(ctx context.Context) error {
	for _, path := range s.policy.ComposeFiles {
		if _, err := readManagedFile(path, 2*1024*1024, false); err != nil {
			return fmt.Errorf("invalid deployment file")
		}
	}
	if _, err := validateManagedRegularFile(s.policy.EnvironmentFile, 2*1024*1024, true); err != nil {
		return fmt.Errorf("invalid deployment file")
	}
	merged, err := s.commandOutput(ctx, s.composeArgs("config", "--no-interpolate", "--format", "json")...)
	if err != nil {
		return fmt.Errorf("merged compose configuration is invalid")
	}
	var ownership renderedComposeConfig
	if err := json.Unmarshal([]byte(merged), &ownership); err != nil {
		return fmt.Errorf("merged compose configuration is invalid")
	}
	application, ok := ownership.Services[s.policy.ApplicationService]
	if !ok || !usesManagedImageVariable(application.Image) {
		return fmt.Errorf("application image must use SUB2API_IMAGE")
	}
	merged, err = s.commandOutput(ctx, s.composeArgs("config", "--format", "json")...)
	if err != nil {
		return fmt.Errorf("merged compose configuration is invalid")
	}
	var compose renderedComposeConfig
	if err := json.Unmarshal([]byte(merged), &compose); err != nil {
		return fmt.Errorf("merged compose configuration is invalid")
	}
	application, ok = compose.Services[s.policy.ApplicationService]
	if !ok {
		return fmt.Errorf("application service missing")
	}
	if _, ok := compose.Services[s.policy.DatabaseService]; !ok {
		return fmt.Errorf("database service missing")
	}
	if _, ok := compose.Services[s.policy.RedisService]; !ok {
		return fmt.Errorf("redis service missing")
	}
	socketDirectory := filepath.Dir(s.policy.SocketPath)
	socketPath, hasSocketPath := composeEnvironmentValue(application.Environment, "SUB2API_UPDATER_SOCKET")
	updaterGID, hasUpdaterGID := composeEnvironmentValue(application.Environment, "SUB2API_UPDATER_GID")
	hasSocketMount := false
	if len(application.VolumesFrom) != 0 {
		return fmt.Errorf("application must not use volumes_from")
	}
	for _, volume := range application.Volumes {
		if volume.Type == "bind" && bindExposesDockerSocket(volume.Source) || filepath.Base(filepath.Clean(volume.Target)) == "docker.sock" {
			return fmt.Errorf("application must not mount the Docker socket")
		}
		if volume.Type == "volume" {
			declared, exists := compose.Volumes[volume.Source]
			if !exists || declared.Name == "" {
				return fmt.Errorf("application named volume is invalid")
			}
			if bindExposesDockerSocket(declared.DriverOpts["device"]) {
				return fmt.Errorf("application must not mount the Docker socket")
			}
			output, err := s.commandOutput(ctx, "volume", "inspect", "--format", "{{json .}}", declared.Name)
			var inspected inspectedDockerVolume
			if err != nil || json.Unmarshal([]byte(output), &inspected) != nil || inspected.Driver != "local" {
				return fmt.Errorf("application named volume is unsupported")
			}
			if bindExposesDockerSocket(inspected.Options["device"]) {
				return fmt.Errorf("application must not mount the Docker socket")
			}
			if len(declared.DriverOpts) != 0 || len(inspected.Options) != 0 {
				return fmt.Errorf("application named volume is unsupported")
			}
		}
		if volume.Type == "bind" && volume.Source == socketDirectory && volume.Target == socketDirectory {
			hasSocketMount = true
		}
	}
	if !hasSocketPath || socketPath != s.policy.SocketPath || !hasUpdaterGID || !containsString(application.GroupAdd, updaterGID) || !hasSocketMount {
		return fmt.Errorf("application updater socket access is incomplete")
	}
	return nil
}

func usesManagedImageVariable(image string) bool {
	if image == "${SUB2API_IMAGE}" {
		return true
	}
	const prefix = "${SUB2API_IMAGE:-"
	if !strings.HasPrefix(image, prefix) || !strings.HasSuffix(image, "}") {
		return false
	}
	fallback := image[len(prefix) : len(image)-1]
	return fallback != "" && !strings.ContainsAny(fallback, "${}")
}

func bindExposesDockerSocket(source string) bool {
	paths := []string{source}
	if resolved, err := filepath.EvalSymlinks(source); err == nil {
		paths = append(paths, resolved)
	}
	for _, path := range paths {
		path = filepath.Clean(path)
		if filepath.Base(path) == "docker.sock" || pathWithin(path, "/run/docker.sock") || pathWithin(path, "/var/run/docker.sock") {
			return true
		}
	}
	return false
}

func composeEnvironmentValue(data json.RawMessage, name string) (string, bool) {
	var list []string
	if err := json.Unmarshal(data, &list); err == nil {
		for _, entry := range list {
			key, value, found := strings.Cut(entry, "=")
			if found && key == name {
				return value, true
			}
		}
		return "", false
	}
	var values map[string]*string
	if err := json.Unmarshal(data, &values); err != nil {
		return "", false
	}
	value, ok := values[name]
	if !ok || value == nil {
		return "", false
	}
	return *value, true
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
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
	if err := s.runDocker(ctx, nil, io.Discard, "pull", manifest.Image); err != nil {
		return fmt.Errorf("approved image pull failed")
	}
	digests, err := s.imageDigests(ctx, manifest.Image)
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
	databaseBackup := filepath.Join(directory, "postgres.dump")
	if err := copyRegularFile(s.policy.EnvironmentFile, environmentCopy, 0600, true); err != nil {
		return nil, err
	}
	composeFiles, err := backupComposeFiles(s.policy.ComposeFiles, directory)
	if err != nil {
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
		EnvironmentCopy: environmentCopy, ComposeFiles: composeFiles,
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
		if err := s.recreateDatabase(ctx); err != nil {
			_ = dump.Close()
			return err
		}
		restoreErr := s.runDocker(ctx, dump, io.Discard, s.composeArgs(
			"exec", "-T", s.policy.DatabaseService, "pg_restore", "-U", s.policy.DatabaseUser,
			"-d", s.policy.DatabaseName, "--no-owner", "--no-privileges",
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

func (s *Service) recreateDatabase(ctx context.Context) error {
	maintenanceDatabase := "postgres"
	if s.policy.DatabaseName == maintenanceDatabase {
		maintenanceDatabase = "template1"
	}
	database := `"` + s.policy.DatabaseName + `"`
	owner := `"` + s.policy.DatabaseUser + `"`
	if err := s.runDocker(ctx, nil, io.Discard, s.composeArgs(
		"exec", "-T", s.policy.DatabaseService, "psql", "-U", s.policy.DatabaseUser,
		"-d", maintenanceDatabase, "-v", "ON_ERROR_STOP=1",
		"-c", "DROP DATABASE IF EXISTS "+database+" WITH (FORCE)",
		"-c", "CREATE DATABASE "+database+" OWNER "+owner,
	)...); err != nil {
		return fmt.Errorf("database recreation failed")
	}
	return nil
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
	prefix := []string{"compose", "--project-directory", s.policy.DeploymentDirectory}
	for _, path := range s.policy.ComposeFiles {
		prefix = append(prefix, "-f", path)
	}
	prefix = append(prefix, "--env-file", s.policy.EnvironmentFile)
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

func backupComposeFiles(paths []string, directory string) ([]backupComposeFile, error) {
	files := make([]backupComposeFile, 0, len(paths))
	for index, originalPath := range paths {
		data, err := readManagedFile(originalPath, 2*1024*1024, false)
		if err != nil {
			return nil, fmt.Errorf("invalid backup source")
		}
		backupPath := filepath.Join(directory, fmt.Sprintf("compose-%03d.yaml", index))
		if err := atomicWriteFile(backupPath, data, 0600); err != nil {
			return nil, err
		}
		files = append(files, backupComposeFile{
			OriginalPath: originalPath,
			BackupPath:   backupPath,
			SHA256:       composeChecksum(data),
		})
	}
	return files, nil
}

func composeChecksum(data []byte) string {
	digest := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", digest)
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
	if len(backup.ComposeFiles) != len(s.policy.ComposeFiles) {
		return fmt.Errorf("invalid backup compose file set")
	}
	for index, file := range backup.ComposeFiles {
		if file.OriginalPath != s.policy.ComposeFiles[index] ||
			file.BackupPath != filepath.Join(backup.Directory, fmt.Sprintf("compose-%03d.yaml", index)) {
			return fmt.Errorf("invalid backup compose file identity")
		}
		data, err := readManagedFile(file.BackupPath, 2*1024*1024, true)
		if err != nil || file.SHA256 != composeChecksum(data) {
			return fmt.Errorf("invalid backup compose file checksum")
		}
	}
	data, err := readManagedFile(filepath.Join(backup.Directory, "metadata.json"), maxStateBytes, true)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var recorded backupMetadata
	if err := decoder.Decode(&recorded); err != nil || !reflect.DeepEqual(recorded, *backup) {
		return fmt.Errorf("backup metadata does not match recorded state")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("invalid backup metadata")
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
