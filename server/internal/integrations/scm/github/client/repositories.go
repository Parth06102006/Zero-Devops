package client

import (
	"Zero_Devops/server/internal/domain"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	maxPerPage      = 100
	maxResponseSize = 2 << 20
)

type RepositoryClient struct {
	httpClient *http.Client
	baseURL    string
}

func NewRepositoryClient(httpClient *http.Client) *RepositoryClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &RepositoryClient{httpClient: httpClient, baseURL: "https://api.github.com"}
}

func (c *RepositoryClient) ListRepositories(ctx context.Context, installationToken string, cursor string, query string, perPage int) (*domain.RepositoryList, error) {
	if perPage < 1 {
		perPage = 30
	}
	if perPage > maxPerPage {
		perPage = maxPerPage
	}

	page, err := decodeCursor(cursor)
	if err != nil {
		return nil, domain.ErrBadParamInput
	}

	params := url.Values{}
	params.Set("per_page", strconv.Itoa(perPage))
	params.Set("page", strconv.Itoa(page))
	normalizedQuery := normalizeQuery(query)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/installation/repositories?"+params.Encode(), http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+installationToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github repositories API returned status %d", resp.StatusCode)
	}

	var payload struct {
		Repositories []struct {
			ID            int64  `json:"id"`
			FullName      string `json:"full_name"`
			Name          string `json:"name"`
			DefaultBranch string `json:"default_branch"`
			CloneURL      string `json:"clone_url"`
			Private       bool   `json:"private"`
			Owner         struct {
				Login string `json:"login"`
			} `json:"owner"`
		} `json:"repositories"`
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxResponseSize))
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}

	result := &domain.RepositoryList{Repositories: make([]domain.RepositoryPicker, 0, len(payload.Repositories))}
	for _, repo := range payload.Repositories {
		picker := domain.RepositoryPicker{
			ID: repo.ID, Owner: repo.Owner.Login, Name: repo.Name, FullName: repo.FullName,
			DefaultBranch: repo.DefaultBranch, CloneURL: repo.CloneURL, Private: repo.Private,
		}
		if normalizedQuery != "" && !repositoryMatchesQuery(picker, normalizedQuery) {
			continue
		}
		result.Repositories = append(result.Repositories, picker)
	}
	if len(payload.Repositories) == perPage {
		result.NextCursor = encodeCursor(page + 1)
	}
	return result, nil
}

func (c *RepositoryClient) GetRepositoryDetails(ctx context.Context, installationToken string, repoID int64) (*domain.RepositoryPicker, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/repositories/%d", c.baseURL, repoID), http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+installationToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, domain.ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github repositories API returned status %d", resp.StatusCode)
	}

	var payload struct {
		ID            int64  `json:"id"`
		FullName      string `json:"full_name"`
		Name          string `json:"name"`
		DefaultBranch string `json:"default_branch"`
		CloneURL      string `json:"clone_url"`
		Private       bool   `json:"private"`
		Owner         struct {
			Login string `json:"login"`
		} `json:"owner"`
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxResponseSize))
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}

	return &domain.RepositoryPicker{
		ID: payload.ID, Owner: payload.Owner.Login, Name: payload.Name,
		FullName: payload.FullName, DefaultBranch: payload.DefaultBranch,
		CloneURL: payload.CloneURL, Private: payload.Private,
	}, nil
}

func (c *RepositoryClient) ResolveCommit(ctx context.Context, installationToken, owner, repo, shaOrRef string) (string, error) {
	shaOrRef = strings.TrimSpace(shaOrRef)
	if shaOrRef == "" {
		return "", domain.ErrBadParamInput
	}

	path := fmt.Sprintf("%s/repos/%s/%s/commits/%s",
		c.baseURL,
		url.PathEscape(owner),
		url.PathEscape(repo),
		url.PathEscape(shaOrRef),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, http.NoBody)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+installationToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", domain.ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("github commit API returned status %d", resp.StatusCode)
	}

	var payload struct {
		SHA string `json:"sha"`
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxResponseSize))
	if err := decoder.Decode(&payload); err != nil {
		return "", err
	}

	sha := strings.ToLower(strings.TrimSpace(payload.SHA))
	if len(sha) != 40 || strings.Trim(sha, "0123456789abcdef") != "" {
		return "", fmt.Errorf("github commit API returned invalid SHA")
	}
	return sha, nil
}

func normalizeQuery(query string) string {
	return strings.Join(strings.Fields(strings.ToLower(query)), " ")
}

func repositoryMatchesQuery(repo domain.RepositoryPicker, normalizedQuery string) bool {
	haystack := strings.ToLower(repo.FullName + " " + repo.Owner + " " + repo.Name)
	for _, term := range strings.Fields(normalizedQuery) {
		if !strings.Contains(haystack, term) {
			return false
		}
	}
	return true
}

func encodeCursor(page int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(page)))
}

func decodeCursor(cursor string) (int, error) {
	if cursor == "" {
		return 1, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, err
	}
	page, err := strconv.Atoi(string(decoded))
	if err != nil || page < 1 {
		return 0, fmt.Errorf("invalid cursor")
	}
	return page, nil
}
