package usecase

import (
	"context"
	"server/domain"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

func generateTokens(user *domain.User) (string , string,error){
	var secretKey = []byte(viper.GetString("JWT_SECRET"))
	accessClaims := jwt.MapClaims{
		"user_id":user.ID,
		"email":user.Email,
		"exp":time.Now().Add(15*time.Minute).Unix(),
		"iat":time.Now().Unix(),
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256,accessClaims)
	signedAccessToken, err := accessToken.SignedString(secretKey)
	if err != nil {
		return "", "", err
	}
	refreshClaims := jwt.MapClaims{
		"user_id": user.ID,
		"exp":     time.Now().Add(7 * 24 * time.Hour).Unix(), 
		"iat":     time.Now().Unix(),
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	signedRefreshToken, err := refreshToken.SignedString(secretKey)
	if err != nil {
		return "", "", err
	}

	return signedAccessToken, signedRefreshToken, nil
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
	
	appAccessToken , appRefreshToken,err := generateTokens(oauthUser)
	if err != nil {
	return nil, err
}
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

  func (a *authUsecase) RefreshToken(ctx context.Context, refreshToken string) (*domain.TokenResponse, error) {
        secretKey := []byte(viper.GetString("JWT_SECRET"))

        token, err := jwt.Parse(refreshToken, func(token *jwt.Token) (interface{}, error) {
                if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
                        return nil, domain.ErrInvalidToken
                }
                return secretKey, nil
        })
        if err != nil || !token.Valid {
                return nil, domain.ErrInvalidToken
        }

        claims, ok := token.Claims.(jwt.MapClaims)
        if !ok {
                return nil, domain.ErrInvalidToken
        }
        userID := int64(claims["user_id"].(float64))

        user, err := a.userRepo.GetByID(ctx, userID)
        if err != nil {
                return nil, err
        }

        if user.RefreshToken != refreshToken {
                return nil, domain.ErrInvalidToken
        }

        newAccessToken, newRefreshToken, err := generateTokens(&user)
        if err != nil {
                return nil, err
        }

        err = a.userRepo.Update(ctx, user.ID, newRefreshToken)
        if err != nil {
                return nil, err
        }

        return &domain.TokenResponse{
                AccessToken:  newAccessToken,
                RefreshToken: newRefreshToken,
        }, nil
  }