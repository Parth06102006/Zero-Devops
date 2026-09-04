package usecase

import (
	"Zero_Devops/server/internal/domain"
	appmiddleware "Zero_Devops/server/internal/middleware"
	"context"
	"strings"
	"time"

	"go.uber.org/zap"
)

type projectUsecase struct {
	projectRepo    domain.ProjectRepository
	githubUsecase  domain.GithubUsecase
	commandScanner domain.CommandScanner
}

func NewProjectUsecase(projectRepo domain.ProjectRepository, githubUsecase domain.GithubUsecase, commandScanner domain.CommandScanner) domain.ProjectUsecase {
	return &projectUsecase{projectRepo: projectRepo, githubUsecase: githubUsecase, commandScanner: commandScanner}
}

func (p *projectUsecase) CreateProject(ctx context.Context, userID string, params domain.CreateProjectParams) (*domain.Project, error) {
	log := appmiddleware.LoggerFromContext(ctx)

	picker, err := p.githubUsecase.GetRepositoryDetails(ctx, userID, params.RepositoryID)
	if err != nil {
		log.Error("failed to verify repository details", zap.Error(err), zap.Int64("repository_id", params.RepositoryID))
		return nil, err
	}

	installation, err := p.githubUsecase.GetGithubAppInstallation(ctx, userID)
	if err != nil {
		log.Error("failed to get github installation", zap.Error(err))
		return nil, err
	}

	scanResult := p.commandScanner.Scan(ctx, params.BuildConfiguration)
	if scanResult.Status == domain.CommandScanStatusDenied {
		log.Warn("build command denied by policy",
			zap.String("denied_reason", scanResult.DeniedReason),
			zap.Int64("repository_id", params.RepositoryID))
		return nil, domain.ErrCommandDenied
	}

	project := &domain.Project{
		UserID:                    userID,
		InstallationID:            installation.ID,
		GitHubRepositoryID:        picker.ID,
		RepositoryOwner:           picker.Owner,
		RepositoryName:            picker.Name,
		RepositoryFullName:        picker.FullName,
		ConfiguredBranch:          toFullRef(params.ConfiguredBranch, picker.DefaultBranch),
		ProjectWebhookEnabled:     params.ProjectWebhookEnabled,
		RepositoryAvailable:       true,
		DesiredRevisionGeneration: 0,
		BuildConfiguration:        params.BuildConfiguration,
		ConfigurationVersion:      1,
		CommandPolicyVersion:      scanResult.PolicyVersion,
		CommandScanResult:         scanResult,
		CreatedAt:                 time.Now(),
		UpdatedAt:                 time.Now(),
	}

	if err := p.projectRepo.Store(ctx, project); err != nil {
		log.Error("failed to store project", zap.Error(err))
		return nil, err
	}

	return project, nil
}

func (p *projectUsecase) ListProjects(ctx context.Context, userID string) ([]domain.Project, error) {
	projects, err := p.projectRepo.ListByUserID(ctx, userID)
	if err != nil {
		appmiddleware.LoggerFromContext(ctx).Error("failed to list projects", zap.Error(err))
		return nil, err
	}
	return projects, nil
}

func (p *projectUsecase) GetProject(ctx context.Context, userID, projectID string) (*domain.Project, error) {
	project, err := p.projectRepo.GetByID(ctx, userID, projectID)
	if err != nil {
		appmiddleware.LoggerFromContext(ctx).Error("failed to get project", zap.Error(err))
		return nil, err
	}
	return project, nil
}

func (p *projectUsecase) UpdateProject(ctx context.Context, userID, projectID string, params domain.UpdateProjectParams) (*domain.Project, error) {
	project, err := p.projectRepo.GetByID(ctx, userID, projectID)
	if err != nil {
		appmiddleware.LoggerFromContext(ctx).Error("failed to get project for update", zap.Error(err))
		return nil, err
	}

	if params.ConfiguredBranch != nil {
		project.ConfiguredBranch = toFullRef(*params.ConfiguredBranch, project.RepositoryName)
	}
	if params.ProjectWebhookEnabled != nil {
		project.ProjectWebhookEnabled = *params.ProjectWebhookEnabled
	}
	if params.BuildConfiguration != nil {
		scanResult := p.commandScanner.Scan(ctx, *params.BuildConfiguration)
		if scanResult.Status == domain.CommandScanStatusDenied {
			appmiddleware.LoggerFromContext(ctx).Warn("build command denied by policy",
				zap.String("denied_reason", scanResult.DeniedReason),
				zap.String("project_id", projectID))
			return nil, domain.ErrCommandDenied
		}
		project.BuildConfiguration = *params.BuildConfiguration
		project.CommandPolicyVersion = scanResult.PolicyVersion
		project.CommandScanResult = scanResult
	}

	if err := p.projectRepo.Update(ctx, project); err != nil {
		if err == domain.ErrNotFound {
			return nil, err
		}
		appmiddleware.LoggerFromContext(ctx).Error("failed to update project", zap.Error(err))
		return nil, err
	}

	return project, nil
}

func (p *projectUsecase) DeleteProject(ctx context.Context, userID, projectID string) error {
	if err := p.projectRepo.Delete(ctx, userID, projectID); err != nil {
		appmiddleware.LoggerFromContext(ctx).Error("failed to delete project", zap.Error(err))
		return err
	}
	return nil
}

func toFullRef(branch, fallback string) string {
	if strings.TrimSpace(branch) == "" {
		branch = fallback
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return ""
	}
	if strings.HasPrefix(branch, "refs/heads/") {
		return branch
	}
	return "refs/heads/" + strings.TrimPrefix(branch, "/")
}
