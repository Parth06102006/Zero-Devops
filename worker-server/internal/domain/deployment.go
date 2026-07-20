package domain

import "context"

// DeploymentRepository defines the interface for deployment data operations.
type DeploymentRepository interface {
	Insert(ctx context.Context, job DeployJob) error
	UpdateStatus(ctx context.Context, deploymentID string, status string, retryCount int) error
	UpdateOutputURL(ctx context.Context, deploymentID string, outputURL string) error
	ReadImageTag(ctx context.Context, deploymentID string) (string, error)
	MarkBuilding(ctx context.Context, deploymentID string) error
	MarkFailed(ctx context.Context, deploymentID string, errMsg string) error
	MarkCanceled(ctx context.Context, deploymentID string, errMsg string) error
	MarkFinished(ctx context.Context, deploymentID string, outputURL string) error
}
