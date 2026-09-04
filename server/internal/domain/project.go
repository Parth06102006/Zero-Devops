package domain

import (
	"context"
	"time"
)

// BuildConfiguration is the approved, policy-validated build command snapshot
// stored on a project. It mirrors the deploy.jobs V1 `Configuration` (see
// server/internal/deployments/contract) but lives in domain so the projects
// layer does not depend on the contract package. It is a structured command
// (executable + argument array + working directory), never an arbitrary
// `sh -c` string, and the scanner policy version is recorded so rebuilds are
// reproducible and auditable.
type BuildConfiguration struct {
	Executable           string   `json:"executable"`
	Args                 []string `json:"args"`
	WorkingDir           string   `json:"working_dir"`
	ScannerPolicyVersion string   `json:"scanner_policy_version"`
}

// CommandScanResult records the outcome of validating a build command against
// the versioned scanner/policy. `command_policy_version` is also denormalized
// into its own column on the projects table for quick filtering, while the
// full result lives in the `command_scan_result` JSONB column.
type CommandScanResult struct {
	Status            string `json:"status"`         // approved | denied | pending
	PolicyVersion     string `json:"policy_version"` // e.g. "v1"
	Source            string `json:"source"`         // autoscan | manual
	DetectedFramework string `json:"detected_framework,omitempty"`
	Message           string `json:"message,omitempty"`
	DeniedReason      string `json:"denied_reason,omitempty"`
}

// CommandScanResultStatus enumerates the possible command scan outcomes.
const (
	CommandScanStatusApproved = "approved"
	CommandScanStatusDenied   = "denied"
	CommandScanStatusPending  = "pending"
)

// CommandScanSource enumerates how a scan was initiated.
const (
	CommandScanSourceManual   = "manual"
	CommandScanSourceAutoscan = "autoscan"
)

// CommandScanner validates a build command against a versioned security policy.
// It never blindly trusts a frontend-supplied command; an unsafe or
// non-allowlisted command yields a denied result so the caller can refuse to
// persist it. The scanner is synchronous and returns only the result, not an
// error, so policy denials are ordinary outcomes rather than failures.
type CommandScanner interface {
	Scan(ctx context.Context, cfg BuildConfiguration) CommandScanResult
}

// Project is a user's selected/configured deployable repository. It is the
// permanent identity (`ID`) of a configured project; `GitHubRepositoryID`
// identifies the repository even if its name changes. The inventory of all
// available repositories is NOT persisted — only this explicit selection is.
//
// A webhook may only create builds for a project that is `ProjectWebhookEnabled` and whose
// `ConfiguredBranch` exactly matches the push ref at webhook-processing time.
// `DesiredRevisionGeneration` is a monotonic counter advanced on every accepted
// trigger so a stale/older queued result cannot become the project's current
// successful result after a newer revision is accepted.
type Project struct {
	ID                        string             `json:"id"`
	UserID                    string             `json:"user_id"`
	InstallationID            string             `json:"installation_id"` // FK -> github_installations.id (UUID)
	GitHubRepositoryID        int64              `json:"github_repository_id"`
	RepositoryOwner           string             `json:"repository_owner"`
	RepositoryName            string             `json:"repository_name"`
	RepositoryFullName        string             `json:"repository_full_name"`
	ConfiguredBranch          string             `json:"configured_branch"` // stored as full ref, e.g. refs/heads/main
	ProjectWebhookEnabled     bool               `json:"project_webhook_enabled"`
	RepositoryAvailable       bool               `json:"repository_available"`
	DesiredRevisionGeneration int64              `json:"desired_revision_generation"`
	BuildConfiguration        BuildConfiguration `json:"build_configuration"`
	ConfigurationVersion      int                `json:"configuration_version"`
	CommandPolicyVersion      string             `json:"command_policy_version"`
	CommandScanResult         CommandScanResult  `json:"command_scan_result"`
	CreatedAt                 time.Time          `json:"created_at"`
	UpdatedAt                 time.Time          `json:"updated_at"`
}

// CreateProjectParams carries only the client-supplied inputs. The usecase is
// responsible for: verifying the repository is visible to the caller's active
// installation (never trusting a frontend repo_id alone), fetching repository
// display metadata from GitHub, normalizing the branch to a full ref, running
// the scanner/policy on the build command, and persisting the resulting
// snapshot. Do not persist the entire GitHub repository inventory here.
type CreateProjectParams struct {
	RepositoryID          int64              `json:"repository_id"`
	ConfiguredBranch      string             `json:"configured_branch"`
	ProjectWebhookEnabled bool               `json:"project_webhook_enabled"`
	BuildConfiguration    BuildConfiguration `json:"build_configuration"`
}

// UpdateProjectParams carries optional PATCH fields. Pointer fields distinguish
// "not provided" from "set to zero value"; only non-nil fields are applied.
// The configured branch may be changed only through this authenticated API;
// webhook handling reads the persisted value and never a client-supplied ref.
type UpdateProjectParams struct {
	ConfiguredBranch      *string             `json:"configured_branch,omitempty"`
	ProjectWebhookEnabled *bool               `json:"project_webhook_enabled,omitempty"`
	BuildConfiguration    *BuildConfiguration `json:"build_configuration,omitempty"`
}

// ProjectUsecase defines the project configuration business logic. Every
// method is scoped to the authenticated user and returns ErrNotFound (→ 404)
// rather than leaking another user's resource existence.
type ProjectUsecase interface {
	CreateProject(ctx context.Context, userID string, params CreateProjectParams) (*Project, error)
	ListProjects(ctx context.Context, userID string) ([]Project, error)
	GetProject(ctx context.Context, userID, projectID string) (*Project, error)
	UpdateProject(ctx context.Context, userID, projectID string, params UpdateProjectParams) (*Project, error)
	DeleteProject(ctx context.Context, userID, projectID string) error
}

// ProjectRepository defines the data operations for projects. The unique
// (user_id, github_repository_id) constraint prevents duplicate active
// selections for the same owner/repository; repositories outside this table
// are not persisted. `Store` populates `Project.ID` via RETURNING.
type ProjectRepository interface {
	Store(ctx context.Context, p *Project) error
	ListByUserID(ctx context.Context, userID string) ([]Project, error)
	GetByID(ctx context.Context, userID, id string) (*Project, error)
	Update(ctx context.Context, p *Project) error
	Delete(ctx context.Context, userID, id string) error
	GetProjectRepoAvailability(ctx context.Context, githubInstallationID string) (map[int64]bool, error)
	UpdateProjectRepoAvailability(ctx context.Context, githubInstallationID string, listOfRepos map[int64]bool) error
	GetByInstallationAndRepositoryID(ctx context.Context, githubInstallationID string, githubRepositoryID int64) (*Project, error)
	// IncrementDesiredRevisionGeneration atomically advances the project's
	// desired-revision counter and returns the new generation. It is keyed by
	// the webhook identity (installation + GitHub repository ID); a missing row
	// yields ErrNotFound.
	IncrementDesiredRevisionGeneration(ctx context.Context, githubInstallationID string, githubRepositoryID int64) (int64, error)
}
