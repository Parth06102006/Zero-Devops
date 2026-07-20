// Package usecase contains the authentication business logic
package usecase

import (
	"Zero_Devops/server/internal/domain"
	"context"
	"time"

	appmiddleware "Zero_Devops/server/internal/middleware"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	claimUserID        = "user_id"
	claimExp           = "exp"
	claimIat           = "iat"
	accessTokenExpiry  = 15
	refreshTokenExpiry = 720
)

type authUsecase struct {
	userRepo       domain.UserRepository
	providers      map[string]domain.OAuthProvider
	contextTimeout time.Duration
}

// NewAuthUsecase creates a new auth usecase
func NewAuthUsecase(u domain.UserRepository, providers map[string]domain.OAuthProvider, timeout time.Duration) domain.AuthUsecase {
	return &authUsecase{
		userRepo:       u,
		providers:      providers,
		contextTimeout: timeout,
	}
}

func generateTokens(user *domain.User) (accessToken, refreshToken string, err error) {
	var secretKey = []byte(viper.GetString("JWT_SECRET"))
	accessClaims := jwt.MapClaims{
		claimUserID: user.ID,
		"email":     user.Email,
		claimExp:    time.Now().Add(accessTokenExpiry * time.Minute).Unix(),
		claimIat:    time.Now().Unix(),
	}
	accessTokenSigned := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	signedAccessToken, err := accessTokenSigned.SignedString(secretKey)
	if err != nil {
		return "", "", err
	}
	refreshClaims := jwt.MapClaims{
		claimUserID: user.ID,
		claimExp:    time.Now().Add(refreshTokenExpiry * time.Hour).Unix(),
		claimIat:    time.Now().Unix(),
	}

	refreshTokenSigned := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	signedRefreshToken, err := refreshTokenSigned.SignedString(secretKey)
	if err != nil {
		return "", "", err
	}

	return signedAccessToken, signedRefreshToken, nil
}

func (a *authUsecase) HandleOAuthCallback(ctx context.Context, code, provider string) (*domain.TokenResponse, error) {
	log := appmiddleware.LoggerFromContext(ctx)
	log.Info("Handling OAuth callback", zap.String("provider", provider))

	p, ok := a.providers[provider]
	if !ok {
		log.Error("Provider not supported", zap.String("provider", provider))
		return nil, domain.ErrProviderNotSupported
	}

	providerToken, err := p.ExchangeCode(ctx, code)
	if err != nil {
		return nil, err
	}

	oauthUser, err := p.GetUser(ctx, providerToken)
	if err != nil {
		return nil, err
	}

	existingUser, err := a.userRepo.GetProviderByID(ctx, oauthUser.ProviderID)
	if err != nil && err != domain.ErrNotFound {
		return nil, err
	}

	if existingUser.ID == "" {
		userToSave := domain.User{
			ProviderID: oauthUser.ProviderID,
			Provider:   oauthUser.Provider,
			Username:   oauthUser.Username,
			Email:      oauthUser.Email,
			AvatarURL:  oauthUser.AvatarURL,
			CreatedAt:  time.Now(),
		}
		err := a.userRepo.Store(ctx, &userToSave)
		if err != nil {
			return nil, err
		}
		appAccessToken, appRefreshToken, err := generateTokens(&userToSave)
		if err != nil {
			return nil, err
		}
		err = a.userRepo.UpdateRefreshToken(ctx, userToSave.ID, appRefreshToken)
		if err != nil {
			return nil, err
		}
		return &domain.TokenResponse{
			AccessToken:  appAccessToken,
			RefreshToken: appRefreshToken,
		}, nil
	}

	appAccessToken, appRefreshToken, err := generateTokens(&existingUser)
	if err != nil {
		return nil, err
	}
	err = a.userRepo.UpdateRefreshToken(ctx, existingUser.ID, appRefreshToken)
	if err != nil {
		return nil, err
	}
	return &domain.TokenResponse{
		AccessToken:  appAccessToken,
		RefreshToken: appRefreshToken,
	}, nil
}

func (a *authUsecase) RefreshToken(ctx context.Context, refreshToken string) (*domain.TokenResponse, error) {
	log := appmiddleware.LoggerFromContext(ctx)
	log.Info("Handling token refresh")

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
	userID := claims[claimUserID].(string)

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

	err = a.userRepo.UpdateRefreshToken(ctx, user.ID, newRefreshToken)
	if err != nil {
		return nil, err
	}

	return &domain.TokenResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

// Logout function
func (a *authUsecase) Logout(ctx context.Context, accessToken string) error {
	log := appmiddleware.LoggerFromContext(ctx)
	log.Info("Handling user logout")

	secretKey := (viper.GetString("JWT_SECRET"))
	if secretKey == "" {
		log.Error("JWT secret missing from configuration")
		return domain.ErrMissingSecret
	}

	token, err := jwt.Parse(accessToken, func(_ *jwt.Token) (any, error) {
		hmacSecret := []byte(secretKey)

		return hmacSecret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || token == nil || !token.Valid {
		return domain.ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return domain.ErrInvalidToken
	}

	userID, ok := claims[claimUserID].(string)
	if !ok {
		return domain.ErrInvalidToken
	}

	err = a.userRepo.UpdateRefreshToken(ctx, userID, "")

	if err != nil {
		return domain.ErrLoggingOut
	}

	return nil
}

func (a *authUsecase) GetCurrentUser(ctx context.Context, accessToken string) (domain.UserResponse, error) {
	log := appmiddleware.LoggerFromContext(ctx)

	secretKey := (viper.GetString("JWT_SECRET"))
	if secretKey == "" {
		log.Error("JWT secret missing from configuration")
		return domain.UserResponse{}, domain.ErrMissingSecret
	}

	token, err := jwt.Parse(accessToken, func(_ *jwt.Token) (any, error) {
		hmacSecret := []byte(secretKey)

		return hmacSecret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || token == nil || !token.Valid {
		return domain.UserResponse{}, domain.ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return domain.UserResponse{}, domain.ErrInvalidToken
	}

	userID, ok := claims[claimUserID].(string)
	if !ok {
		return domain.UserResponse{}, domain.ErrInvalidToken
	}

	user, err := a.userRepo.GetByID(ctx, userID)

	if err != nil {
		return domain.UserResponse{}, err
	}

	return domain.UserResponse{
		ID:        user.ID,
		Provider:  user.Provider,
		Username:  user.Username,
		Email:     user.Email,
		AvatarURL: user.AvatarURL,
	}, nil
}
