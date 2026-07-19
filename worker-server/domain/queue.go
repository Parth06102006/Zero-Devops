package domain

import amqp "github.com/rabbitmq/amqp091-go"

// RabbitMQ holds the connection and channel for RabbitMQ messaging.
type RabbitMQ struct {
	Conn    *amqp.Connection
	Channel *amqp.Channel
}

// DeployJob represents a deployment job to be processed by the worker.
type DeployJob struct {
	DeploymentID int64  `json:"deployment_id"`
	CloneURL     string `json:"clone_url"`
	RetryCount   int    `json:"retry_count"`
	RequestID    string `json:"request_id"`
}

// DeployStatusMessage represents a status update for a deployment.
type DeployStatusMessage struct {
	DeploymentID int64  `json:"deployment_id"`
	Status       string `json:"status"`
}

// QueueUsecase defines the interface for queue operations.
type QueueUsecase interface {
	Close()
	Channel() *amqp.Channel
	SetUpQueues() error
	PublishJob(job DeployJob) error
	PublishStatusUpdate(status DeployStatusMessage) error
}
