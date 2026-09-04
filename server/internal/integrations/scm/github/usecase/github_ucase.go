// Package usecase contains GitHub integration business logic
package usecase

import (
	"Zero_Devops/server/internal/domain"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	
	"Zero_Devops/server/internal/integrations/scm/github/cache"
	appmiddleware "Zero_Devops/server/internal/middleware"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const jwtExpiryMinutes = 10

type githubAppUsecase struct {
	githubRepo       domain.GithubRepository
	tokenProvider    domain.InstallationTokenProvider
	repositoryClient domain.GithubRepositoryClient
	cache            cache.RepositoryListCache
	repoListMu       sync.Mutex
	repoListCalls    map[string]*repoListCall
}

type repoListCall struct {
	wg     sync.WaitGroup
	result *domain.RepositoryList
	err    error
}

// NewGithubAppUsecase creates a new GithubUsecase
func NewGithubAppUsecase(githubRepo domain.GithubRepository, dependencies ...interface{}) domain.GithubUsecase {
	usecase := &githubAppUsecase{githubRepo: githubRepo, repoListCalls: make(map[string]*repoListCall)}
	for _, dependency := range dependencies {
		switch value := dependency.(type) {
		case domain.InstallationTokenProvider:
			usecase.tokenProvider = value
		case domain.GithubRepositoryClient:
			usecase.repositoryClient = value
		case *cache.RedisRepositoryListCache:
			usecase.cache = value
		case cache.RepositoryListCache:
			usecase.cache = value
		}
	}
	return usecase
}

// GithubTokenResponse represents an OAuth token response from GitHub
type GithubTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

// Installation represents a GitHub App installation
type Installation struct {
	ID      int64 `json:"id"`
	Account struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	}
	AppID int64 `json:"app_id"`
}

// GithubInstallationList represents a list of GitHub App installations
type GithubInstallationList struct {
	TotalCount    int            `json:"total_count"`
	Installations []Installation `json:"installations"`
}

