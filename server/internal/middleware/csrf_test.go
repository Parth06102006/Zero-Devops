package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/spf13/viper"
)

// okHandler is a simple next handler that records it was reached.
func okHandler(c *echo.Context) error {
	return c.String(http.StatusOK, "ok")
}

// setupCSRFTest sets COOKIE_DOMAIN and ALLOWED_ORIGINS for the duration of the
// test and restores them on cleanup.
func setupCSRFTest(t *testing.T, cookieDomain, allowedOrigins string) {
	t.Helper()
	viper.Set("COOKIE_DOMAIN", cookieDomain)
	viper.Set("ALLOWED_ORIGINS", allowedOrigins)
	t.Cleanup(func() {
		viper.Set("COOKIE_DOMAIN", "")
		viper.Set("ALLOWED_ORIGINS", "")
	})
}

func applyCSRF(req *http.Request) *httptest.ResponseRecorder {
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	mw := NewCSRFOriginMiddleware()
	_ = mw(okHandler)(c)
	return rec
}

// ---------------------------------------------------------------------------
// Local development: middleware must be a no-op regardless of origin.
// ---------------------------------------------------------------------------

func TestCSRF_LocalDev_Skipped(t *testing.T) {
	setupCSRFTest(t, "", "http://localhost:3000")

	req := httptest.NewRequest(http.MethodPost, "/projects", http.NoBody)
	req.Header.Set("Origin", "https://evil-site.example")

	rec := applyCSRF(req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 in local dev, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// Non-mutating methods: always pass through even in production.
// ---------------------------------------------------------------------------

func TestCSRF_GetMethod_Skipped(t *testing.T) {
	setupCSRFTest(t, ".parthgarg.me", "https://ghost.parthgarg.me")

	req := httptest.NewRequest(http.MethodGet, "/projects", http.NoBody)
	req.Header.Set("Origin", "https://evil-site.example")

	rec := applyCSRF(req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for GET in production, got %d", rec.Code)
	}
}

func TestCSRF_OptionsMethod_Skipped(t *testing.T) {
	setupCSRFTest(t, ".parthgarg.me", "https://ghost.parthgarg.me")

	req := httptest.NewRequest(http.MethodOptions, "/projects", http.NoBody)
	req.Header.Set("Origin", "https://evil-site.example")

	rec := applyCSRF(req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for OPTIONS in production, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// Production mode: allowed origin must pass.
// ---------------------------------------------------------------------------

func TestCSRF_AllowedOrigin_Passes(t *testing.T) {
	setupCSRFTest(t, ".parthgarg.me", "https://ghost.parthgarg.me,http://localhost:3000")

	for _, origin := range []string{"https://ghost.parthgarg.me", "http://localhost:3000"} {
		t.Run(origin, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/projects", http.NoBody)
			req.Header.Set("Origin", origin)

			rec := applyCSRF(req)
			if rec.Code != http.StatusOK {
				t.Errorf("expected 200 for allowed origin %q, got %d", origin, rec.Code)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Production mode: unknown origin must be blocked.
// ---------------------------------------------------------------------------

func TestCSRF_UnknownOrigin_Blocked(t *testing.T) {
	setupCSRFTest(t, ".parthgarg.me", "https://ghost.parthgarg.me")

	req := httptest.NewRequest(http.MethodPost, "/projects", http.NoBody)
	req.Header.Set("Origin", "https://evil-site.example")

	rec := applyCSRF(req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for unknown origin, got %d", rec.Code)
	}
}

func TestCSRF_DeleteUnknownOrigin_Blocked(t *testing.T) {
	setupCSRFTest(t, ".parthgarg.me", "https://ghost.parthgarg.me")

	req := httptest.NewRequest(http.MethodDelete, "/projects/1", http.NoBody)
	req.Header.Set("Origin", "https://evil-site.example")

	rec := applyCSRF(req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for DELETE with unknown origin, got %d", rec.Code)
	}
}

func TestCSRF_PatchUnknownOrigin_Blocked(t *testing.T) {
	setupCSRFTest(t, ".parthgarg.me", "https://ghost.parthgarg.me")

	req := httptest.NewRequest(http.MethodPatch, "/projects/1", http.NoBody)
	req.Header.Set("Origin", "https://evil-site.example")

	rec := applyCSRF(req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for PATCH with unknown origin, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// Referer fallback: when Origin is absent, use Referer host.
// ---------------------------------------------------------------------------

func TestCSRF_RefererFallback_AllowedPasses(t *testing.T) {
	setupCSRFTest(t, ".parthgarg.me", "https://ghost.parthgarg.me")

	req := httptest.NewRequest(http.MethodPost, "/projects", http.NoBody)
	req.Header.Set("Referer", "https://ghost.parthgarg.me/dashboard")

	rec := applyCSRF(req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 when Referer matches allowed origin, got %d", rec.Code)
	}
}

func TestCSRF_RefererFallback_UnknownBlocked(t *testing.T) {
	setupCSRFTest(t, ".parthgarg.me", "https://ghost.parthgarg.me")

	req := httptest.NewRequest(http.MethodPost, "/projects", http.NoBody)
	req.Header.Set("Referer", "https://evil-site.example/page")

	rec := applyCSRF(req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 when Referer is unknown, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// No Origin and no Referer: pass through (server-to-server calls).
// ---------------------------------------------------------------------------

func TestCSRF_NoOriginNoReferer_PassThrough(t *testing.T) {
	setupCSRFTest(t, ".parthgarg.me", "https://ghost.parthgarg.me")

	req := httptest.NewRequest(http.MethodPost, "/projects", http.NoBody)
	// No Origin, no Referer headers set.

	rec := applyCSRF(req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 when no origin headers present, got %d", rec.Code)
	}
}
