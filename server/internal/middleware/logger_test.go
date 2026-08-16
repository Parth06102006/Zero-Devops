package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestRequestLoggerMiddleware_InjectsLogger(t *testing.T) {
	viper.Set("APP_ENV", "test")
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	base := zap.NewNop()
	handler := RequestLoggerMiddleware(base)(func(c *echo.Context) error {
		ctx := c.Request().Context()
		l := LoggerFromContext(ctx)
		if l == nil {
			t.Fatal("expected non-nil logger in context")
		}
		return nil
	})

	if err := handler(c); err != nil {
		t.Fatal(err)
	}
}

func TestLoggerFromContext_ReturnsBaseWhenNotSet(t *testing.T) {
	ctx := context.Background()
	l := LoggerFromContext(ctx)
	if l == nil {
		t.Error("expected non-nil logger, got nil")
	}
}

func TestLoggerFromContext_ReturnsLoggerFromContext(t *testing.T) {
	expected := zap.NewNop()
	ctx := context.WithValue(context.Background(), loggerCtxKey{}, expected)

	l := LoggerFromContext(ctx)
	if l != expected {
		t.Error("expected to retrieve the injected logger")
	}
}

func TestRequestLoggerMiddleware_ChainCallsNext(t *testing.T) {
	base := zap.NewNop()
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var called bool
	handler := RequestLoggerMiddleware(base)(func(_ *echo.Context) error {
		called = true
		return nil
	})

	if err := handler(c); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("expected next handler to be called")
	}
}

func TestRequestLoggerMiddleware_LogsCompletedRequest(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	base := zap.New(core)
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health", http.NoBody)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := RequestLoggerMiddleware(base)(func(c *echo.Context) error {
		return c.String(http.StatusAccepted, "ok")
	})

	if err := handler(c); err != nil {
		t.Fatal(err)
	}

	entries := recorded.FilterMessage("http request completed").All()
	if len(entries) != 1 {
		t.Fatalf("expected one completed request log, got %d", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["method"] != http.MethodGet {
		t.Errorf("expected method %q, got %v", http.MethodGet, fields["method"])
	}
	if fields["path"] != "/health" {
		t.Errorf("expected path %q, got %v", "/health", fields["path"])
	}
	if fields["status"] != int64(http.StatusAccepted) {
		t.Errorf("expected status %d, got %v", http.StatusAccepted, fields["status"])
	}
}

func TestRequestLoggerMiddleware_LogsHTTPErrorStatus(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	base := zap.New(core)
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := RequestLoggerMiddleware(base)(func(_ *echo.Context) error {
		return echo.NewHTTPError(http.StatusNotFound, "Not Found")
	})

	err := handler(c)
	if err == nil {
		t.Fatal("expected error")
	}

	entries := recorded.FilterMessage("http request failed").All()
	if len(entries) != 1 {
		t.Fatalf("expected one failed request log, got %d", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["status"] != int64(http.StatusNotFound) {
		t.Errorf("expected status %d, got %v", http.StatusNotFound, fields["status"])
	}
}
