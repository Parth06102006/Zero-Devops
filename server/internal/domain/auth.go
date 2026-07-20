// Package domain provides domain models and interfaces for the application
package domain

import (
	"context"
)

// TokenResponse represents the session management tokens returned after authentication
type TokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

// OAuthUser represents the user data returned by an OAuth provider
type OAuthUser struct {
	Provider   string
	ProviderID int64
	Username   string
	Email      string
	AvatarURL  string
}

// UserResponse represents the public user data returned to clients
type UserResponse struct {
	ID        string `json:"id"`
	Provider  string `json:"provider"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatarURL"`
}

// OAuthProvider defines the interface for OAuth authentication providers
type OAuthProvider interface {
	ExchangeCode(ctx context.Context, code string) (string, error)
	GetUser(ctx context.Context, accessToken string) (*OAuthUser, error)
}

// AuthUsecase defines the interface for authentication use cases
type AuthUsecase interface {
	HandleOAuthCallback(ctx context.Context, code string, provider string) (*TokenResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error)
	GetCurrentUser(ctx context.Context, accessToken string) (UserResponse, error)
	Logout(ctx context.Context, accessToken string) error
}