func (g *githubAppUsecase) InstallGithubApp(ctx context.Context, client *http.Client, code, userID string) error {
	log := appmiddleware.LoggerFromContext(ctx)
	log.Info("Starting GitHub App installation", zap.String("user_id", userID))

	githubAppClientID := viper.GetString("GITHUB_APP_CLIENT_ID")
	githubAppClientSecret := viper.GetString("GITHUB_APP_CLIENT_SECRET")
	githubAppID := viper.GetInt64("GITHUB_APP_ID")

	data := url.Values{}
	data.Add("client_id", githubAppClientID)
	data.Add("client_secret", githubAppClientSecret)
	data.Add("code", code)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))

	if err != nil {
		return domain.ErrInvalidCode
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	response, err := client.Do(req)
	if err != nil {
		log.Error("Failed to perform access token request", zap.Error(err))
		return err
	}

	defer func() {
		if err := response.Body.Close(); err != nil {
			log.Error("failed to close response body", zap.Error(err))
		}
	}()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return domain.ErrInvalidCode
	}

	var githubTokenResponse GithubTokenResponse
	if err := json.NewDecoder(response.Body).Decode(&githubTokenResponse); err != nil {
		return err
	}

	reqInstallation, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/installations", http.NoBody)

	if err != nil {
		return err
	}

	reqInstallation.Header.Set("Authorization", "Bearer "+githubTokenResponse.AccessToken)
	reqInstallation.Header.Set("Accept", "application/vnd.github+json")

	responseInstallation, err := client.Do(reqInstallation)
	if err != nil {
		log.Error("Failed to perform installations fetch request", zap.Error(err))
		return err
	}

	defer func() {
		if err := responseInstallation.Body.Close(); err != nil {
			log.Error("failed to close installation response body", zap.Error(err))
		}
	}()

	if responseInstallation.StatusCode < 200 || responseInstallation.StatusCode >= 300 {
		return domain.ErrGithubInstallationFetchFailed
	}

	var githubAppInstallationList GithubInstallationList
	if err := json.NewDecoder(responseInstallation.Body).Decode(&githubAppInstallationList); err != nil {
		return err
	}

	for _, inst := range githubAppInstallationList.Installations {
		if inst.Account.Type == "User" && inst.AppID == githubAppID {
			githubAppInstallation := domain.GithubInstallation{
				UserID:         userID,
				InstallationID: inst.ID,
				AccountType:    inst.Account.Type,
				AccountLogin:   inst.Account.Login,
				Status:         domain.GithubInstallationStatusActive,
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			}
			err := g.githubRepo.StoreInstallation(ctx, &githubAppInstallation)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *githubAppUsecase) GetGithubAppInstallation(ctx context.Context, userID string) (*domain.GithubInstallation, error) {
	githubRepo, err := g.githubRepo.GetInstallationByUserID(ctx, userID)

	if err != nil {
		return nil, err
	}

	return githubRepo, nil
}

func (g *githubAppUsecase) DeleteGithubApp(ctx context.Context, userID string) error {
	return g.githubRepo.DeleteInstallationByUserID(ctx, userID)
}

func (g *githubAppUsecase) ListRepositories(ctx context.Context, userID, cursor, query string, perPage int) (*domain.RepositoryList, error) {
	installation, err := g.githubRepo.GetInstallationByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if installation.Status != domain.GithubInstallationStatusActive {
		return nil, domain.ErrInvalidStatus
	}

	normalizedQuery := strings.Join(strings.Fields(strings.ToLower(query)), " ")
	cacheKey := repositoryCacheKey(installation.InstallationID, cursor, normalizedQuery, perPage)
	if g.cache != nil {
		if cached, cacheErr := g.cache.Get(ctx, cacheKey); cacheErr == nil && cached != nil {
			return cached, nil
		}
	}

	if g.tokenProvider == nil || g.repositoryClient == nil {
		return nil, domain.ErrInternalServerError
	}
	result, err := g.listRepositoriesSingleFlight(cacheKey, func() (*domain.RepositoryList, error) {
		token, err := g.tokenProvider.CreateInstallationToken(ctx, installation.InstallationID)
		if err != nil {
			return nil, err
		}
		return g.repositoryClient.ListRepositories(ctx, token, cursor, normalizedQuery, perPage)
	})
	if err != nil {
		return nil, err
	}
	if g.cache != nil {
		ttl := time.Duration(viper.GetInt("REDIS_REPOSITORY_CACHE_TTL_SECONDS")) * time.Second
		_ = g.cache.Set(ctx, cacheKey, result, ttl)
	}
	return result, nil
}

// InvalidateRepositoryCache evicts the cached repository list for an installation.
// It is called on installation_repositories / suspend / delete / reconnect; TTL
// remains only a fallback for missed events. Redis never decides authorization.
func (g *githubAppUsecase) InvalidateRepositoryCache(ctx context.Context, installationID int64) error {
	if g.cache == nil {
		return nil
	}
	return g.cache.InvalidateInstallation(ctx, installationID)
}

func (g *githubAppUsecase) listRepositoriesSingleFlight(key string, fn func() (*domain.RepositoryList, error)) (*domain.RepositoryList, error) {
	g.repoListMu.Lock()
	if call, ok := g.repoListCalls[key]; ok {
		g.repoListMu.Unlock()
		call.wg.Wait()
		return call.result, call.err
	}
	call := &repoListCall{}
	call.wg.Add(1)
	g.repoListCalls[key] = call
	g.repoListMu.Unlock()

	call.result, call.err = fn()
	call.wg.Done()

	g.repoListMu.Lock()
	delete(g.repoListCalls, key)
	g.repoListMu.Unlock()
	return call.result, call.err
}

func (g *githubAppUsecase) GetRepositoryDetails(ctx context.Context, userID string, repoID int64) (*domain.RepositoryPicker, error) {
	installation, err := g.githubRepo.GetInstallationByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if installation.Status != domain.GithubInstallationStatusActive {
		return nil, domain.ErrInvalidStatus
	}
	if g.tokenProvider == nil || g.repositoryClient == nil {
		return nil, domain.ErrInternalServerError
	}

	token, err := g.tokenProvider.CreateInstallationToken(ctx, installation.InstallationID)
	if err != nil {
		return nil, err
	}
	return g.repositoryClient.GetRepositoryDetails(ctx, token, repoID)
}

func repositoryCacheKey(installationID int64, cursor, query string, perPage int) string {
	input := cursor + "|" + query + "|" + strconv.Itoa(perPage)
	sum := sha256.Sum256([]byte(input))
	return cache.KeyPrefix + strconv.FormatInt(installationID, 10) + ":repositories:v1:" + hex.EncodeToString(sum[:])
}
