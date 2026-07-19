package domain

import (
	"context"
)

// Session Management Tokens
type TokenResponse struct {
	AccessToken		string 		`json:"accessToken"`
	RefreshToken	string	`json:"refreshToken"`
}

type OAuthUser struct {
	Provider	string
	ProviderID 	int64
	Username	string
	Email		string
	AvatarURL	string
}

type UserResponse struct {
	ID        int64  `json:"id"`
	Provider  string `json:"provider"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatarURL"`
}

type OAuthProvider interface {
	ExchangeCode(ctx context.Context, code string) (string,error)
	GetUser(ctx context.Context , accessToken string)(*OAuthUser, error)
}

//  Oauth Methods
type AuthUsecase interface {
	
	HandleOAuthCallback(ctx context.Context, code string, provider string) (*TokenResponse, error)
	
	// FUTURE : CUSTOM AUTH
	// RegisterCustom(ctx context.Context , username string , email string , password string) error
	// LoginCustom(ctx context.Context, email string , password string) (*TokenResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error)
	GetCurrentUser(ctx context.Context,accessToken string) (UserResponse,error)
	Logout(ctx context.Context, accessToken string) error
}