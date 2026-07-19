// Package AuthProvider provides OAuth authentication providers
package AuthProvider

import (
	"Zero_Devops/server/domain"
	"context"
	"encoding/json"
	"net/http"

	appmiddleware "Zero_Devops/server/middleware"

	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

type githubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

type githubProvider struct {
	config *oauth2.Config
}

// NewGithubProvider returns a new github provider
func NewGithubProvider(clientID, clientSecret, redirectURL string) domain.OAuthProvider {
	return &githubProvider{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"user:email", "read:user"},
			Endpoint:     github.Endpoint,
		},
	}
}

func (g *githubProvider) ExchangeCode(ctx context.Context, code string) (string, error) {
	token, err := g.config.Exchange(ctx, code)
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("github: code exchange failed", zap.Error(err))
		return "", err
	}
	return token.AccessToken, nil
}

func (g *githubProvider) GetUser(ctx context.Context, accessToken string) (*domain.OAuthUser, error) {
	client := g.config.Client(ctx, &oauth2.Token{AccessToken: accessToken})

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", http.NoBody)
	if err != nil {
		return nil, err
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			appmiddleware.LoggerFromContext(ctx).Error("failed to close response body", zap.Error(err))
		}
	}()

	ghUser := githubUser{}

	err = json.NewDecoder(res.Body).Decode(&ghUser)

	if err != nil {
		return nil, err
	}

	return &domain.OAuthUser{
		Provider:   "github",
		ProviderID: ghUser.ID,
		Username:   ghUser.Login,
		Email:      ghUser.Email,
		AvatarURL:  ghUser.AvatarURL,
	}, nil
}
