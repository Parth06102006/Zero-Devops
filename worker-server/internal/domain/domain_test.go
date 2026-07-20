package domain

import (
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

func TestQueueUsecaseInterface(_ *testing.T) {
	var _ QueueUsecase = (*queueUsecaseMock)(nil)
}

type queueUsecaseMock struct{}

func (m *queueUsecaseMock) Close()                                          {}
func (m *queueUsecaseMock) Channel() *amqp.Channel                          { return nil }
func (m *queueUsecaseMock) SetUpQueues() error                              { return nil }
func (m *queueUsecaseMock) PublishJob(_ DeployJob) error                    { return nil }
func (m *queueUsecaseMock) PublishStatusUpdate(_ DeployStatusMessage) error { return nil }

func TestUploadUsecaseInterface(_ *testing.T) {
	var _ UploadUsecase = (*uploadUsecaseMock)(nil)
}

type uploadUsecaseMock struct{}

func (m *uploadUsecaseMock) UploadImage(_ string) (string, error) { return "", nil }

func TestWorkerUsecaseInterface(_ *testing.T) {
	var _ WorkerUsecase = (*workerUsecaseMock)(nil)
}

type workerUsecaseMock struct{}

func (m *workerUsecaseMock) StartWorker(_ *zap.Logger) error { return nil }

func TestDeployJobStruct(t *testing.T) {
	job := DeployJob{
		DeploymentID: "42",
		CloneURL:     "https://github.com/user/repo.git",
		RetryCount:   1,
		RequestID:    "req-123",
	}
	if job.DeploymentID != "42" {
		t.Errorf("expected DeploymentID 42, got %s", job.DeploymentID)
	}
	if job.CloneURL != "https://github.com/user/repo.git" {
		t.Errorf("expected CloneURL 'https://github.com/user/repo.git', got '%s'", job.CloneURL)
	}
	if job.RetryCount != 1 {
		t.Errorf("expected RetryCount 1, got %d", job.RetryCount)
	}
	if job.RequestID != "req-123" {
		t.Errorf("expected RequestID 'req-123', got '%s'", job.RequestID)
	}
}

func TestDeployStatusMessageStruct(t *testing.T) {
	msg := DeployStatusMessage{
		DeploymentID: "7",
		Status:       "done",
	}
	if msg.DeploymentID != "7" {
		t.Errorf("expected DeploymentID 7, got %s", msg.DeploymentID)
	}
	if msg.Status != "done" {
		t.Errorf("expected Status 'done', got '%s'", msg.Status)
	}
}

func TestRabbitMQStruct(t *testing.T) {
	conn := &amqp.Connection{}
	ch := &amqp.Channel{}
	rmq := RabbitMQ{Conn: conn, Channel: ch}
	if rmq.Conn != conn {
		t.Error("Conn was not stored")
	}
	if rmq.Channel != ch {
		t.Error("Channel was not stored")
	}
}

func TestUploadClientStruct(t *testing.T) {
	uc := UploadClient{
		BucketName:    "my-bucket",
		PublicBaseURL: "https://cdn.example.com",
	}
	if uc.BucketName != "my-bucket" {
		t.Errorf("expected BucketName 'my-bucket', got '%s'", uc.BucketName)
	}
	if uc.PublicBaseURL != "https://cdn.example.com" {
		t.Errorf("expected PublicBaseURL 'https://cdn.example.com', got '%s'", uc.PublicBaseURL)
	}
}
