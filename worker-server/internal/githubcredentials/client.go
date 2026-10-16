// Package githubcredentials obtains short-lived clone credentials from the trusted control plane.
package githubcredentials

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"Zero_Devops/worker_server/internal/domain"
)

type client struct {
	serverURL  string
	authToken  string
	httpClient *http.Client
}

// New creates a new CloneCredentialProvider for obtaining short-lived clone credentials.
func New(serverURL, authToken string) (domain.CloneCredentialProvider, error) {
	if strings.TrimSpace(serverURL) == "" || strings.TrimSpace(authToken) == "" {
		return nil, errors.New("SERVER_INTERNAL_URL and WORKER_INTERNAL_AUTH_TOKEN are required")
	}
	parsed, err := url.Parse(serverURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid SERVER_INTERNAL_URL")
	}
	return &client{
		serverURL:  strings.TrimRight(serverURL, "/"),
		authToken:  authToken,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func (c *client) GetCloneCredential(ctx context.Context, deploymentID string) (domain.CloneCredential, error) {
	endpoint := c.serverURL + "/internal/deployments/" + url.PathEscape(deploymentID) + "/clone-credential"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, http.NoBody)
	if err != nil {
		return domain.CloneCredential{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.authToken)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.CloneCredential{}, fmt.Errorf("request clone credential: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return domain.CloneCredential{}, fmt.Errorf("clone credential endpoint returned %s", resp.Status)
	}
	var credential domain.CloneCredential
	if err := json.NewDecoder(resp.Body).Decode(&credential); err != nil {
		return domain.CloneCredential{}, fmt.Errorf("decode clone credential: %w", err)
	}
	if credential.Username != "x-access-token" || credential.Token == "" {
		return domain.CloneCredential{}, errors.New("invalid clone credential response")
	}
	return credential, nil
}
