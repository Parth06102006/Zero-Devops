package helper

import (
	"Zero_Devops/server/internal/domain"
	"errors"
	"net/http"
	"testing"

	"github.com/spf13/viper"
)

func TestGetStatusCode_NilError(t *testing.T) {
	if code := GetStatusCode(nil); code != http.StatusOK {
		t.Errorf("expected 200, got %d", code)
	}
}

func TestGetStatusCode_InternalServerError(t *testing.T) {
	if code := GetStatusCode(domain.ErrInternalServerError); code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", code)
	}
}

func TestGetStatusCode_NotFound(t *testing.T) {
	if code := GetStatusCode(domain.ErrNotFound); code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", code)
	}
}

func TestGetStatusCode_Conflict(t *testing.T) {
	if code := GetStatusCode(domain.ErrConflict); code != http.StatusConflict {
		t.Errorf("expected 409, got %d", code)
	}
}

func TestGetStatusCode_GenericError(t *testing.T) {
	if code := GetStatusCode(errors.New("something else")); code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", code)
	}
}

func TestGetStatusCode_WebhookErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"missing event header", domain.ErrMissingGithubEventHeader, http.StatusBadRequest},
		{"missing delivery header", domain.ErrMissingGithubDeliveryHeader, http.StatusBadRequest},
		{"unsupported event", domain.ErrEventNotFound, http.StatusAccepted},
		{"missing signature", domain.ErrMissingHubSignatureHeader, http.StatusUnauthorized},
		{"bad signature", domain.ErrHMACVerificationFailed, http.StatusUnauthorized},
		{"payload too large", domain.ErrPayloadTooLarge, http.StatusRequestEntityTooLarge},
		{"malformed json", domain.ErrParsingPayload, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if code := GetStatusCode(tt.err); code != tt.want {
				t.Errorf("expected %d, got %d", tt.want, code)
			}
		})
	}
}

func TestBuildErrorResponse_Production(t *testing.T) {
	viper.Set("APP_ENV", "production")

	resp := BuildErrorResponse("something went wrong", domain.ErrNotFound, "req-123")
	if resp.Success {
		t.Error("expected success to be false")
	}
	if resp.Error.Code != http.StatusNotFound {
		t.Errorf("expected code 404, got %d", resp.Error.Code)
	}
	if resp.Error.Message != "something went wrong" {
		t.Errorf("expected message 'something went wrong', got %s", resp.Error.Message)
	}
	if resp.RequestID != "req-123" {
		t.Errorf("expected requestID 'req-123', got %s", resp.RequestID)
	}
	if resp.Error.Debug != nil {
		t.Error("expected debug to be nil in production")
	}
}

func TestBuildErrorResponse_NonProduction(t *testing.T) {
	viper.Set("APP_ENV", "development")

	resp := BuildErrorResponse("test error", domain.ErrNotFound, "req-456")
	if resp.Error.Debug == nil {
		t.Fatal("expected debug to be non-nil in non-production")
	}
	if resp.Error.Debug.RawError != domain.ErrNotFound.Error() {
		t.Errorf("expected raw error '%s', got '%s'", domain.ErrNotFound.Error(), resp.Error.Debug.RawError)
	}
	if resp.Error.Debug.Stack == "" {
		t.Error("expected stack trace to be non-empty")
	}
}

func TestBuildErrorResponse_WithReason(t *testing.T) {
	viper.Set("APP_ENV", "development")

	resp := BuildErrorResponse("test", domain.ErrConflict, "req-789", WithReason("duplicate entry"))
	if resp.Error.Debug == nil {
		t.Fatal("expected debug to be non-nil")
	}
	if resp.Error.Debug.Reason != "duplicate entry" {
		t.Errorf("expected reason 'duplicate entry', got '%s'", resp.Error.Debug.Reason)
	}
}

func TestBuildErrorResponse_WithQuery(t *testing.T) {
	viper.Set("APP_ENV", "development")

	resp := BuildErrorResponse("query failed", domain.ErrInternalServerError, "req-000", WithQuery("SELECT * FROM users"))
	if resp.Error.Debug == nil {
		t.Fatal("expected debug to be non-nil")
	}
	if resp.Error.Debug.Query != "SELECT * FROM users" {
		t.Errorf("expected query 'SELECT * FROM users', got '%s'", resp.Error.Debug.Query)
	}
}

func TestBuildErrorResponse_MultipleOptions(t *testing.T) {
	viper.Set("APP_ENV", "development")

	resp := BuildErrorResponse("multi", domain.ErrNotFound, "req-multi", WithReason("reason"), WithQuery("query"))
	if resp.Error.Debug.Reason != "reason" {
		t.Errorf("expected reason 'reason', got '%s'", resp.Error.Debug.Reason)
	}
	if resp.Error.Debug.Query != "query" {
		t.Errorf("expected query 'query', got '%s'", resp.Error.Debug.Query)
	}
}
