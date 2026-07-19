// Package queue provides RabbitMQ queue operations for the worker server.
package queue

import (
	"encoding/json"

	"Zero_Devops/worker_server/domain"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

type queueUsecase struct {
	queueClient *domain.RabbitMQ
	logger      *zap.Logger
}

// NewQueueUsecase creates a new QueueUsecase with the given logger, connection, and channel.
func NewQueueUsecase(logger *zap.Logger, conn *amqp.Connection, channel *amqp.Channel) domain.QueueUsecase {
	return &queueUsecase{
		queueClient: &domain.RabbitMQ{
			Conn:    conn,
			Channel: channel,
		},
		logger: logger,
	}
}

func (r *queueUsecase) failOnError(err error, msg string) {
	if err != nil {
		r.logger.Panic(msg, zap.Error(err))
	}
}

// Close closes the RabbitMQ connection and channel.
func (r *queueUsecase) Close() {
	queueClient := r.queueClient
	if cerr := queueClient.Conn.Close(); cerr != nil {
		r.logger.Error("failed to close rabbitmq connection", zap.Error(cerr))
	}
	if cerr := queueClient.Channel.Close(); cerr != nil {
		r.logger.Error("failed to close rabbitmq channel", zap.Error(cerr))
	}
}

// Channel returns the RabbitMQ channel.
func (r *queueUsecase) Channel() *amqp.Channel {
	return r.queueClient.Channel
}

// SetUpQueues declares the exchanges and queues required for the worker.
func (r *queueUsecase) ensureExchange(name, kind string) error {
	ch, err := r.queueClient.Conn.Channel()
	if err != nil {
		return err
	}
	defer func() { _ = ch.Close() }()

	err = ch.ExchangeDeclarePassive(name, kind, true, false, false, false, nil)
	if err == nil {
		return nil
	}

	return r.queueClient.Channel.ExchangeDeclare(
		name, kind, true, false, false, false, nil,
	)
}

func (r *queueUsecase) ensureQueue(name string, args amqp.Table) error {
	ch, err := r.queueClient.Conn.Channel()
	if err != nil {
		return err
	}
	defer func() { _ = ch.Close() }()

	_, err = ch.QueueDeclarePassive(name, true, false, false, false, args)
	if err == nil {
		return nil
	}

	_, err = r.queueClient.Channel.QueueDeclare(name, true, false, false, false, args)
	return err
}

func (r *queueUsecase) ensureQueueWithDLQ(name, dlqName, _ string, args amqp.Table) error {
	if err := r.ensureQueue(dlqName, nil); err != nil {
		return err
	}

	if err := r.queueClient.Channel.QueueBind(dlqName, dlqName, "deploy.dlx", false, nil); err != nil {
		return err
	}

	return r.ensureQueue(name, args)
}

func (r *queueUsecase) SetUpQueues() error {
	if err := r.ensureExchange("deploy.dlx", "direct"); err != nil {
		r.failOnError(err, "Failed to create the Exchange")
		return err
	}

	argsJobs := amqp.Table{
		"x-dead-letter-exchange":    "deploy.dlx",
		"x-dead-letter-routing-key": "deploy.jobs.dlq",
	}
	if err := r.ensureQueueWithDLQ("deploy.jobs", "deploy.jobs.dlq", "deploy.jobs.dlq", argsJobs); err != nil {
		r.failOnError(err, "Failed to set up job queue")
		return err
	}

	argsStatus := amqp.Table{
		"x-dead-letter-exchange":    "deploy.dlx",
		"x-dead-letter-routing-key": "deploy.status.dlq",
	}
	if err := r.ensureQueueWithDLQ("deploy.status", "deploy.status.dlq", "deploy.status.dlq", argsStatus); err != nil {
		r.failOnError(err, "Failed to set up status queue")
		return err
	}

	return nil
}

// PublishJob publishes a deploy job to the queue.
func (r *queueUsecase) PublishJob(job domain.DeployJob) error {
	body, err := json.Marshal(job)

	if err != nil {
		r.failOnError(err, "Failed to Receive Jobs")
		return err
	}

	return r.queueClient.Channel.Publish(
		"",
		"deploy.jobs",
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
}

// PublishStatusUpdate publishes a deployment status message to the queue.
func (r *queueUsecase) PublishStatusUpdate(status domain.DeployStatusMessage) error {
	body, err := json.Marshal(status)

	if err != nil {
		r.failOnError(err, "Failed to Publish Status")
		return err
	}

	return r.queueClient.Channel.Publish(
		"",
		"deploy.status",
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
}
