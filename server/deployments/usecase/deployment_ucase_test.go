package usecase

import (
	"Zero_Devops/server/domain"
	"context"
	"testing"
)

type deploymentRepoMock struct {
	storeFn   func(ctx context.Context, d *domain.Deployment) error
	getUserFn func(ctx context.Context, userID int64) ([]domain.Deployment, error)
	getIDFn   func(ctx context.Context, userID, id int64) (*domain.Deployment, error)
}

func (m *deploymentRepoMock) Store(ctx context.Context, d *domain.Deployment) error {
	if m.storeFn != nil {
		return m.storeFn(ctx, d)
	}
	return nil
}

func (m *deploymentRepoMock) GetByUserID(ctx context.Context, userID int64) ([]domain.Deployment, error) {
	if m.getUserFn != nil {
		return m.getUserFn(ctx, userID)
	}
	return nil, nil
}

func (m *deploymentRepoMock) GetByID(ctx context.Context, userID, id int64) (*domain.Deployment, error) {
	if m.getIDFn != nil {
		return m.getIDFn(ctx, userID, id)
	}
	return nil, nil
}

type githubRepoMock struct {
	getInstFn func(ctx context.Context, userID int64) (*domain.GithubInstallation, error)
}

func (m *githubRepoMock) StoreInstallation(ctx context.Context, inst *domain.GithubInstallation) error { return nil }
func (m *githubRepoMock) GetInstallationByUserID(ctx context.Context, userID int64) (*domain.GithubInstallation, error) {
	if m.getInstFn != nil {
		return m.getInstFn(ctx, userID)
	}
	return nil, nil
}
func (m *githubRepoMock) DeleteInstallationByUserID(ctx context.Context, userID int64) error { return nil }
func (m *githubRepoMock) UpdateInstallationStatus(ctx context.Context, userID int64, status string) error {
	return nil
}

func TestGetDeployments_PassesThrough(t *testing.T) {
	want := []domain.Deployment{{ID: 1, UserID: 2}}
	uc := NewDeploymentUsecase(&deploymentRepoMock{
		getUserFn: func(ctx context.Context, userID int64) ([]domain.Deployment, error) {
			if userID != 2 {
				t.Fatalf("unexpected userID %d", userID)
			}
			return want, nil
		},
	}, &githubRepoMock{})

	got, err := uc.GetDeployments(context.Background(), 2)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("unexpected deployments: %+v", got)
	}
}

func TestGetDeploymentByID_PassesThrough(t *testing.T) {
	want := &domain.Deployment{ID: 9}
	uc := NewDeploymentUsecase(&deploymentRepoMock{
		getIDFn: func(ctx context.Context, userID, id int64) (*domain.Deployment, error) {
			if userID != 2 || id != 9 {
				t.Fatalf("unexpected args userID=%d id=%d", userID, id)
			}
			return want, nil
		},
	}, &githubRepoMock{})

	got, err := uc.GetDeploymentByID(context.Background(), 2, 9)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got.ID != 9 {
		t.Fatalf("unexpected deployment: %+v", got)
	}
}
