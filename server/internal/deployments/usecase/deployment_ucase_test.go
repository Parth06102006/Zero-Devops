package usecase

import (
	"Zero_Devops/server/internal/domain"
	"context"
	"testing"
)

type deploymentRepoMock struct {
	storeFn              func(ctx context.Context, d *domain.Deployment) error
	getUserFn            func(ctx context.Context, userID string) ([]domain.Deployment, error)
	getIDFn              func(ctx context.Context, userID, id string) (*domain.Deployment, error)
	updateStatusFn       func(ctx context.Context, deploymentID string, status domain.DeploymentStatus) error
	UpdateOutputURLFn    func(ctx context.Context, deploymentID string, outputURL string) error
	UpdateErrorMessageFn func(ctx context.Context, deploymentID string, errMsg string) error
}

func (m *deploymentRepoMock) Store(ctx context.Context, d *domain.Deployment) error {
	if m.storeFn != nil {
		return m.storeFn(ctx, d)
	}
	return nil
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

func (m *deploymentRepoMock) UpdateStatus(ctx context.Context, deploymentID string, status domain.DeploymentStatus) error {
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, deploymentID, status)
	}
	return nil
}

func (m *deploymentRepoMock) UpdateOutputURL(ctx context.Context, deploymentID, outputURL string) error {
	if m.UpdateOutputURLFn != nil {
		return m.UpdateOutputURLFn(ctx, deploymentID, outputURL)
	}
	return nil
}

func (m *deploymentRepoMock) UpdateErrorMessage(ctx context.Context, deploymentID, errMsg string) error {
	if m.UpdateErrorMessageFn != nil {
		return m.UpdateErrorMessageFn(ctx, deploymentID, errMsg)
	}
	return nil
}

type githubRepoMock struct {
	getInstFn func(ctx context.Context, userID string) (*domain.GithubInstallation, error)
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

func TestGetDeployments_PassesThrough(t *testing.T) {
	want := []domain.Deployment{{ID: "1", UserID: "2"}}
	uc := NewDeploymentUsecase(&deploymentRepoMock{
		getUserFn: func(_ context.Context, userID string) ([]domain.Deployment, error) {
			if userID != "2" {
				t.Fatalf("unexpected userID %s", userID)
			}
			return want, nil
		},
	}, &githubRepoMock{}, nil)

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
	uc := NewDeploymentUsecase(&deploymentRepoMock{
		getIDFn: func(_ context.Context, userID, id string) (*domain.Deployment, error) {
			if userID != "2" || id != "9" {
				t.Fatalf("unexpected args userID=%s id=%s", userID, id)
			}
			return want, nil
		},
	}, &githubRepoMock{}, nil)

	got, err := uc.GetDeploymentByID(context.Background(), "2", "9")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got.ID != "9" {
		t.Fatalf("unexpected deployment: %+v", got)
	}
}
