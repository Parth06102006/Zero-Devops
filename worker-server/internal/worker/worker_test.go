package worker

import (
	"context"
	"testing"

	"Zero_Devops/worker_server/internal/domain"

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

type fakeDeploymentRepo struct{}

func (f *fakeDeploymentRepo) Insert(_ context.Context, _ domain.DeployJob) error {
	return nil
}
func (f *fakeDeploymentRepo) UpdateStatus(_ context.Context, _, _ string, _ int) error {
	return nil
}
func (f *fakeDeploymentRepo) UpdateOutputURL(_ context.Context, _, _ string) error {
	return nil
}
func (f *fakeDeploymentRepo) ReadImageTag(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (f *fakeDeploymentRepo) MarkBuilding(_ context.Context, _ string) error {
	return nil
}
func (f *fakeDeploymentRepo) MarkFailed(_ context.Context, _, _ string) error {
	return nil
}
func (f *fakeDeploymentRepo) MarkCanceled(_ context.Context, _, _ string) error {
	return nil
}
func (f *fakeDeploymentRepo) MarkFinished(_ context.Context, _, _ string) error {
	return nil
}

func TestNewWorkerUsecaseStoresDependencies(t *testing.T) {
	queueClient := &fakeQueueUsecase{channel: &amqp.Channel{}}
	repo := &fakeDeploymentRepo{}
	uploader := &fakeUploadUsecase{url: "s3://bucket/image.tar"}

	usecase := NewWorkerUsecase(queueClient, repo, uploader)

	workerClient, ok := usecase.(*workerUsecase)
	if !ok {
		t.Fatalf("NewWorkerUsecase returned %T, want *workerUsecase", usecase)
	}
	if workerClient.queueClient != queueClient {
		t.Fatal("queue dependency was not stored")
	}
	if workerClient.repo != repo {
		t.Fatal("repo dependency was not stored")
	}
	if workerClient.artifactUploader != uploader {
		t.Fatal("upload dependency was not stored")
	}
}

func TestWorkerUsecaseImplementsDomainInterface(_ *testing.T) {
	var _ domain.WorkerUsecase = (*workerUsecase)(nil)
}
