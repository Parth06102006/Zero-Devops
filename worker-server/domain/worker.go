package domain

import "go.uber.org/zap"

// WorkerUsecase defines the interface for the deployment worker.
type WorkerUsecase interface {
	StartWorker(baseLogger *zap.Logger) error
}
