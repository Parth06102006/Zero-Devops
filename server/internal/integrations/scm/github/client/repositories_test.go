package client

import (
	"Zero_Devops/server/internal/domain"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListRepositories_ParsesAndFiltersByQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/installation/repositories") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("per_page"); got != "2" {
			t.Errorf("expected per_page=2, got %s", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"repositories": []map[string]interface{}{
				{"id": 1, "full_name": "alice/widgets", "name": "widgets", "owner": map[string]string{"login": "alice"}, "default_branch": "main", "clone_url": "https://github.com/alice/widgets.git", "private": true},
				{"id": 2, "full_name": "bob/widgets", "name": "widgets", "owner": map[string]string{"login": "bob"}, "default_branch": "main", "clone_url": "https://github.com/bob/widgets.git", "private": false},
			},
		})
	}))
	defer srv.Close()

	client := &RepositoryClient{httpClient: srv.Client(), baseURL: srv.URL}
	res, err := client.ListRepositories(context.Background(), "tok", "", "alice", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Repositories) != 1 {
		t.Fatalf("expected 1 repo after query filter, got %d", len(res.Repositories))
	}
	if res.Repositories[0].FullName != "alice/widgets" {
		t.Fatalf("unexpected filtered repo: %+v", res.Repositories[0])
	}
	if res.NextCursor == "" {
		t.Fatalf("expected non-empty next cursor when page is full")
	}
}

func TestListRepositories_InvalidCursor(t *testing.T) {
	client := &RepositoryClient{httpClient: http.DefaultClient, baseURL: githubAPIBaseURL}
	_, err := client.ListRepositories(context.Background(), "tok", "!!!not-valid-base64!!!", "", 30)
	if !errors.Is(err, domain.ErrBadParamInput) {
		t.Fatalf("expected ErrBadParamInput for invalid cursor, got %v", err)
	}
}

func TestGetRepositoryDetails_MapsResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/99" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id": 99, "full_name": "alice/widgets", "name": "widgets",
			"owner": map[string]string{"login": "alice"}, "default_branch": "main",
			"clone_url": "https://github.com/alice/widgets.git", "private": true,
		})
	}))
	defer srv.Close()

	client := &RepositoryClient{httpClient: srv.Client(), baseURL: srv.URL}
	picker, err := client.GetRepositoryDetails(context.Background(), "tok", 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if picker.ID != 99 || picker.Owner != "alice" || picker.FullName != "alice/widgets" {
		t.Fatalf("unexpected picker: %+v", picker)
	}
}

func TestGetRepositoryDetails_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := &RepositoryClient{httpClient: srv.Client(), baseURL: srv.URL}
	_, err := client.GetRepositoryDetails(context.Background(), "tok", 5)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestResolveCommit_ResolvesSHA(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/repos/alice/widgets/commits/") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"sha": "ABC123DEADBEEF00000000000000000000000000"})
	}))
	defer srv.Close()

	client := &RepositoryClient{httpClient: srv.Client(), baseURL: srv.URL}
	sha, err := client.ResolveCommit(context.Background(), "tok", "alice", "widgets", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "abc123deadbeef00000000000000000000000000"
	if sha != want {
		t.Fatalf("expected lowercase normalized sha %s, got %s", want, sha)
	}
}

func TestResolveCommit_EmptyRef(t *testing.T) {
	client := &RepositoryClient{httpClient: http.DefaultClient, baseURL: githubAPIBaseURL}
	_, err := client.ResolveCommit(context.Background(), "tok", "alice", "widgets", "  ")
	if !errors.Is(err, domain.ErrBadParamInput) {
		t.Fatalf("expected ErrBadParamInput for empty ref, got %v", err)
	}
}

func TestResolveCommit_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := &RepositoryClient{httpClient: srv.Client(), baseURL: srv.URL}
	_, err := client.ResolveCommit(context.Background(), "tok", "alice", "widgets", "abc123")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestResolveCommit_InvalidSHARejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"sha": "not-a-real-sha"})
	}))
	defer srv.Close()

	client := &RepositoryClient{httpClient: srv.Client(), baseURL: srv.URL}
	_, err := client.ResolveCommit(context.Background(), "tok", "alice", "widgets", "main")
	if err == nil {
		t.Fatalf("expected error for non-40-hex SHA")
	}
}
