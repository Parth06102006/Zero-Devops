package usecase

import (
	"Zero_Devops/server/internal/domain"
	"context"
	"testing"
	"time"
)

type deploymentRepoMock struct {
	storeProjectBuildOutboxFn func(ctx context.Context, params domain.StoreProjectBuildWithOutboxParams) (*domain.Deployment, error)
	getUserFn                 func(ctx context.Context, userID string) ([]domain.Deployment, error)
	getIDFn                   func(ctx context.Context, userID, id string) (*domain.Deployment, error)
	getProjectFn              func(ctx context.Context, userID, projectID string) ([]domain.Deployment, error)
	applyStatusUpdateFn       func(ctx context.Context, params domain.ApplyStatusParams) (*domain.ApplyStatusResult, error)
}

func (m *deploymentRepoMock) StoreWebhookBuildWithOutbox(_ context.Context, _ domain.StoreWebhookBuildParams) (*domain.Deployment, error) {
	return nil, nil
}

func (m *deploymentRepoMock) GetByUserID(ctx context.Context, userID string) ([]domain.Deployment, error) {
	if m.getUserFn != nil {
		return m.getUserFn(ctx, userID)
	}
	return nil, nil
}

func (m *deploymentRepoMock) GetByID(ctx context.Context, userID, id string) (*domain.Deployment, error) {
	if m.getIDFn != nil {
		return m.getIDFn(ctx, userID, id)
	}
	return nil, nil
}

func (m *deploymentRepoMock) GetByProjectID(ctx context.Context, userID, projectID string) ([]domain.Deployment, error) {
	if m.getProjectFn != nil {
		return m.getProjectFn(ctx, userID, projectID)
	}
	return nil, nil
}

func (m *deploymentRepoMock) StoreProjectBuildWithOutbox(ctx context.Context, params domain.StoreProjectBuildWithOutboxParams) (*domain.Deployment, error) {
	if m.storeProjectBuildOutboxFn != nil {
		return m.storeProjectBuildOutboxFn(ctx, params)
	}
	return nil, nil
}

func (m *deploymentRepoMock) ClaimOutboxBatch(_ context.Context, _ int) ([]domain.OutboxEvent, error) {
	return nil, nil
}

func (m *deploymentRepoMock) MarkOutboxSent(_ context.Context, _ string) error {
	return nil
}

func (m *deploymentRepoMock) MarkOutboxPublishFailed(_ context.Context, _ string, _ string, _ time.Time) error {
	return nil
}

func (m *deploymentRepoMock) ResetStuckPublishing(_ context.Context, _ time.Duration) (int64, error) {
	return 0, nil
}

func (m *deploymentRepoMock) ApplyStatusUpdate(ctx context.Context, params domain.ApplyStatusParams) (*domain.ApplyStatusResult, error) {
	if m.applyStatusUpdateFn != nil {
		return m.applyStatusUpdateFn(ctx, params)
	}
	return nil, nil
}

func (m *deploymentRepoMock) DeleteSentOutboxOlderThan(_ context.Context, _ time.Duration) (int64, error) {
	return 0, nil
}

type githubRepoMock struct {
	getInstFn func(ctx context.Context, userID string) (*domain.GithubInstallation, error)
}

type installationTokenProviderMock struct {
	createFn func(ctx context.Context, installationID int64) (string, error)
}

func (m *installationTokenProviderMock) CreateInstallationToken(ctx context.Context, installationID int64) (string, error) {
	if m.createFn != nil {
		return m.createFn(ctx, installationID)
	}
	return "test-token", nil
}

func (m *githubRepoMock) StoreInstallation(_ context.Context, _ *domain.GithubInstallation) error {
	return nil
}
func (m *githubRepoMock) GetInstallationByUserID(ctx context.Context, userID string) (*domain.GithubInstallation, error) {
	if m.getInstFn != nil {
		return m.getInstFn(ctx, userID)
	}
	return nil, nil
}
func (m *githubRepoMock) DeleteInstallationByUserID(_ context.Context, _ string) error {
	return nil
}
func (m *githubRepoMock) UpdateInstallationStatus(_ context.Context, _, _ string) error {
	return nil
}
func (m *githubRepoMock) UpdateInstallationStatusByGithubInstallationID(_ context.Context, _ int64, _ string) error {
	return nil
}
func (m *githubRepoMock) GetInstallationIdByGithubInstallationID(_ context.Context, _ int64) (string, error) {
	return "", nil
}
func (m *githubRepoMock) UpdateInstallationExternalIDByID(_ context.Context, _, _ string) error {
	return nil
}

