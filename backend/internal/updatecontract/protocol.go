package updatecontract

import "time"

type Operation string

const (
	OperationPrepare  Operation = "prepare"
	OperationInstall  Operation = "install"
	OperationRollback Operation = "rollback"
)

type UpdaterState string

const (
	UpdaterStateUnavailable UpdaterState = "unavailable"
	UpdaterStateIdle        UpdaterState = "idle"
	UpdaterStatePreparing   UpdaterState = "preparing"
	UpdaterStatePrepared    UpdaterState = "prepared"
	UpdaterStateInstalling  UpdaterState = "installing"
	UpdaterStateRollingBack UpdaterState = "rolling_back"
	UpdaterStateSucceeded   UpdaterState = "succeeded"
	UpdaterStateFailed      UpdaterState = "failed"
	UpdaterStateCritical    UpdaterState = "critical"
)

// OperationRequest is intentionally incapable of carrying an image, path, or command.
type OperationRequest struct {
	Version      string `json:"version"`
	Confirmation string `json:"confirmation,omitempty"`
	Actor        string `json:"actor"`
}

type OperationAccepted struct {
	OperationID string    `json:"operation_id"`
	Action      Operation `json:"action"`
	State       string    `json:"state"`
}

type OperationSummary struct {
	OperationID    string    `json:"operation_id"`
	Action         Operation `json:"action"`
	Actor          string    `json:"actor"`
	SourceVersion  string    `json:"source_version"`
	TargetVersion  string    `json:"target_version"`
	StartedAt      time.Time `json:"started_at"`
	FinishedAt     time.Time `json:"finished_at,omitempty"`
	Result         string    `json:"result"`
	RollbackResult string    `json:"rollback_result,omitempty"`
	Error          string    `json:"error,omitempty"`
}

type UpdaterStatus struct {
	SchemaVersion    int               `json:"schema_version"`
	UpdaterVersion   string            `json:"updater_version"`
	Healthy          bool              `json:"healthy"`
	State            UpdaterState      `json:"state"`
	Busy             bool              `json:"busy"`
	InstalledVersion string            `json:"installed_version"`
	PreparedVersion  string            `json:"prepared_version,omitempty"`
	RollbackVersion  string            `json:"rollback_version,omitempty"`
	CurrentMigration int               `json:"current_migration"`
	LastAttempt      *OperationSummary `json:"last_attempt,omitempty"`
	LastRollback     *OperationSummary `json:"last_rollback,omitempty"`
	LastError        string            `json:"last_error,omitempty"`
	UpdatedAt        time.Time         `json:"updated_at"`
}
