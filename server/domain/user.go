package domain

import (
	"context"
	"time"
)

// Handle oauth callback
// Login and Signup if created
// Represents the User data struct
type User struct {
	ID			int64		`json:"id"`
	ProviderID	int64	`json:"providerId"`
	Provider 	string	`json:"provider"`
	Username	string		`json:"username"`
	Email 		string	`json:"email"`
	AvatarURL 	string	`json:"avatarURL"`
	CreatedAt 	time.Time	`json:"createdAt"`,
}

type UserRepository interface {
	GetByID(ctx context.Context , id int64) (User, error)
	GetByUsername(ctx context.Context , username string) (User , error)
	Store(ctx context.Context,u *User) error
	UpdateAccessToken(ctx context.Context, id int64, token string) error
}