package queue

import (
	"encoding/json"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

func failOnError(err error , msg string){
	if err != nil{
		log.Panicf("%s : %s",msg,err)
	}
}

type RabbitMQ struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

type DeployJob struct {
	DeploymentID string  `json:"id"`
	Clone_URL 	 string	 `json:"clone_url"`
	RetryCount	 int     `json:"retry_count"`
}

func NewRabbitMQ(url string) (*RabbitMQ,error){
	conn,err := amqp.Dial(url)
	if err != nil{
		failOnError(err,"Faild to connect to RabbitMQ")
		return nil,err
	}

	ch,err := conn.Channel()
	if err != nil{
		failOnError(err,"Failed to open a channel")
		return nil,err
	}

	return &RabbitMQ{conn:conn,channel:ch},nil
}

func (r *RabbitMQ) Close(){
	r.conn.Close()
	r.channel.Close()
}

func (r *RabbitMQ) Channel() *amqp.Channel {
	return r.channel
}

func (r *RabbitMQ) SetUpQueues() error{

		err := r.channel.ExchangeDeclare(		
			"deploy.dlx",
			"direct",
			true,
			false,
			false,
			false,
			nil,
		)

	if err != nil{
		failOnError(err,"Failed to create the Exchange")
		return err
	}	

	_,err = r.channel.QueueDeclare(
		"deploy.jobs.dlq",
		true,
		false,
		false,
		false,
		nil,
	)

	if err != nil{
		failOnError(err,"Failed to  Create DLQ")
		return err
	}	

	err = r.channel.QueueBind("deploy.jobs.dlq", "deploy.jobs.dlq", "deploy.dlx", false, nil)

	if err != nil{
		failOnError(err,"Failed to Bind")
		return err
	}	

	args := amqp.Table{
		"x-dead-letter-exchange":    "deploy.dlx",
		"x-dead-letter-routing-key": "deploy.jobs.dlq",
	}

	_,err = r.channel.QueueDeclare(
		"deploy.jobs",
		true,
		false,
		false,
		false,
		args,
	)

	if err != nil{
		failOnError(err,"Failed to declare a queue")
		return err
	}

	return nil

}

func (r* RabbitMQ) PublishJob(job DeployJob) error{
	body, err := json.Marshal(job)

	if err != nil{
		failOnError(err,"Failed to Receive Jobs")
		return err
	}

	return r.channel.Publish(
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

