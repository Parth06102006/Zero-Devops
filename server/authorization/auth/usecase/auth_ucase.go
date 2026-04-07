package usecase

import (
	"context"
	"server/domain"
	"time"

	"github.com/spf13/viper"
)

type authUsecase struct {
	userRepo domain.UserRepository
	providers      map[string]domain.OAuthProvider
	contextTimeout time.Duration
}

func NewAuthUsecase(u domain.UserRepository, providers map[string]domain.OAuthProvider, timeout time.Duration) domain.AuthUsecase {
	return &authUsecase{
		userRepo: u,
		providers: providers,
		contextTimeout: timeout,
	}
}

func generateTokens(user *domain.User) (string , string){
	var secretKey = []byte(viper.GetString("JWT_SECRET"))
	// need to wrtie the function to generate the accessToken and the refreshToken
}

func (a *authUsecase) HandleOAuthCallback(ctx context.Context, code string, provider string) (*domain.TokenResponse, error) {
	p, ok := a.providers[provider]
	if !ok {
		return nil, domain.ErrProviderNotSupported
	}

	providerToken, err := p.ExchangeCode(ctx, code)
	if err != nil {
		return nil, err
	}

	oauthUser,err := p.GetUser(ctx,providerToken)
	if err != nil{
		return nil,err
	}

	existingUser,err := a.userRepo.GetByProviderId(ctx,	oauthUser.ProviderId)
	
	appAccessToken , appRefreshToken := generateTokens(oauthUser)

	if existingUser.ID == 0 {
		userToSave := domain.User{
			ProviderID: oauthUser.ProviderId,
			Provider:   oauthUser.Provider,
			Username:   oauthUser.Username,
			Email:      oauthUser.Email,
			AvatarURL:  oauthUser.AvatarURL,
			CreatedAt:  time.Now(),
			RefreshToken: appRefreshToken,
		}
		err := a.userRepo.Store(ctx, &userToSave)
		if err != nil {
			return nil, err
		}
	} else {
		err := a.userRepo.Update(ctx, existingUser.ID, appRefreshToken)
		if err != nil {
			return nil, err
		}
	}

	return &domain.TokenResponse{
        AccessToken:  appAccessToken,
        RefreshToken: appRefreshToken,
    }, nil

}

func (a *authUsecase) RefreshToken(ctx context.Context, refreshToken string) error {
	
}