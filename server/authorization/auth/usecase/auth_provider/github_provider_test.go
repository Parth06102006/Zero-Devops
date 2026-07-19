package AuthProvider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/oauth2"
)

func TestExchangeCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/x-www-form-urlencoded")
		_, _ = io.WriteString(w, "access_token=token-123&token_type=bearer")
	}))
	defer srv.Close()

	p := &githubProvider{config: &oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: srv.URL}}}
	token, err := p.ExchangeCode(context.Background(), "code")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if token != "token-123" {
		t.Fatalf("expected token-123, got %s", token)
	}
}

func TestGetUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got == "" {
			t.Fatal("expected authorization header")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":         99,
			"login":      "octocat",
			"email":      "octo@example.com",
			"avatar_url": "https://example.com/a.png",
		})
	}))
	defer srv.Close()

	oldTransport := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = "http"
		req.URL.Host = srv.Listener.Addr().String()
		return oldTransport.RoundTrip(req)
	})
	defer func() { http.DefaultTransport = oldTransport }()

	p := &githubProvider{config: &oauth2.Config{}}
	got, err := p.GetUser(context.Background(), "access-token")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got.Provider != "github" || got.ProviderID != 99 || got.Username != "octocat" {
		t.Fatalf("unexpected user: %+v", got)
	}
}

func TestGetUser_DecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "{")
	}))
	defer srv.Close()

	oldTransport := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = "http"
		req.URL.Host = srv.Listener.Addr().String()
		return oldTransport.RoundTrip(req)
	})
	defer func() { http.DefaultTransport = oldTransport }()

	p := &githubProvider{config: &oauth2.Config{}}
	_, err := p.GetUser(context.Background(), "access-token")
	if err == nil {
		t.Fatal("expected error")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
