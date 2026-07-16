package domain

import amqp "github.com/rabbitmq/amqp091-go"

type RabbitMQ struct {
	Conn    *amqp.Connection
	Channel *amqp.Channel
}

type DeployJob struct {
	DeploymentID int64  `json:"deployment_id"`
	Clone_URL    string `json:"clone_url"`
	RetryCount   int    `json:"retry_count"`
}

type DeployStatusMessage struct {
	DeploymentID int64  `json:"deployment_id"`
	Status       string `json:"status"`
}

type QueueUsecase interface {
	Close()
	Channel() *amqp.Channel
	SetUpQueues() error
	PublishJob(job DeployJob) error
	PublishStatusUpdate(status DeployStatusMessage) error
}
