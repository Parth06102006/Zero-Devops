package domain

import (
	"context"
	"time"
)

// User represents the user domain model for OAuth callback handling
type User struct {
	ID           string    `json:"id"`
	ProviderID   int64     `json:"providerId"`
	Provider     string    `json:"provider"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	AvatarURL    string    `json:"avatarURL"`
	CreatedAt    time.Time `json:"createdAt"`
	RefreshToken string    `json:"refreshToken"`
}

// UserRepository defines the interface for user data operations
type UserRepository interface {
	GetByID(ctx context.Context, id string) (User, error)
	GetByUsername(ctx context.Context, username string) (User, error)
	GetProviderByID(ctx context.Context, providerID int64) (User, error)
	Store(ctx context.Context, u *User) error
	UpdateRefreshToken(ctx context.Context, id string, refreshToken string) error
}
