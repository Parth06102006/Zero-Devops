// Package queue handles RabbitMQ queue setup and configuration
package queue

import amqp "github.com/rabbitmq/amqp091-go"

func SetUpQueues(conn *amqp.Connection, queueChannel *amqp.Channel) error {
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
			return err
		}

		err = queueChannel.QueueBind("deploy.jobs.dlq", "deploy.jobs.dlq", "deploy.dlx", false, nil)
		if err != nil {
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
			return err
		}

		err = queueChannel.QueueBind("deploy.status.dlq", "deploy.status.dlq", "deploy.dlx", false, nil)
		if err != nil {
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
			return err
		}
	}

	return nil
}