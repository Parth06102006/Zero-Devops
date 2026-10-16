package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestCleanPathMiddleware(t *testing.T) {
	tests := []struct {
		name         string
		requestPath  string
		expectedPath string
	}{
		{
			name:         "Double leading slash",
			requestPath:  "//webhooks/github",
			expectedPath: "/webhooks/github",
		},
		{
			name:         "Triple leading slash",
			requestPath:  "///webhooks/github",
			expectedPath: "/webhooks/github",
		},
		{
			name:         "Middle duplicate slashes",
			requestPath:  "/auth//refresh",
			expectedPath: "/auth/refresh",
		},
		{
			name:         "Normal path untouched",
			requestPath:  "/webhooks/github",
			expectedPath: "/webhooks/github",
		},
		{
			name:         "Trailing slash preserved",
			requestPath:  "//integrations/scm/github/",
			expectedPath: "/integrations/scm/github/",
		},
		{
			name:         "Root path untouched",
			requestPath:  "/",
			expectedPath: "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			e.Pre(CleanPathMiddleware)

			var capturedPath string
			e.Any("/*", func(c *echo.Context) error {
				capturedPath = c.Request().URL.Path
				return c.String(http.StatusOK, "ok")
			})

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, tt.requestPath, http.NoBody)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if capturedPath != tt.expectedPath {
				t.Errorf("CleanPathMiddleware for %q: expected URL.Path %q, got %q", tt.requestPath, tt.expectedPath, capturedPath)
			}
		})
	}
}

func TestCleanPathMiddleware_RouteMatching(t *testing.T) {
	e := echo.New()
	e.Pre(CleanPathMiddleware)

	handlerCalled := false
	e.POST("/webhooks/github", func(c *echo.Context) error {
		handlerCalled = true
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "//webhooks/github", http.NoBody)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if !handlerCalled {
		t.Error("expected /webhooks/github handler to be called for //webhooks/github request")
	}
}
