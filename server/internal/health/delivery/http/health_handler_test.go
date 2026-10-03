package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

type mockDBPinger struct {
	pingErr error
}

func (m *mockDBPinger) PingContext(ctx context.Context) error {
	return m.pingErr
}

type mockRedisPinger struct {
	pingErr error
}

func (m *mockRedisPinger) Ping(ctx context.Context) error {
	return m.pingErr
}

type mockRabbitMQPinger struct {
	connected bool
}

func (m *mockRabbitMQPinger) IsConnected() bool {
	return m.connected
}

func TestHealthHandler_Liveness(t *testing.T) {
	e := echo.New()
	handler := NewHealthHandler(e, &mockDBPinger{pingErr: errors.New("db down")}, &mockRedisPinger{pingErr: errors.New("redis down")}, &mockRabbitMQPinger{connected: false}, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.Liveness(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["status"] != "UP" {
		t.Fatalf("expected status UP, got %s", body["status"])
	}
}

func TestHealthHandler_Readiness_Healthy(t *testing.T) {
	e := echo.New()
	handler := NewHealthHandler(e, &mockDBPinger{pingErr: nil}, &mockRedisPinger{pingErr: nil}, &mockRabbitMQPinger{connected: true}, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.Readiness(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["status"] != "READY" {
		t.Fatalf("expected status READY, got %v", body["status"])
	}
}

func TestHealthHandler_Readiness_Unhealthy(t *testing.T) {
	e := echo.New()
	handler := NewHealthHandler(e, &mockDBPinger{pingErr: errors.New("connection refused")}, &mockRedisPinger{pingErr: nil}, &mockRabbitMQPinger{connected: true}, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.Readiness(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["status"] != "UNREADY" {
		t.Fatalf("expected status UNREADY, got %v", body["status"])
	}
}

func TestHealthHandler_CompositeHealth(t *testing.T) {
	e := echo.New()
	handler := NewHealthHandler(e, &mockDBPinger{pingErr: nil}, &mockRedisPinger{pingErr: nil}, &mockRabbitMQPinger{connected: true}, "2.4.0")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.CompositeHealth(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp HealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "pass" {
		t.Fatalf("expected status pass, got %s", resp.Status)
	}
	if resp.Version != "2.4.0" {
		t.Fatalf("expected version 2.4.0, got %s", resp.Version)
	}
	if resp.Checks["database"].Status != "up" {
		t.Fatalf("expected database check up, got %s", resp.Checks["database"].Status)
	}
	if resp.Checks["redis"].Status != "up" {
		t.Fatalf("expected redis check up, got %s", resp.Checks["redis"].Status)
	}
	if resp.Checks["rabbitmq"].Status != "up" {
		t.Fatalf("expected rabbitmq check up, got %s", resp.Checks["rabbitmq"].Status)
	}
}
