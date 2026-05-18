package domain

import (
	"context"
	"time"
)

// Handle oauth callback
// Login and Signup if created
// Represents the User data struct
type User struct {
	ID           int64     `json:"id"`
	ProviderId   int64     `json:"providerId"`
	Provider     string    `json:"provider"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	AvatarURL    string    `json:"avatarURL"`
	CreatedAt    time.Time `json:"createdAt"`
	RefreshToken string    `json:"refreshToken"`
}

type UserRepository interface {
	GetByID(ctx context.Context, id int64) (User, error)
	GetByUsername(ctx context.Context, username string) (User, error)
	GetProviderById(ctx context.Context, providerId int64) (User, error)
	Store(ctx context.Context, u *User) error
	UpdateRefreshToken(ctx context.Context, id int64, refreshToken string) error
}
