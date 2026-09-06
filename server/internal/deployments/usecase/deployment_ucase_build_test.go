package usecase

import (
	"Zero_Devops/server/internal/deployments/contract"
	"Zero_Devops/server/internal/domain"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type projectRepoMock struct {
	getFn func(ctx context.Context, userID, id string) (*domain.Project, error)
}

func (m *projectRepoMock) Store(_ context.Context, _ *domain.Project) error { return nil }
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
func (m *projectRepoMock) Delete(_ context.Context, _, _ string) error       { return nil }
func (m *projectRepoMock) GetProjectRepoAvailability(_ context.Context, _ string) (map[int64]bool, error) {
	return nil, nil
}
func (m *projectRepoMock) UpdateProjectRepoAvailability(_ context.Context, _ string, _ map[int64]bool) error {
	return nil
}
func (m *projectRepoMock) GetByInstallationAndRepositoryID(_ context.Context, _ string, _ int64) (*domain.Project, error) {
	return nil, nil
}

type repositoryClientMock struct{}

func (m *repositoryClientMock) ListRepositories(_ context.Context, _, _, _ string, _ int) (*domain.RepositoryList, error) {
	return nil, nil
}
func (m *repositoryClientMock) GetRepositoryDetails(_ context.Context, _ string, _ int64) (*domain.RepositoryPicker, error) {
	return &domain.RepositoryPicker{}, nil
}
func (m *repositoryClientMock) ResolveCommit(_ context.Context, _, _, _, _ string) (string, error) {
	return "0", nil
}

func newBuildUsecase(pr domain.ProjectRepository, gh domain.GithubRepository, tp domain.InstallationTokenProvider) domain.DeploymentUsecase {
	return NewDeploymentUsecase(context.Background(), &deploymentRepoMock{}, gh, tp, nil, pr, &repositoryClientMock{})
}

// newOutboxBuildUsecase builds a usecase whose deployment repo captures the
// StoreProjectBuildWithOutbox call for assertion.
func newOutboxBuildUsecase(repo *deploymentRepoMock, pr domain.ProjectRepository) domain.DeploymentUsecase {
	return NewDeploymentUsecase(context.Background(), repo, &githubRepoMock{
		getInstFn: func(_ context.Context, _ string) (*domain.GithubInstallation, error) {
			return &domain.GithubInstallation{ID: "inst-1", InstallationID: 10, Status: domain.GithubInstallationStatusActive}, nil
		},
	}, &installationTokenProviderMock{}, nil, pr, &repositoryClientMock{})
}

func outboxTestProject() *domain.Project {
	return &domain.Project{
		ID:                        "p1",
		UserID:                    "u",
		InstallationID:            "inst-1",
		GitHubRepositoryID:        99,
		DesiredRevisionGeneration: 3,
		BuildConfiguration: domain.BuildConfiguration{
			Executable:           "npm",
			Args:                 []string{"run", "build"},
			WorkingDir:           ".",
			ScannerPolicyVersion: "policy-v1",
		},
		ConfigurationVersion: 2,
	}
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
		getFn: func(_ context.Context, _, _ string) (*domain.Project, error) {
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
		getFn: func(_ context.Context, _, _ string) (*domain.Project, error) {
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
		getFn: func(_ context.Context, _, _ string) (*domain.Project, error) {
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

func TestCreateProjectBuild_WritesOutboxEventInSameTransaction(t *testing.T) {
	pr := &projectRepoMock{
		getFn: func(_ context.Context, _, _ string) (*domain.Project, error) {
			return outboxTestProject(), nil
		},
	}

	var captured domain.StoreProjectBuildWithOutboxParams
	repo := &deploymentRepoMock{
		storeProjectBuildOutboxFn: func(_ context.Context, params domain.StoreProjectBuildWithOutboxParams) (*domain.Deployment, error) {
			captured = params
			params.Deployment.ID = "dep-1" // mirrors INSERT ... RETURNING id
			return params.Deployment, nil
		},
	}

	uc := newOutboxBuildUsecase(repo, pr)
	deployment, err := uc.CreateProjectBuild(context.Background(), "u", domain.CreateProjectBuildParams{
		ProjectID:      "p1",
		ShaOrRef:       "main",
		IdempotencyKey: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if deployment == nil || deployment.ID == "" {
		t.Fatalf("expected stored deployment with ID, got %+v", deployment)
	}

	// Outbox params must carry the immutable V1 job metadata.
	if captured.EventID == "" {
		t.Fatal("EventID must be generated")
	}
	if captured.InstallationID != 10 {
		t.Fatalf("InstallationID = %d, want external GitHub installation 10", captured.InstallationID)
	}
	if captured.DesiredRevisionGeneration != 3 {
		t.Fatalf("DesiredRevisionGeneration = %d, want 3", captured.DesiredRevisionGeneration)
	}
	if captured.CorrelationID == "" {
		t.Fatal("CorrelationID must default to a fresh UUID")
	}
	if captured.Deployment.Trigger != contract.TriggerManual {
		t.Fatalf("trigger = %q, want %q", captured.Deployment.Trigger, contract.TriggerManual)
	}
	if captured.Deployment.ManualIdempotencyKey == "" {
		t.Fatal("manual idempotency key must be persisted")
	}
	if captured.Deployment.Status != domain.DeploymentStatusPending {
		t.Fatalf("status = %q, want pending", captured.Deployment.Status)
	}
}

func TestCreateProjectBuild_UsesClientCorrelationIDWhenProvided(t *testing.T) {
	pr := &projectRepoMock{
		getFn: func(_ context.Context, _, _ string) (*domain.Project, error) {
			return outboxTestProject(), nil
		},
	}

	var captured domain.StoreProjectBuildWithOutboxParams
	repo := &deploymentRepoMock{
		storeProjectBuildOutboxFn: func(_ context.Context, params domain.StoreProjectBuildWithOutboxParams) (*domain.Deployment, error) {
			captured = params
			return params.Deployment, nil
		},
	}

	uc := newOutboxBuildUsecase(repo, pr)
	if _, err := uc.CreateProjectBuild(context.Background(), "u", domain.CreateProjectBuildParams{
		ProjectID:      "p1",
		ShaOrRef:       "main",
		IdempotencyKey: uuid.NewString(),
		CorrelationID:  "corr-from-client",
	}); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if captured.CorrelationID != "corr-from-client" {
		t.Fatalf("CorrelationID = %q, want corr-from-client", captured.CorrelationID)
	}
}

func TestCreateProjectBuild_DuplicateIdempotencyKeyReturnsConflict(t *testing.T) {
	pr := &projectRepoMock{
		getFn: func(_ context.Context, _, _ string) (*domain.Project, error) {
			return outboxTestProject(), nil
		},
	}
	repo := &deploymentRepoMock{
		storeProjectBuildOutboxFn: func(context.Context, domain.StoreProjectBuildWithOutboxParams) (*domain.Deployment, error) {
			return nil, domain.ErrConflict
		},
	}

	uc := newOutboxBuildUsecase(repo, pr)
	_, err := uc.CreateProjectBuild(context.Background(), "u", domain.CreateProjectBuildParams{
		ProjectID:      "p1",
		ShaOrRef:       "main",
		IdempotencyKey: uuid.NewString(),
	})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestCreateProjectBuild_OutboxStoreFailurePropagates(t *testing.T) {
	pr := &projectRepoMock{
		getFn: func(_ context.Context, _, _ string) (*domain.Project, error) {
			return outboxTestProject(), nil
		},
	}
	repo := &deploymentRepoMock{
		storeProjectBuildOutboxFn: func(context.Context, domain.StoreProjectBuildWithOutboxParams) (*domain.Deployment, error) {
			return nil, errors.New("db down")
		},
	}

	uc := newOutboxBuildUsecase(repo, pr)
	_, err := uc.CreateProjectBuild(context.Background(), "u", domain.CreateProjectBuildParams{
		ProjectID:      "p1",
		ShaOrRef:       "main",
		IdempotencyKey: uuid.NewString(),
	})
	if err == nil || errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected a propagated store error, got %v", err)
	}
}
