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
	ID                        string           `json:"id"`
	UserID                    string           `json:"user_id"`
	RepoID                    int64            `json:"repo_id"`
	CloneURL                  string           `json:"clone_url"`
	Status                    DeploymentStatus `json:"status"`
	ProjectID                 string           `json:"project_id,omitempty"`
	GithubInstallationID      string           `json:"github_installation_id,omitempty"`
	CommitSHA                 string           `json:"commit_sha,omitempty"`
	RequestedRef              string           `json:"requested_ref,omitempty"`
	Trigger                   string           `json:"trigger,omitempty"`
	DesiredRevisionGeneration int64            `json:"desired_revision_generation,omitempty"`
	// BuildNumber is the per-project human-readable sequence (1, 2, 3, ...)
	// shared by manual and webhook builds. Assigned transactionally from
	// projects.next_build_number by the store-with-outbox functions; legacy
	// rows without a project have no number (0).
	BuildNumber           int64              `json:"build_number,omitempty"`
	ConfigurationSnapshot BuildConfiguration `json:"configuration_snapshot,omitempty"`
	ConfigurationVersion  int                `json:"configuration_version,omitempty"`
	CommandPolicyVersion  string             `json:"command_policy_version,omitempty"`
	CommandScanResult     CommandScanResult  `json:"command_scan_result,omitempty"`
	ManualIdempotencyKey  string             `json:"manual_idempotency_key,omitempty"`
	OutputURL             string             `json:"output_url,omitempty"`
	ErrorMessage          string             `json:"error_message,omitempty"`
	CreatedAt             time.Time          `json:"created_at"`
	UpdatedAt             time.Time          `json:"updated_at"`
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

// DeploymentUsecase defines the interface for deployment use cases.
// (Legacy CreateDeployment was removed 2026-09-06 together with the
// POST /deploy route; builds are created exclusively through
// CreateProjectBuild / webhook pushes.)
type DeploymentUsecase interface {
	CreateProjectBuild(ctx context.Context, userID string, params CreateProjectBuildParams) (*Deployment, error)
	GetDeployments(ctx context.Context, userID string) ([]Deployment, error)
	GetDeploymentByID(ctx context.Context, userID, deploymentID string) (*Deployment, error)
	ListProjectBuilds(ctx context.Context, userID, projectID string) ([]Deployment, error)
	GetBuild(ctx context.Context, userID, buildID string) (*Deployment, error)
}

// WebhookBuildEventType is the deployment_outbox event type for build requests
// created by webhook pushes. The outbox payload is a deploy.jobs V1
// BuildRequestV1 JSON document.
const WebhookBuildEventType = "deploy.jobs"

// OutboxState enumerates the deployment_outbox.state lifecycle.
const (
	OutboxStatePending    = "pending"
	OutboxStatePublishing = "publishing"
	OutboxStateSent       = "sent"
	OutboxStateFailed     = "failed"
	OutboxStateDeadLetter = "dead_letter"
)

// StoreWebhookBuildParams carries the immutable inputs for a webhook-triggered
// build. The repository advances the project's desired-revision generation
// and consumes a build number inside the same transaction as the deployments
// row and deploy.jobs V1 outbox event, so a redelivered webhook delivery can
// never bump the generation without also creating the build (the unique
// webhook_delivery_id violation rolls the whole transaction back).
type StoreWebhookBuildParams struct {
	ProjectID             string
	UserID                string
	RepoID                int64
	CloneURL              string
	GithubInstallationID  string // FK -> github_installations.id (DB UUID)
	WebhookDeliveryID     string // FK -> webhook_deliveries.id (DB UUID)
	CommitSHA             string // push payload.After (authoritative)
	RequestedRef          string // push payload.Ref
	ConfigurationSnapshot BuildConfiguration
	ConfigurationVersion  int
	CommandPolicyVersion  string
	CommandScanResult     CommandScanResult
	EventID               string // deploy.jobs event ID (V1 payload event_id)
	InstallationID        int64  // external GitHub installation ID (V1 payload installation_id)
	CorrelationID         string
}

// OutboxEvent is one deployment_outbox row used by the dispatcher.
type OutboxEvent struct {
	ID             string
	DeploymentID   string
	EventType      string
	MessageVersion int
	Payload        []byte
	State          string
	AttemptCount   int
	AvailableAt    time.Time
	SentAt         *time.Time
	LastError      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// StoreProjectBuildWithOutboxParams carries the immutable inputs for a
// manual project build plus the deploy.jobs V1 outbox payload fields.
// The repository persists the deployments row and the outbox event in one transaction.
type StoreProjectBuildWithOutboxParams struct {
	Deployment                *Deployment // pre-filled row fields (status pending, trigger, SHA, etc.)
	EventID                   string
	InstallationID            int64 // external GitHub installation ID (V1 payload)
	CorrelationID             string
	RepositoryFullName        string
	DesiredRevisionGeneration int64
}

// ApplyStatusParams is the worker-reported status update to apply atomically.
type ApplyStatusParams struct {
	DeploymentID string
	Status       DeploymentStatus
	OutputURL    string // optional; empty means leave unchanged
	ErrorMessage string // optional; empty means leave unchanged
}

// ApplyStatusResult is the outcome of ApplyStatusUpdate.
type ApplyStatusResult struct {
	Deployment          *Deployment
	IsCurrentGeneration bool // true when deployment.generation == project.desired_revision_generation
}

// DeploymentRepository defines the interface for deployment data operations
type DeploymentRepository interface {
	StoreWebhookBuildWithOutbox(ctx context.Context, params StoreWebhookBuildParams) (*Deployment, error)
	GetByUserID(ctx context.Context, userID string) ([]Deployment, error)
	GetByID(ctx context.Context, userID, id string) (*Deployment, error)
	GetByProjectID(ctx context.Context, userID, projectID string) ([]Deployment, error)
	StoreProjectBuildWithOutbox(ctx context.Context, params StoreProjectBuildWithOutboxParams) (*Deployment, error)
	ClaimOutboxBatch(ctx context.Context, limit int) ([]OutboxEvent, error)
	MarkOutboxSent(ctx context.Context, id string) error
	MarkOutboxPublishFailed(ctx context.Context, id string, errMsg string, nextAvailableAt time.Time) error
	ResetStuckPublishing(ctx context.Context, olderThan time.Duration) (int64, error)
	ApplyStatusUpdate(ctx context.Context, params ApplyStatusParams) (*ApplyStatusResult, error)
	DeleteSentOutboxOlderThan(ctx context.Context, olderThan time.Duration) (int64, error)
}
