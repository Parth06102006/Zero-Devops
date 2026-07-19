package queue

import (
	"testing"

	"Zero_Devops/worker_server/domain"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

func TestNewQueueUsecaseStoresConnectionAndChannel(t *testing.T) {
	conn := &amqp.Connection{}
	ch := &amqp.Channel{}

	usecase := NewQueueUsecase(zap.NewNop(), conn, ch)

	queueClient, ok := usecase.(*queueUsecase)
	if !ok {
		t.Fatalf("NewQueueUsecase returned %T, want *queueUsecase", usecase)
	}
	if queueClient.queueClient.Conn != conn {
		t.Fatal("queue connection was not stored")
	}
	if queueClient.Channel() != ch {
		t.Fatal("queue channel was not stored")
	}
}

func TestQueueUsecaseImplementsDomainInterface(_ *testing.T) {
	var _ domain.QueueUsecase = (*queueUsecase)(nil)
}
