package usecase

import (
	"Zero_Devops/server/domain"
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
)

type authUsecase struct {
	userRepo       domain.UserRepository
	providers      map[string]domain.OAuthProvider
	contextTimeout time.Duration
}

func NewAuthUsecase(u domain.UserRepository, providers map[string]domain.OAuthProvider, timeout time.Duration) domain.AuthUsecase {
	return &authUsecase{
		userRepo:       u,
		providers:      providers,
		contextTimeout: timeout,
	}
}

func generateTokens(user *domain.User) (string, string, error) {
	// So here i am using the byte slices for the jwt signin function then I am using the viper to get the environment variables where I am adding the environment variables
	var secretKey = []byte(viper.GetString("JWT_SECRET"))
	accessClaims := jwt.MapClaims{
		"user_id": user.ID,
		"email":   user.Email,
		"exp":     time.Now().Add(15 * time.Minute).Unix(),
		"iat":     time.Now().Unix(),
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
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

	oauthUser, err := p.GetUser(ctx, providerToken)
	if err != nil {
		return nil, err
	}

	existingUser, err := a.userRepo.GetProviderById(ctx, oauthUser.ProviderId)

	if existingUser.ID == 0 {
		userToSave := domain.User{
			ProviderId: oauthUser.ProviderId,
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
	} else {
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
	secretKey := (viper.GetString("JWT_SECRET"))
	if secretKey == "" {
		return domain.ErrMissingSecret
	}
	// Validate the Token
	// Get the user detail
	// After getting the user detail I need to remove the refresh token from the database
	//The removal of the token from the cookie would be done by the Logout API

	// Parsing And Validating Access Token
	token, err := jwt.Parse(accessToken, func(token *jwt.Token) (any, error) {
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

	// We are using the type assertion since claims is of type interference and since userId is stored as string in claims then we check if it is stored as string since it is stored as string then programmaticaly at the run time we would assing the value to the userId otherwise run time panic would occur
	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		return domain.ErrInvalidToken
	}
	userId := int64(userIDFloat)

	// Updating the Refresh Token to Empty String to Not store it anymore
	err = a.userRepo.UpdateRefreshToken(ctx, userId, "")

	if err != nil {
		return domain.ErrLoggingOut
	}

	return nil
}

func (a *authUsecase) GetCurrentUser(ctx context.Context, accessToken string) (domain.UserResponse, error) {
	secretKey := (viper.GetString("JWT_SECRET"))
	if secretKey == "" {
		return domain.UserResponse{}, domain.ErrMissingSecret
	}

	token, err := jwt.Parse(accessToken, func(token *jwt.Token) (any, error) {
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

	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		return domain.UserResponse{}, domain.ErrInvalidToken
	}
	userId := int64(userIDFloat)

	user, err := a.userRepo.GetByID(ctx, userId)

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
