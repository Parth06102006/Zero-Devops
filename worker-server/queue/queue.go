package queue

import (
	"encoding/json"
	"log"
	"Zero_Devops/worker_server/domain"
	amqp "github.com/rabbitmq/amqp091-go"
)

func failOnError(err error , msg string){
	if err != nil{
		log.Panicf("%s : %s",msg,err)
	}
}

type queueUsecase struct {
	queueClient	*domain.RabbitMQ
}

func NewQueueUsecase(conn *amqp.Connection , channel *amqp.Channel) domain.QueueUsecase{
	return &queueUsecase{
		queueClient: &domain.RabbitMQ{
			Conn: conn,
			Channel: channel,
		},
	}
}

func (r *queueUsecase) Close(){
	queueClient := r.queueClient
	queueClient.Conn.Close()
	queueClient.Channel.Close()
}

func (r *queueUsecase) Channel() *amqp.Channel {
	return r.queueClient.Channel
}

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
			failOnError(err, "Failed to create the Exchange")
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
			failOnError(err, "Failed to Create DLQ")
			return err
		}

		err = queueChannel.QueueBind("deploy.jobs.dlq", "deploy.jobs.dlq", "deploy.dlx", false, nil)
		if err != nil {
			failOnError(err, "Failed to Bind")
			return err
		}
	}

	args_jobs := amqp.Table{
		"x-dead-letter-exchange":    "deploy.dlx",
		"x-dead-letter-routing-key": "deploy.jobs.dlq",
	}

	// 3. Declare JOBS QUEUE if missing
	exists, err = queueExists("deploy.jobs", args_jobs)
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
			args_jobs,
		)
		if err != nil {
			failOnError(err, "Failed to declare job queue")
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
			failOnError(err, "Failed to Create DLQ")
			return err
		}

		err = queueChannel.QueueBind("deploy.status.dlq", "deploy.status.dlq", "deploy.dlx", false, nil)
		if err != nil {
			failOnError(err, "Failed to Bind")
			return err
		}
	}

	args_status := amqp.Table{
		"x-dead-letter-exchange":    "deploy.dlx",
		"x-dead-letter-routing-key": "deploy.status.dlq",
	}

	// 5. Declare STATUS QUEUE if missing
	exists, err = queueExists("deploy.status", args_status)
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
			args_status,
		)
		if err != nil {
			failOnError(err, "Failed to declare status queue")
			return err
		}
	}

	return nil
}

func (r* queueUsecase) PublishJob(job domain.DeployJob) error{
	body, err := json.Marshal(job)

	if err != nil{
		failOnError(err,"Failed to Receive Jobs")
		return err
	}

	return r.queueClient.Channel.Publish(
		"",
		"deploy.jobs",
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			DeliveryMode: amqp.Persistent,
			Body: body,
		},
	)
}

func (r* queueUsecase) PublishStatusUpdate(status domain.DeployStatusMessage) error{
	body, err := json.Marshal(status)

	if err != nil{
		failOnError(err,"Failed to Publish Status")
		return err
	}

	return r.queueClient.Channel.Publish(
		"",
		"deploy.status",
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			DeliveryMode: amqp.Persistent,
			Body: body,
		},
	)
}


