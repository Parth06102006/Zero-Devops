// Package queue handles RabbitMQ queue setup and configuration
package queue

import amqp "github.com/rabbitmq/amqp091-go"

func exchangeExists(conn *amqp.Connection, name, kind string) (bool, error) {
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

func queueExists(conn *amqp.Connection, name string, args amqp.Table) (bool, error) {
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

func declareExchange(ch *amqp.Channel, name, kind string) error {
	return ch.ExchangeDeclare(
		name,
		kind,
		true,
		false,
		false,
		false,
		nil,
	)
}

func declareQueue(ch *amqp.Channel, name string, args amqp.Table) error {
	_, err := ch.QueueDeclare(
		name,
		true,
		false,
		false,
		false,
		args,
	)
	return err
}

func bindQueue(ch *amqp.Channel, name, routingKey, exchange string) error {
	return ch.QueueBind(name, routingKey, exchange, false, nil)
}

// SetUpQueues initializes and configures the message queues
func SetUpQueues(conn *amqp.Connection, queueChannel *amqp.Channel) error {
	exists, err := exchangeExists(conn, "deploy.dlx", "direct")
	if err != nil {
		return err
	}
	if !exists {
		if err := declareExchange(queueChannel, "deploy.dlx", "direct"); err != nil {
			return err
		}
	}

	exists, err = queueExists(conn, "deploy.jobs.dlq", nil)
	if err != nil {
		return err
	}
	if !exists {
		if err := declareQueue(queueChannel, "deploy.jobs.dlq", nil); err != nil {
			return err
		}
		if err := bindQueue(queueChannel, "deploy.jobs.dlq", "deploy.jobs.dlq", "deploy.dlx"); err != nil {
			return err
		}
	}

	argsJobs := amqp.Table{
		"x-dead-letter-exchange":    "deploy.dlx",
		"x-dead-letter-routing-key": "deploy.jobs.dlq",
	}

	exists, err = queueExists(conn, "deploy.jobs", argsJobs)
	if err != nil {
		return err
	}
	if !exists {
		if err := declareQueue(queueChannel, "deploy.jobs", argsJobs); err != nil {
			return err
		}
	}

	exists, err = queueExists(conn, "deploy.status.dlq", nil)
	if err != nil {
		return err
	}
	if !exists {
		if err := declareQueue(queueChannel, "deploy.status.dlq", nil); err != nil {
			return err
		}
		if err := bindQueue(queueChannel, "deploy.status.dlq", "deploy.status.dlq", "deploy.dlx"); err != nil {
			return err
		}
	}

	argsStatus := amqp.Table{
		"x-dead-letter-exchange":    "deploy.dlx",
		"x-dead-letter-routing-key": "deploy.status.dlq",
	}

	exists, err = queueExists(conn, "deploy.status", argsStatus)
	if err != nil {
		return err
	}
	if !exists {
		if err := declareQueue(queueChannel, "deploy.status", argsStatus); err != nil {
			return err
		}
	}

	return nil
}
