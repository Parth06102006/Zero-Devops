package usecase

import (
	"Zero_Devops/server/internal/domain"
	"context"
	"errors"
	"net/http"
	"testing"
)

type projectRepoMock struct {
	storeFn  func(ctx context.Context, p *domain.Project) error
	listFn   func(ctx context.Context, userID string) ([]domain.Project, error)
	getFn    func(ctx context.Context, userID, id string) (*domain.Project, error)
	updateFn func(ctx context.Context, p *domain.Project) error
	deleteFn func(ctx context.Context, userID, id string) error
}

func (m *projectRepoMock) Store(ctx context.Context, p *domain.Project) error {
	if m.storeFn != nil {
		return m.storeFn(ctx, p)
	}
	return nil
}
func (m *projectRepoMock) ListByUserID(ctx context.Context, userID string) ([]domain.Project, error) {
	if m.listFn != nil {
		return m.listFn(ctx, userID)
	}
	return nil, nil
}
func (m *projectRepoMock) GetByID(ctx context.Context, userID, id string) (*domain.Project, error) {
	if m.getFn != nil {
		return m.getFn(ctx, userID, id)
	}
	return nil, nil
}
func (m *projectRepoMock) Update(ctx context.Context, p *domain.Project) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, p)
	}
	return nil
}
func (m *projectRepoMock) Delete(ctx context.Context, userID, id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, userID, id)
	}
	return nil
}
func (m *projectRepoMock) GetProjectRepoAvailability(_ context.Context, _ string) (map[int64]bool, error) {
	return nil, nil
}
func (m *projectRepoMock) UpdateProjectRepoAvailability(_ context.Context, _ string, _ map[int64]bool) error {
	return nil
}
func (m *projectRepoMock) GetByInstallationAndRepositoryID(_ context.Context, _ string, _ int64) (*domain.Project, error) {
	return nil, nil
}

type githubUsecaseMock struct {
	detailsFn func(ctx context.Context, userID string, repoID int64) (*domain.RepositoryPicker, error)
	instFn    func(ctx context.Context, userID string) (*domain.GithubInstallation, error)
}

func (m *githubUsecaseMock) InstallGithubApp(_ context.Context, _ *http.Client, _, _ string) error {
	return nil
}
func (m *githubUsecaseMock) DeleteGithubApp(_ context.Context, _ string) error { return nil }
func (m *githubUsecaseMock) GetGithubAppInstallation(ctx context.Context, userID string) (*domain.GithubInstallation, error) {
	if m.instFn != nil {
		return m.instFn(ctx, userID)
	}
	return nil, nil
}
func (m *githubUsecaseMock) ListRepositories(_ context.Context, _, _, _ string, _ int) (*domain.RepositoryList, error) {
	return nil, nil
}
func (m *githubUsecaseMock) GetRepositoryDetails(ctx context.Context, userID string, repoID int64) (*domain.RepositoryPicker, error) {
	if m.detailsFn != nil {
		return m.detailsFn(ctx, userID, repoID)
	}
	return nil, nil
}
func (m *githubUsecaseMock) InvalidateRepositoryCache(_ context.Context, _ int64) error { return nil }

type commandScannerMock struct {
	scanFn func(ctx context.Context, cfg domain.BuildConfiguration) domain.CommandScanResult
}

func (m *commandScannerMock) Scan(ctx context.Context, cfg domain.BuildConfiguration) domain.CommandScanResult {
	if m.scanFn != nil {
		return m.scanFn(ctx, cfg)
	}
	return domain.CommandScanResult{Status: domain.CommandScanStatusApproved, PolicyVersion: "v1"}
}

