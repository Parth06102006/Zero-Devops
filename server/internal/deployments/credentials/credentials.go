// Package credentials mints ephemeral GitHub clone credentials for authenticated workers.
package credentials

import (
	"Zero_Devops/server/internal/domain"
	"context"
	"errors"
	"strings"
)

type CloneCredential struct {
	Username string
	Token    string
}

type Usecase interface {
	GetCloneCredential(ctx context.Context, deploymentID string) (*CloneCredential, error)
}

type usecase struct {
	deployments   domain.DeploymentRepository
	installations domain.GithubRepository
	tokens        domain.InstallationTokenProvider
}

func New(deployments domain.DeploymentRepository, installations domain.GithubRepository, tokens domain.InstallationTokenProvider) Usecase {
	return &usecase{deployments: deployments, installations: installations, tokens: tokens}
}

// GetCloneCredential derives the installation solely from the persisted
// deployment. Queue-supplied installation and repository IDs are never trusted
// as authorization inputs.
func (u *usecase) GetCloneCredential(ctx context.Context, deploymentID string) (*CloneCredential, error) {
	if strings.TrimSpace(deploymentID) == "" {
		return nil, domain.ErrBadParamInput
	}
	deployment, err := u.deployments.GetInternalByID(ctx, deploymentID)
	if err != nil {
		return nil, err
	}
	if deployment.Status == domain.DeploymentStatusSuccess || deployment.Status == domain.DeploymentStatusCanceled || deployment.GithubInstallationID == "" {
		return nil, domain.ErrInvalidStatus
	}
	installation, err := u.installations.GetInstallationByID(ctx, deployment.GithubInstallationID)
	if err != nil {
		return nil, err
	}
	if installation.Status != domain.GithubInstallationStatusActive || installation.ConnectionStatus != domain.GithubConnectionStatusConnected {
		return nil, domain.ErrInvalidStatus
	}
	if u.tokens == nil {
		return nil, errors.New("github installation token provider is unavailable")
	}
	token, err := u.tokens.CreateInstallationToken(ctx, installation.InstallationID)
	if err != nil {
		return nil, err
	}
	return &CloneCredential{Username: "x-access-token", Token: token}, nil
}