func (m *githubRepoMock) GetInstallationStatusByID(_ context.Context, _ string) (string, error) {
	return domain.GithubInstallationStatusActive, nil
}

func TestGetDeployments_PassesThrough(t *testing.T) {
	want := []domain.Deployment{{ID: "1", UserID: "2"}}
	uc := NewDeploymentUsecase(context.Background(), &deploymentRepoMock{
		getUserFn: func(_ context.Context, userID string) ([]domain.Deployment, error) {
			if userID != "2" {
				t.Fatalf("unexpected userID %s", userID)
			}
			return want, nil
		},
	}, &githubRepoMock{}, &installationTokenProviderMock{}, nil)

	got, err := uc.GetDeployments(context.Background(), "2")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("unexpected deployments: %+v", got)
	}
}

func TestGetDeploymentByID_PassesThrough(t *testing.T) {
	want := &domain.Deployment{ID: "9"}
	uc := NewDeploymentUsecase(context.Background(), &deploymentRepoMock{
		getIDFn: func(_ context.Context, userID, id string) (*domain.Deployment, error) {
			if userID != "2" || id != "9" {
				t.Fatalf("unexpected args userID=%s id=%s", userID, id)
			}
			return want, nil
		},
	}, &githubRepoMock{}, &installationTokenProviderMock{}, nil)

	got, err := uc.GetDeploymentByID(context.Background(), "2", "9")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got.ID != "9" {
		t.Fatalf("unexpected deployment: %+v", got)
	}
}

func TestListProjectBuilds_EmptyProjectID(t *testing.T) {
	uc := NewDeploymentUsecase(context.Background(), &deploymentRepoMock{}, &githubRepoMock{}, &installationTokenProviderMock{}, nil)
	_, err := uc.ListProjectBuilds(context.Background(), "2", "  ")
	if err != domain.ErrBadParamInput {
		t.Fatalf("expected ErrBadParamInput, got %v", err)
	}
}

func TestListProjectBuilds_PassesThrough(t *testing.T) {
	want := []domain.Deployment{{ID: "1", UserID: "2", ProjectID: "p1"}}
	uc := NewDeploymentUsecase(context.Background(), &deploymentRepoMock{
		getProjectFn: func(_ context.Context, userID, projectID string) ([]domain.Deployment, error) {
			if userID != "2" || projectID != "p1" {
				t.Fatalf("unexpected args userID=%s projectID=%s", userID, projectID)
			}
			return want, nil
		},
	}, &githubRepoMock{}, &installationTokenProviderMock{}, nil)

	got, err := uc.ListProjectBuilds(context.Background(), "2", "p1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("unexpected deployments: %+v", got)
	}
}

func TestGetBuild_EmptyBuildID(t *testing.T) {
	uc := NewDeploymentUsecase(context.Background(), &deploymentRepoMock{}, &githubRepoMock{}, &installationTokenProviderMock{}, nil)
	_, err := uc.GetBuild(context.Background(), "2", " ")
	if err != domain.ErrBadParamInput {
		t.Fatalf("expected ErrBadParamInput, got %v", err)
	}
}

func TestGetBuild_PassesThrough(t *testing.T) {
	want := &domain.Deployment{ID: "9", UserID: "2"}
	uc := NewDeploymentUsecase(context.Background(), &deploymentRepoMock{
		getIDFn: func(_ context.Context, userID, id string) (*domain.Deployment, error) {
			if userID != "2" || id != "9" {
				t.Fatalf("unexpected args userID=%s id=%s", userID, id)
			}
			return want, nil
		},
	}, &githubRepoMock{}, &installationTokenProviderMock{}, nil)

	got, err := uc.GetBuild(context.Background(), "2", "9")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got.ID != "9" {
		t.Fatalf("unexpected deployment: %+v", got)
	}
}