func TestCreateProject_Success(t *testing.T) {
	var stored *domain.Project
	repo := &projectRepoMock{
		storeFn: func(_ context.Context, p *domain.Project) error {
			stored = p
			return nil
		},
	}
	gh := &githubUsecaseMock{
		detailsFn: func(_ context.Context, _ string, _ int64) (*domain.RepositoryPicker, error) {
			return &domain.RepositoryPicker{ID: 99, Owner: "o", Name: "n", FullName: "o/n", DefaultBranch: "main"}, nil
		},
		instFn: func(_ context.Context, _ string) (*domain.GithubInstallation, error) {
			return &domain.GithubInstallation{ID: "inst-1", InstallationID: 10, Status: domain.GithubInstallationStatusActive}, nil
		},
	}
	uc := NewProjectUsecase(repo, gh, &commandScannerMock{})

	got, err := uc.CreateProject(context.Background(), "u", domain.CreateProjectParams{
		RepositoryID:          99,
		ConfiguredBranch:      "feature",
		ProjectWebhookEnabled: true,
		BuildConfiguration:    domain.BuildConfiguration{Executable: "go", Args: []string{"build"}, WorkingDir: "/app"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ConfigurationVersion != 1 {
		t.Fatalf("expected ConfigurationVersion 1, got %d", got.ConfigurationVersion)
	}
	if got.ConfiguredBranch != "refs/heads/feature" {
		t.Fatalf("expected normalized branch refs/heads/feature, got %q", got.ConfiguredBranch)
	}
	if stored == nil || stored.InstallationID != "inst-1" {
		t.Fatalf("project not stored with installation id: %+v", stored)
	}
}

func TestCreateProject_DuplicateSelectionConflict(t *testing.T) {
	repo := &projectRepoMock{
		storeFn: func(_ context.Context, _ *domain.Project) error {
			return domain.ErrConflict
		},
	}
	gh := &githubUsecaseMock{
		detailsFn: func(_ context.Context, _ string, _ int64) (*domain.RepositoryPicker, error) {
			return &domain.RepositoryPicker{ID: 99, DefaultBranch: "main"}, nil
		},
		instFn: func(_ context.Context, _ string) (*domain.GithubInstallation, error) {
			return &domain.GithubInstallation{ID: "inst-1", Status: domain.GithubInstallationStatusActive}, nil
		},
	}
	uc := NewProjectUsecase(repo, gh, &commandScannerMock{})

	_, err := uc.CreateProject(context.Background(), "u", domain.CreateProjectParams{
		RepositoryID:       99,
		ConfiguredBranch:   "main",
		BuildConfiguration: domain.BuildConfiguration{Executable: "go", Args: []string{"build"}, WorkingDir: "/app"},
	})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected ErrConflict for duplicate selection, got %v", err)
	}
}

func TestCreateProject_RepositoryNotVisible(t *testing.T) {
	repo := &projectRepoMock{}
	gh := &githubUsecaseMock{
		detailsFn: func(_ context.Context, _ string, _ int64) (*domain.RepositoryPicker, error) {
			return nil, domain.ErrNotFound
		},
	}
	uc := NewProjectUsecase(repo, gh, &commandScannerMock{})

	_, err := uc.CreateProject(context.Background(), "u", domain.CreateProjectParams{
		RepositoryID:       99,
		ConfiguredBranch:   "main",
		BuildConfiguration: domain.BuildConfiguration{Executable: "go", Args: []string{"build"}, WorkingDir: "/app"},
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound when repository is not visible, got %v", err)
	}
}

func TestCreateProject_CommandDenied(t *testing.T) {
	repo := &projectRepoMock{}
	gh := &githubUsecaseMock{
		detailsFn: func(_ context.Context, _ string, _ int64) (*domain.RepositoryPicker, error) {
			return &domain.RepositoryPicker{ID: 99, DefaultBranch: "main"}, nil
		},
		instFn: func(_ context.Context, _ string) (*domain.GithubInstallation, error) {
			return &domain.GithubInstallation{ID: "inst-1", Status: domain.GithubInstallationStatusActive}, nil
		},
	}
	scanner := &commandScannerMock{
		scanFn: func(_ context.Context, _ domain.BuildConfiguration) domain.CommandScanResult {
			return domain.CommandScanResult{Status: domain.CommandScanStatusDenied, DeniedReason: "curl is banned"}
		},
	}
	uc := NewProjectUsecase(repo, gh, scanner)

	_, err := uc.CreateProject(context.Background(), "u", domain.CreateProjectParams{
		RepositoryID:       99,
		ConfiguredBranch:   "main",
		BuildConfiguration: domain.BuildConfiguration{Executable: "curl", Args: []string{"evil"}, WorkingDir: "/app"},
	})
	if !errors.Is(err, domain.ErrCommandDenied) {
		t.Fatalf("expected ErrCommandDenied, got %v", err)
	}
}

func TestToFullRef(t *testing.T) {
	cases := []struct {
		in, fallback, want string
	}{
		{"main", "master", "refs/heads/main"},
		{"", "master", "refs/heads/master"},
		{"refs/heads/main", "master", "refs/heads/main"},
		{"  dev  ", "", "refs/heads/dev"},
		{"", "", ""},
	}
	for _, c := range cases {
		if got := toFullRef(c.in, c.fallback); got != c.want {
			t.Fatalf("toFullRef(%q,%q)=%q want %q", c.in, c.fallback, got, c.want)
		}
	}
}
