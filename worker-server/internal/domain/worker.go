package domain

import (
	"context"

	"go.uber.org/zap"
)

// WorkerUsecase defines the interface for the deployment worker.
type WorkerUsecase interface {
	StartWorker(baseLogger *zap.Logger) error
}

// CloneCredential is an ephemeral GitHub installation credential. It must not
// be persisted, logged, or placed in RabbitMQ messages.
type CloneCredential struct {
	Username string `json:"username"`
	Token    string `json:"token"`
}

// CloneCredentialProvider fetches credentials immediately before checkout.
type CloneCredentialProvider interface {
	GetCloneCredential(ctx context.Context, deploymentID string) (CloneCredential, error)
}
