//go:build unit

package updater

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPolicyFileMustBeRootOwnedAndNotWritableByOthers(t *testing.T) {
	require.NoError(t, validatePolicyFileSecurity(0600, 0))
	require.Error(t, validatePolicyFileSecurity(0600, 1000))
	require.Error(t, validatePolicyFileSecurity(os.FileMode(0620), 0))
}

func testPolicy(root string) Policy {
	deploy := filepath.Join(root, "deploy")
	return Policy{
		SchemaVersion: 2, SocketPath: filepath.Join(root, "run", "updater.sock"),
		StatePath: filepath.Join(root, "state", "state.json"), AuditPath: filepath.Join(root, "state", "audit.jsonl"),
		LockPath: filepath.Join(root, "state", "operation.lock"), BackupDirectory: filepath.Join(root, "backups"),
		DeploymentDirectory: deploy, ComposeFiles: []string{filepath.Join(deploy, "docker-compose.yml")},
		EnvironmentFile: filepath.Join(deploy, ".env"), DockerBinary: "/usr/bin/docker",
		ApplicationService: "sub2api", DatabaseService: "postgres", RedisService: "redis",
		DatabaseUser: "sub2api", DatabaseName: "sub2api",
		TrustedImageRepository: "ghcr.io/firedvl/sub2api-rework",
		ManifestBaseURL:        "https://github.com/firedvl/sub2api-rework/releases/download",
		HealthBaseURL:          "http://127.0.0.1:8080", MinimumFreeBytes: 1024,
		OperationTimeoutSeconds: 300, HealthTimeoutSeconds: 30,
		InitialInstalledVersion: "0.1.183-rework.1", InitialMigration: 232,
	}
}

func TestPolicyRejectsTrustBoundaryExpansion(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name   string
		mutate func(*Policy)
	}{
		{"relative path", func(p *Policy) { p.ComposeFiles = []string{"docker-compose.yml"} }},
		{"outside deployment", func(p *Policy) { p.EnvironmentFile = filepath.Join(root, "secrets.env") }},
		{"arbitrary binary", func(p *Policy) { p.DockerBinary = "/bin/sh" }},
		{"wrong repository", func(p *Policy) { p.TrustedImageRepository = "ghcr.io/attacker/image" }},
		{"wrong manifest source", func(p *Policy) { p.ManifestBaseURL = "https://example.com/releases" }},
		{"public health target", func(p *Policy) { p.HealthBaseURL = "https://gateway.example.com" }},
		{"automatic policy identity", func(p *Policy) { p.InitialInstalledVersion = "latest" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy := testPolicy(root)
			test.mutate(&policy)
			require.Error(t, policy.Validate())
		})
	}
}
