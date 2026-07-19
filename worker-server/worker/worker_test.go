package worker

import (
	"database/sql"
	"testing"

	"Zero_Devops/worker_server/domain"

	amqp "github.com/rabbitmq/amqp091-go"
)

type fakeQueueUsecase struct {
	channel *amqp.Channel
}

func (f *fakeQueueUsecase) Close() {
}

func (f *fakeQueueUsecase) Channel() *amqp.Channel {
	return f.channel
}

func (f *fakeQueueUsecase) SetUpQueues() error {
	return nil
}

func (f *fakeQueueUsecase) PublishJob(_ domain.DeployJob) error {
	return nil
}

func (f *fakeQueueUsecase) PublishStatusUpdate(_ domain.DeployStatusMessage) error {
	return nil
}

type fakeUploadUsecase struct {
	url string
}

func (f *fakeUploadUsecase) UploadImage(_ string) (string, error) {
	return f.url, nil
}

func TestNewWorkerUsecaseStoresDependencies(t *testing.T) {
	queueClient := &fakeQueueUsecase{channel: &amqp.Channel{}}
	db := &sql.DB{}
	uploader := &fakeUploadUsecase{url: "s3://bucket/image.tar"}

	usecase := NewWorkerUsecase(queueClient, db, uploader)

	workerClient, ok := usecase.(*workerUsecase)
	if !ok {
		t.Fatalf("NewWorkerUsecase returned %T, want *workerUsecase", usecase)
	}
	if workerClient.queueClient != queueClient {
		t.Fatal("queue dependency was not stored")
	}
	if workerClient.db != db {
		t.Fatal("db dependency was not stored")
	}
	if workerClient.artifactUploader != uploader {
		t.Fatal("upload dependency was not stored")
	}
}

func TestWorkerUsecaseImplementsDomainInterface(_ *testing.T) {
	var _ domain.WorkerUsecase = (*workerUsecase)(nil)
}
