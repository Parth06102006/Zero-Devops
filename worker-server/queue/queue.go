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
	queueClient.Conn.Close()
	queueClient.Channel.Close()
}

// Channel returns the RabbitMQ channel.
func (r *queueUsecase) Channel() *amqp.Channel {
	return r.queueClient.Channel
}

// SetUpQueues declares the exchanges and queues required for the worker.
func (r *queueUsecase) SetUpQueues() error {
	conn := r.queueClient.Conn
	queueChannel := r.queueClient.Channel

	// Helper to check if exchange exists
	exchangeExists := func(name string, kind string) (bool, error) {
		ch, err := conn.Channel()
		if err != nil {
			return false, err
		}
		defer func() { _ = ch.Close() }()

		err = ch.ExchangeDeclarePassive(name, kind, true, false, false, false, nil)
		if err == nil {
			return true, nil
		}
		return false, nil
	}

	// Helper to check if queue exists
	queueExists := func(name string, args amqp.Table) (bool, error) {
		ch, err := conn.Channel()
		if err != nil {
			return false, err
		}
		defer func() { _ = ch.Close() }()

		_, err = ch.QueueDeclarePassive(name, true, false, false, false, args)
		if err == nil {
			return true, nil
		}
		return false, nil
	}

	// 1. Declare Exchange if missing
	exists, err := exchangeExists("deploy.dlx", "direct")
	if err != nil {
		return err
	}
	if !exists {
		err = queueChannel.ExchangeDeclare(
			"deploy.dlx",
			"direct",
			true,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			r.failOnError(err, "Failed to create the Exchange")
			return err
		}
	}

	// 2. Declare DEAD LETTER QUEUE FOR JOBS if missing
	exists, err = queueExists("deploy.jobs.dlq", nil)
	if err != nil {
		return err
	}
	if !exists {
		_, err = queueChannel.QueueDeclare(
			"deploy.jobs.dlq",
			true,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			r.failOnError(err, "Failed to Create DLQ")
			return err
		}

		err = queueChannel.QueueBind("deploy.jobs.dlq", "deploy.jobs.dlq", "deploy.dlx", false, nil)
		if err != nil {
			r.failOnError(err, "Failed to Bind")
			return err
		}
	}

	argsJobs := amqp.Table{
		"x-dead-letter-exchange":    "deploy.dlx",
		"x-dead-letter-routing-key": "deploy.jobs.dlq",
	}

	// 3. Declare JOBS QUEUE if missing
	exists, err = queueExists("deploy.jobs", argsJobs)
	if err != nil {
		return err
	}
	if !exists {
		_, err = queueChannel.QueueDeclare(
			"deploy.jobs",
			true,
			false,
			false,
			false,
			argsJobs,
		)
		if err != nil {
			r.failOnError(err, "Failed to declare job queue")
			return err
		}
	}

	// 4. Declare DEAD LETTER QUEUE FOR STATUS if missing
	exists, err = queueExists("deploy.status.dlq", nil)
	if err != nil {
		return err
	}
	if !exists {
		_, err = queueChannel.QueueDeclare(
			"deploy.status.dlq",
			true,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			r.failOnError(err, "Failed to Create DLQ")
			return err
		}

		err = queueChannel.QueueBind("deploy.status.dlq", "deploy.status.dlq", "deploy.dlx", false, nil)
		if err != nil {
			r.failOnError(err, "Failed to Bind")
			return err
		}
	}

	argsStatus := amqp.Table{
		"x-dead-letter-exchange":    "deploy.dlx",
		"x-dead-letter-routing-key": "deploy.status.dlq",
	}

	// 5. Declare STATUS QUEUE if missing
	exists, err = queueExists("deploy.status", argsStatus)
	if err != nil {
		return err
	}
	if !exists {
		_, err = queueChannel.QueueDeclare(
			"deploy.status",
			true,
			false,
			false,
			false,
			argsStatus,
		)
		if err != nil {
			r.failOnError(err, "Failed to declare status queue")
			return err
		}
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
