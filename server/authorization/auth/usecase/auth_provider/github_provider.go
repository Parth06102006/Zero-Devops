package AuthProvider

import (
	"context"
	"golang.org/x/sync/errgroup"
	"golang.org/x/oauth2"
	"golang.org/x/oauth/github"
	"github.com/sirupsen/logrus"
)

type githubProvider struct{
	config *oauth2.Config
}

// Need to add a return strcut here
func NewGithubProvider(clientId , clientSecret , redirectUrl string) domain.OAuthProvider {
	return &githubProvider{
		config: &oauth2.Config{
			ClientID:     clientId,
			ClientSecret: clientSecret,
			RedirectURL:  redirectUrl,
			Scopes:       []string{"user:email","read:user"},
			Endpoint:     github.Endpoint,
		},
	}
}

func (g *githubProvider) ExchangeCode(ctx context.Context, code string) string{
    token, err := g.config.Exchange(ctx, code)
    if err != nil {
        logrus.Error("github: code exchange failed: %v", err)
        return "",err
    }
    return token.AccessToken , nil
}

func (g* githubProvider) GetUser(ctx context.Context , accessToken string) (*domain.OAuthUser, error) {
	client := g.config.Client(ctx, &oauth2.Token{AccessToken: accessToken})
	user, err := github.UserInfo(ctx, client)
	if err != nil {
		return nil, err
	}
	return &domain.OAuthUser{
		Provider: "github",
		ID:        user.ID,
		Username:  user.Login,
		Email:     user.Email,
		AvatarURL: user.AvatarURL,
	}, nil
}

