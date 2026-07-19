// Package worker implements the deployment worker that processes jobs from RabbitMQ.
package worker

import (
	"context"
	"database/sql"
	"encoding/json"

	"Zero_Devops/worker_server/deployments"
	"Zero_Devops/worker_server/domain"

	appMiddleware "Zero_Devops/worker_server/middleware"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type workerUsecase struct {
	queueClient      domain.QueueUsecase
	db               *sql.DB
	artifactUploader domain.UploadUsecase
}

// NewWorkerUsecase creates a new WorkerUsecase with the given dependencies.
func NewWorkerUsecase(queueClient domain.QueueUsecase, db *sql.DB, artifactUploader domain.UploadUsecase) domain.WorkerUsecase {
	return &workerUsecase{
		queueClient:      queueClient,
		db:               db,
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
			msg.Nack(false, false)
			continue
		}

		reqID := job.RequestID
		if reqID == "" {
			reqID = uuid.NewString() // fallback if somehow missing/came from an untraced source
		}

		logger := baseLogger.With(zap.String("request_id", reqID))
		ctx := appMiddleware.WithLogger(context.Background(), logger)

		logger.Info("received deploy job message", zap.Uint64("delivery_tag", msg.DeliveryTag))

		err := deployments.ProcessDeployment(ctx, w.db, job, w.artifactUploader, w.queueClient, job.RetryCount, logger)

		maxRetriesCount := viper.GetInt("MAX_RETRIES_COUNT")

		if maxRetriesCount == 0 {
			maxRetriesCount = 3
		}

		if err != nil {
			logger.Error("deployment job failed", zap.Int64("deployment_id", job.DeploymentID), zap.Error(err))
			job.RetryCount++
			if job.RetryCount >= maxRetriesCount {
				msg.Nack(false, false)
			} else {
				if err := w.queueClient.PublishJob(job); err != nil {
					msg.Nack(false, true)
					continue
				}
				msg.Ack(false)
			}
			continue
		}

		logger.Info("deployment job completed", zap.Int64("deployment_id", job.DeploymentID))
		msg.Ack(false)
	}

	baseLogger.Info("Worker consumer delivery channel closed")
	return nil
}
