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
	// DeploymentStatusRunning indicates a running deployment
	DeploymentStatusRunning DeploymentStatus = "running"
	// DeploymentStatusSuccess indicates a successful deployment
	DeploymentStatusSuccess DeploymentStatus = "success"
	// DeploymentStatusFailed indicates a failed deployment
	DeploymentStatusFailed DeploymentStatus = "failed"
	// DeploymentStatusCanceled indicates a canceled deployment
	DeploymentStatusCanceled DeploymentStatus = "canceled"
)

// Deployment represents a deployment record
type Deployment struct {
	ID        int64            `json:"id"`
	UserID    int64            `json:"user_id"`
	RepoID    int64            `json:"repo_id"`
	CloneURL  string           `json:"clone_url"`
	Status    DeploymentStatus `json:"status"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}

// DeploymentUsecase defines the interface for deployment use cases
type DeploymentUsecase interface {
	CreateDeployment(ctx context.Context, userID int64, repoID int64, reqID string) (*Deployment, error)
	GetDeployments(ctx context.Context, userID int64) ([]Deployment, error)
	GetDeploymentByID(ctx context.Context, userID, deploymentID int64) (*Deployment, error)
}

// DeploymentRepository defines the interface for deployment data operations
type DeploymentRepository interface {
	Store(ctx context.Context, d *Deployment) error
	GetByUserID(ctx context.Context, userID int64) ([]Deployment, error)
	GetByID(ctx context.Context, userID, id int64) (*Deployment, error)
	UpdateStatus(ctx context.Context, deploymentID int64, status DeploymentStatus) error
}
