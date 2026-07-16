package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"

	"Zero_Devops/worker_server/deployments"
	"Zero_Devops/worker_server/domain"
)

type workerUsecase struct {
	queueClient      domain.QueueUsecase
	db               *sql.DB
	artifactUploader domain.UploadUsecase
}

func NewWorkerUsecase(queueClient domain.QueueUsecase, db *sql.DB, artifactUploader domain.UploadUsecase) domain.WorkerUsecase {
	return &workerUsecase{
		queueClient:      queueClient,
		db:               db,
		artifactUploader: artifactUploader,
	}
}

func (w *workerUsecase) StartWorker() error {
	r := w.queueClient.Channel()

	err := r.Qos(
		1,
		0,
		false,
	)

	if err != nil {
		log.Printf("Error Creating Worker: %v", err)
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

	log.Println("Worker consumer registered. Listening for 'deploy.jobs' messages on RabbitMQ...")

	for msg := range msgs {
		var job domain.DeployJob

		log.Printf("Received deploy job message: delivery_tag=%d", msg.DeliveryTag)

		if err := json.Unmarshal(msg.Body, &job); err != nil {
			log.Printf("Failed to decode deploy job: %v", err)
			msg.Nack(false, false)
			continue
		}

		err := deployments.ProcessDeployment(context.Background(), w.db, job, w.artifactUploader, w.queueClient)

		if err != nil {
			log.Printf("Deployment job %d failed: %v", job.DeploymentID, err)
			job.RetryCount++
			if job.RetryCount >= 3 {
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

		log.Printf("Deployment job %d completed", job.DeploymentID)
		msg.Ack(false)
	}

	log.Println("Worker consumer delivery channel closed")
	return nil
}
