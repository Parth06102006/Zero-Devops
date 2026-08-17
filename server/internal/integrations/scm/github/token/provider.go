package token

import (
	"Zero_Devops/server/internal/domain"
	"context"
	"encoding/json"
	"net/http"
	"time"
	"os"
	"fmt"

	appmiddleware "Zero_Devops/server/internal/middleware"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

const jwtExpiryMinutes = 10

type installationTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

type tokenProvider struct {
	appId string
	privateKeyPath string
}

func NewInstallationTokenProvider(appId string , privateKeyPath string) domain.InstallationTokenProvider  {
	return &tokenProvider{appId,privateKeyPath}
}

func (p *tokenProvider) CreateInstallationToken(ctx context.Context , InstallationID int64) (string,error) {

	log := appmiddleware.LoggerFromContext(ctx)

	appID := p.appId
	privateKeyPath := p.privateKeyPath

	//nolint:gosec // path comes from trusted server config, not user input
	privateKeyPEM, err := os.ReadFile(privateKeyPath)
	if err != nil {
		log.Error("Failed to read GitHub App private key", zap.Error(err))
		return "", err
	}

	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		log.Error("Failed to parse GitHub App private key", zap.Error(err))
		return "", err
	}

	now := time.Now()
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iat": now.Unix(),
		"exp": now.Add(jwtExpiryMinutes * time.Minute).Unix(),
		"iss": appID,
	})

	signedJWT, err := jwtToken.SignedString(privateKey)
	if err != nil {
		log.Error("Failed to sign JWT", zap.Error(err))
		return "",err
	}

	tokenURL := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", InstallationID)
	tokenReq, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, http.NoBody)
	if err != nil {
		log.Error("Failed to create token request", zap.Error(err))
		return "",err
	}
	tokenReq.Header.Set("Authorization", "Bearer "+signedJWT)
	tokenReq.Header.Set("Accept", "application/vnd.github+json")

	tokenResp, err := http.DefaultClient.Do(tokenReq)
	if err != nil {
		log.Error("Failed to get installation token", zap.Error(err))
		return "",err
	}
	defer func() {
		if err := tokenResp.Body.Close(); err != nil {
			log.Error("failed to close token response body", zap.Error(err))
		}
	}()

	if tokenResp.StatusCode != http.StatusCreated {
		log.Error("Unexpected status from GitHub token API", zap.Int("status", tokenResp.StatusCode))
		return "",fmt.Errorf("github token API returned status %d", tokenResp.StatusCode)
	}
	
	var tokenData installationTokenResponse
	if err := json.NewDecoder(tokenResp.Body).Decode(&tokenData); err != nil {
		log.Error("Failed to decode token response", zap.Error(err))
		return "", err
	}

	return tokenData.Token,nil

}