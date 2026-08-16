package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/spf13/viper"
)

func resetViperForCORS() {
	viper.Reset()
}

func TestCORS_AllowsAnyConfiguredOrigin(t *testing.T) {
	resetViperForCORS()
	viper.Set("ALLOWED_ORIGINS", []string{"https://example.com"})

	e := echo.New()
	e.Use(NewCORS())
	e.GET("/test", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if rec.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("expected Access-Control-Allow-Origin 'https://example.com', got '%s'", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORS_WithSpecificOrigins_Allowed(t *testing.T) {
	resetViperForCORS()
	viper.Set("ALLOWED_ORIGINS", []string{"https://myapp.com"})

	e := echo.New()
	e.Use(NewCORS())
	e.GET("/test", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Origin", "https://myapp.com")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if rec.Header().Get("Access-Control-Allow-Origin") != "https://myapp.com" {
		t.Errorf("expected Access-Control-Allow-Origin 'https://myapp.com', got '%s'", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORS_WithSpecificOrigins_Denied(t *testing.T) {
	resetViperForCORS()
	viper.Set("ALLOWED_ORIGINS", []string{"https://myapp.com"})

	e := echo.New()
	e.Use(NewCORS())
	e.GET("/test", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Origin", "https://evil.com")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") == "https://evil.com" {
		t.Error("expected non-matching origin to be denied")
	}
}

func TestCORS_AllowCredentials(t *testing.T) {
	resetViperForCORS()
	viper.Set("ALLOWED_ORIGINS", []string{"https://myapp.com"})

	e := echo.New()
	e.Use(NewCORS())
	e.GET("/test", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Origin", "https://myapp.com")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("expected Access-Control-Allow-Credentials 'true', got '%s'", rec.Header().Get("Access-Control-Allow-Credentials"))
	}
}

func TestCORS_AllowsConfiguredMethods(t *testing.T) {
	resetViperForCORS()
	viper.Set("ALLOWED_ORIGINS", []string{"https://myapp.com"})

	e := echo.New()
	e.Use(NewCORS())
	e.POST("/test", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodOptions, "/test", http.NoBody)
	req.Header.Set("Origin", "https://myapp.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	methods := rec.Header().Get("Access-Control-Allow-Methods")
	if methods == "" {
		t.Error("expected Access-Control-Allow-Methods header in preflight response")
	}
}

func TestCORS_IgnoresBlankOrigins(t *testing.T) {
	resetViperForCORS()
	viper.Set("ALLOWED_ORIGINS", " , https://myapp.com, ")

	e := echo.New()
	e.Use(NewCORS())
	e.GET("/test", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Origin", "https://myapp.com")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "https://myapp.com" {
		t.Errorf("expected Access-Control-Allow-Origin 'https://myapp.com', got '%s'", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}
