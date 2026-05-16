package usecase

import (
	"context"
	"net/http"
	"time"
	"Zero_Devops/server/domain"
)

type githubAppUsecase struct{}

func NewGiithubAppUsecase() *githubAppUsecase {
	return &githubAppUsecase{}
}

func (g *githubAppUsecase) InstallGithubApp(ctx context.Context) error {

	return nil
}

func (g *githubAppUsecase) GetGithubAppInstallation(ctx context.Context) error {
	return nil
}

func (g *githubAppUsecase) DeleteGithubApp(ctx context.Context) error {
	return nil
}
