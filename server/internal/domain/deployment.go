package domain

import (
	"context"
	"time"
)

// DeploymentStatus represents the status of a deployment
type DeploymentStatus string

const (
	// DeploymentStatusPending indicates a pending deployment
	DeploymentStatusPending DeploymentStatus = "pending"
	// DeploymentStatusBuilding indicates a deployment in progress
	DeploymentStatusBuilding DeploymentStatus = "building"
	// DeploymentStatusSuccess indicates a successful deployment
	DeploymentStatusSuccess DeploymentStatus = "success"
	// DeploymentStatusFailed indicates a failed deployment
	DeploymentStatusFailed DeploymentStatus = "failed"
	// DeploymentStatusCanceled indicates a canceled deployment
	DeploymentStatusCanceled DeploymentStatus = "canceled"
)

// Deployment represents a deployment record
type Deployment struct {
	ID                        string             `json:"id"`
	UserID                    string             `json:"user_id"`
	RepoID                    int64              `json:"repo_id"`
	CloneURL                  string             `json:"clone_url"`
	Status                    DeploymentStatus   `json:"status"`
	ProjectID                 string             `json:"project_id,omitempty"`
	GithubInstallationID      string             `json:"github_installation_id,omitempty"`
	CommitSHA                 string             `json:"commit_sha,omitempty"`
	RequestedRef              string             `json:"requested_ref,omitempty"`
	Trigger                   string             `json:"trigger,omitempty"`
	DesiredRevisionGeneration int64              `json:"desired_revision_generation,omitempty"`
	ConfigurationSnapshot     BuildConfiguration `json:"configuration_snapshot,omitempty"`
	ConfigurationVersion      int                `json:"configuration_version,omitempty"`
	CommandPolicyVersion      string             `json:"command_policy_version,omitempty"`
	CommandScanResult         CommandScanResult  `json:"command_scan_result,omitempty"`
	ManualIdempotencyKey      string             `json:"manual_idempotency_key,omitempty"`
	OutputURL                 string             `json:"output_url,omitempty"`
	ErrorMessage              string             `json:"error_message,omitempty"`
	CreatedAt                 time.Time          `json:"created_at"`
	UpdatedAt                 time.Time          `json:"updated_at"`
}

// CreateProjectBuildParams carries the authenticated manual-build request. The
// project ID comes from the route; sha_or_ref and idempotency_key come from the
// request body. The usecase resolves ShaOrRef to an immutable full commit SHA.
type CreateProjectBuildParams struct {
	ProjectID      string `json:"project_id"`
	ShaOrRef       string `json:"sha_or_ref"`
	IdempotencyKey string `json:"idempotency_key"`
	CorrelationID  string `json:"correlation_id"`
}

// DeploymentUsecase defines the interface for deployment use cases
type DeploymentUsecase interface {
	CreateDeployment(ctx context.Context, userID string, repoID int64, reqID string) (*Deployment, error)
	CreateProjectBuild(ctx context.Context, userID string, params CreateProjectBuildParams) (*Deployment, error)
	GetDeployments(ctx context.Context, userID string) ([]Deployment, error)
	GetDeploymentByID(ctx context.Context, userID, deploymentID string) (*Deployment, error)
	ListProjectBuilds(ctx context.Context, userID, projectID string) ([]Deployment, error)
	GetBuild(ctx context.Context, userID, buildID string) (*Deployment, error)
}

// DeploymentRepository defines the interface for deployment data operations
type DeploymentRepository interface {
	Store(ctx context.Context, d *Deployment) error
	StoreProjectBuild(ctx context.Context, d *Deployment) error
	GetByUserID(ctx context.Context, userID string) ([]Deployment, error)
	GetByID(ctx context.Context, userID, id string) (*Deployment, error)
	GetByProjectID(ctx context.Context, userID, projectID string) ([]Deployment, error)
	UpdateStatus(ctx context.Context, deploymentID string, status DeploymentStatus) error
	UpdateOutputURL(ctx context.Context, deploymentID string, outputURL string) error
	UpdateErrorMessage(ctx context.Context, deploymentID string, errMsg string) error
}
