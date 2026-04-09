package domain

import "context"

// Session Management Tokens
type TokenResponse struct {
	AccessToken		string 		`json:"accessToken"`
	RefreshToken	string	`json:"refreshToken"`
}

type OAuthUser struct {
	Provider	string
	ProviderId 	int64
	Username	string
	Email		string
	AvatarURL	string
}

type OAuthProvider interface {
	ExchangeCode(ctx context.Context, code string) (string,error)
	GetUser(ctx context.Context , accessToken string)(*OAuthUser, error)
}
var supportedProviders = map[string]bool{
	"google": false,
	"github": true,
}

//  Oauth Methods
type AuthUsecase interface {
	
	HandleOAuthCallback(ctx context.Context, code string, provider string) (*TokenResponse, error)
	
	// FUTURE : CUSTOM AUTH
	// RegisterCustom(ctx context.Context , username string , email string , password string) error
	// LoginCustom(ctx context.Context, email string , password string) (*TokenResponse, error)
	
	RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error)
	Logout(ctx context.Context, accessToken string) error
}