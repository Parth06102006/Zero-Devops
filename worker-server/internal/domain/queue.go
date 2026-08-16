// Package domain defines core domain types and interfaces for the worker server.
package domain

import (
	"Zero_Devops/worker_server/internal/deployments/contract"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQ holds the connection and channel for RabbitMQ messaging.
type RabbitMQ struct {
	Conn    *amqp.Connection
	Channel *amqp.Channel
}

// DeployJob represents a deployment job to be processed by the worker.
type DeployJob struct {
	DeploymentID string `json:"deployment_id"`
	CloneURL     string `json:"clone_url"`
	CommitSHA    string `json:"commit_sha"`
	RetryCount   int    `json:"retry_count"`
	RequestID    string `json:"request_id"`
}

// DeployStatusMessage represents a status update for a deployment.
type DeployStatusMessage struct {
	DeploymentID string `json:"deployment_id"`
	Status       string `json:"status"`
	OutputURL    string `json:"output_url,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// QueueUsecase defines the interface for queue operations.
type QueueUsecase interface {
	Close()
	Channel() *amqp.Channel
	SetUpQueues() error
	PublishJob(job DeployJob) error
	PublishBuildRequest(request contract.BuildRequestV1) error
	PublishStatusUpdate(status DeployStatusMessage) error
}
