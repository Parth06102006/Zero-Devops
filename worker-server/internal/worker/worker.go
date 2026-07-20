// Package worker implements the deployment worker that processes jobs from RabbitMQ.
package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"Zero_Devops/worker_server/internal/deployments"
	"Zero_Devops/worker_server/internal/domain"

	appMiddleware "Zero_Devops/worker_server/internal/middleware"

	"github.com/google/uuid"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type workerUsecase struct {
	queueClient      domain.QueueUsecase
	repo             domain.DeploymentRepository
	artifactUploader domain.UploadUsecase
}

// NewWorkerUsecase creates a new WorkerUsecase with the given dependencies.
func NewWorkerUsecase(queueClient domain.QueueUsecase, repo domain.DeploymentRepository, artifactUploader domain.UploadUsecase) domain.WorkerUsecase {
	return &workerUsecase{
		queueClient:      queueClient,
		repo:             repo,
		artifactUploader: artifactUploader,
	}
}

// StartWorker starts consuming deploy jobs from RabbitMQ and processing them.
func (w *workerUsecase) StartWorker(baseLogger *zap.Logger) error {
	r := w.queueClient.Channel()

	err := r.Qos(
		1,
		0,
		false,
	)

	if err != nil {
		baseLogger.Error("error creating worker", zap.Error(err))
		return err
	}

	msgs, err := r.Consume(
		"deploy.jobs",
		"",    // consumer tag (auto-generated)
		false, // autoAck is false, so we ack manually.
		false, false, false, nil,
	)
	if err != nil {
		return err
	}

	baseLogger.Info("Worker consumer registered. Listening for 'deploy.jobs' messages on RabbitMQ...")

	for msg := range msgs {
		var job domain.DeployJob

		if err := json.Unmarshal(msg.Body, &job); err != nil {
			baseLogger.Error("failed to decode deploy job", zap.Error(err))
			if nackErr := msg.Nack(false, false); nackErr != nil {
				baseLogger.Error("failed to nack message", zap.Error(nackErr))
			}
			continue
		}

		reqID := job.RequestID
		if reqID == "" {
			reqID = uuid.NewString() // fallback if somehow missing/came from an untraced source
		}

		logger := baseLogger.With(zap.String("request_id", reqID))
		ctx := appMiddleware.WithLogger(context.Background(), logger)

		logger.Info("received deploy job message", zap.Uint64("delivery_tag", msg.DeliveryTag))

		err := deployments.ProcessDeployment(ctx, w.repo, job, w.artifactUploader, w.queueClient, job.RetryCount, logger)

		maxRetriesCount := viper.GetInt("MAX_RETRIES_COUNT")

		if maxRetriesCount == 0 {
			maxRetriesCount = 3
		}

		if err != nil {
			logger.Error("deployment job failed", zap.String("deployment_id", job.DeploymentID), zap.Error(err))
			job.RetryCount++
			if job.RetryCount >= maxRetriesCount {
				errMsg := fmt.Sprintf("max retries (%d) exceeded: %s", maxRetriesCount, err.Error())
				if err := w.repo.MarkCanceled(ctx, job.DeploymentID, errMsg); err != nil {
					logger.Error("failed to mark deployment as canceled", zap.Error(err))
				}
				if pubErr := w.queueClient.PublishStatusUpdate(domain.DeployStatusMessage{
					DeploymentID: job.DeploymentID,
					Status:       "canceled",
					ErrorMessage: errMsg,
				}); pubErr != nil {
					logger.Error("failed to publish canceled status", zap.Error(pubErr))
				}
				if nackErr := msg.Nack(false, false); nackErr != nil {
					logger.Error("failed to nack message", zap.Error(nackErr))
				}
			} else {
				if err := w.queueClient.PublishJob(job); err != nil {
					if nackErr := msg.Nack(false, true); nackErr != nil {
						logger.Error("failed to nack message", zap.Error(nackErr))
					}
					continue
				}
				if ackErr := msg.Ack(false); ackErr != nil {
					logger.Error("failed to ack message", zap.Error(ackErr))
				}
			}
			continue
		}

		logger.Info("deployment job completed", zap.String("deployment_id", job.DeploymentID))
		if ackErr := msg.Ack(false); ackErr != nil {
			logger.Error("failed to ack message", zap.Error(ackErr))
		}
	}

	baseLogger.Info("Worker consumer delivery channel closed")
	return nil
}
