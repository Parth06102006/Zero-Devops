package usecase

import (
	"Zero_Devops/server/internal/domain"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type projectRepoMock struct {
	getFn func(ctx context.Context, userID, id string) (*domain.Project, error)
}

func (m *projectRepoMock) Store(_ context.Context, _ *domain.Project) error    { return nil }
func (m *projectRepoMock) ListByUserID(_ context.Context, _ string) ([]domain.Project, error) {
	return nil, nil
}
func (m *projectRepoMock) GetByID(ctx context.Context, userID, id string) (*domain.Project, error) {
	if m.getFn != nil {
		return m.getFn(ctx, userID, id)
	}
	return nil, nil
}
func (m *projectRepoMock) Update(_ context.Context, _ *domain.Project) error { return nil }
func (m *projectRepoMock) Delete(_ context.Context, _ , _ string) error         { return nil }

type repositoryClientMock struct{}

func (m *repositoryClientMock) ListRepositories(_ context.Context, _ , _ , _ string, _ int) (*domain.RepositoryList, error) {
	return nil, nil
}
func (m *repositoryClientMock) GetRepositoryDetails(_ context.Context, _ string, _ int64) (*domain.RepositoryPicker, error) {
	return &domain.RepositoryPicker{}, nil
}
func (m *repositoryClientMock) ResolveCommit(_ context.Context, _ , _ , _ , _ string) (string, error) {
	return "0", nil
}

func newBuildUsecase(pr domain.ProjectRepository, gh domain.GithubRepository, tp domain.InstallationTokenProvider) domain.DeploymentUsecase {
	return NewDeploymentUsecase(&deploymentRepoMock{}, gh, tp, nil, pr, &repositoryClientMock{})
}

func TestCreateProjectBuild_MissingIdempotencyKey(t *testing.T) {
	uc := newBuildUsecase(&projectRepoMock{}, &githubRepoMock{}, &installationTokenProviderMock{})
	_, err := uc.CreateProjectBuild(context.Background(), "u", domain.CreateProjectBuildParams{
		ProjectID: "p1",
		ShaOrRef:  "main",
	})
	if !errors.Is(err, domain.ErrBadParamInput) {
		t.Fatalf("expected ErrBadParamInput, got %v", err)
	}
}

func TestCreateProjectBuild_ProjectNotFound(t *testing.T) {
	pr := &projectRepoMock{
		getFn: func(_ context.Context, _ , _ string) (*domain.Project, error) {
			return nil, domain.ErrNotFound
		},
	}
	gh := &githubRepoMock{
		getInstFn: func(_ context.Context, _ string) (*domain.GithubInstallation, error) {
			return &domain.GithubInstallation{ID: "inst-1", InstallationID: 10, Status: domain.GithubInstallationStatusActive}, nil
		},
	}
	uc := newBuildUsecase(pr, gh, &installationTokenProviderMock{})
	_, err := uc.CreateProjectBuild(context.Background(), "u", domain.CreateProjectBuildParams{
		ProjectID:      "p1",
		ShaOrRef:       "main",
		IdempotencyKey: uuid.NewString(),
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestCreateProjectBuild_InactiveInstallation(t *testing.T) {
	pr := &projectRepoMock{
		getFn: func(_ context.Context, _ , _ string) (*domain.Project, error) {
			return &domain.Project{ID: "p1", UserID: "u", InstallationID: "inst-1", GitHubRepositoryID: 99}, nil
		},
	}
	gh := &githubRepoMock{
		getInstFn: func(_ context.Context, _ string) (*domain.GithubInstallation, error) {
			return &domain.GithubInstallation{ID: "inst-1", InstallationID: 10, Status: domain.GithubInstallationStatusSuspended}, nil
		},
	}
	uc := newBuildUsecase(pr, gh, &installationTokenProviderMock{})
	_, err := uc.CreateProjectBuild(context.Background(), "u", domain.CreateProjectBuildParams{
		ProjectID:      "p1",
		ShaOrRef:       "main",
		IdempotencyKey: uuid.NewString(),
	})
	if !errors.Is(err, domain.ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestCreateProjectBuild_InstallationMismatch(t *testing.T) {
	pr := &projectRepoMock{
		getFn: func(_ context.Context, _ , _ string) (*domain.Project, error) {
			return &domain.Project{ID: "p1", UserID: "u", InstallationID: "inst-1", GitHubRepositoryID: 99}, nil
		},
	}
	gh := &githubRepoMock{
		getInstFn: func(_ context.Context, _ string) (*domain.GithubInstallation, error) {
			// Active, but belongs to a different installation than the project.
			return &domain.GithubInstallation{ID: "inst-2", InstallationID: 20, Status: domain.GithubInstallationStatusActive}, nil
		},
	}
	uc := newBuildUsecase(pr, gh, &installationTokenProviderMock{})
	_, err := uc.CreateProjectBuild(context.Background(), "u", domain.CreateProjectBuildParams{
		ProjectID:      "p1",
		ShaOrRef:       "main",
		IdempotencyKey: uuid.NewString(),
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound on installation mismatch, got %v", err)
	}
}
