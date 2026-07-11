package worker

import (
	"build_app_test/deployments"
	"build_app_test/worker/queue"
	"context"
	"database/sql"
	"encoding/json"
	"log"

	"build_app_test/upload"
)

func StartWorker(rmq *queue.RabbitMQ, db *sql.DB, artifactUploader upload.Uploader) error {
	r := rmq.Channel()

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
		false, // autoAck — FALSE, we ack manually
		false, false, false, nil,
	)
	if err != nil {
		return err
	}

	for msg := range msgs {
		var job queue.DeployJob

		if err := json.Unmarshal(msg.Body,&job); err != nil{
			msg.Nack(false,false)
			continue;
		}

		err := deployments.ProcessDeployment(context.Background(), db, job, artifactUploader)

		if err != nil{
			job.RetryCount++
			if job.RetryCount >= 3 {
				msg.Nack(false,false)
			}else{
				rmq.PublishJob(job)
				msg.Ack(false)
			}
			continue
		}

		msg.Ack(false)
	}

	return nil
}
