package usecase

import (
	"context"
	"time"
	
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"server/domain"
)

type authUsecase struct {
	userRepo domain.UserRepository
	supportedProviders domain.supportedProviders
	contextTimeout time.Duration
}

func NewUserUsecase(u domain.UserRepository, g domain.GithubRepository, timeout time.Duration) domain.AuthUsecase {
	return &authUsecase{
		userRepo: u,
		contextTimeout: timeout,
	}
}

func (a *authUsecase) HandleOAuthCallback(ctx context.Context , code string ,provider string) (*domain.TokenResponse, error) {
	if _, ok := a.supportedProviders[provider]; !ok {
		return nil, ErrProviderNotSupported
	}
	
	p,ok = 
	
	
	
	
}