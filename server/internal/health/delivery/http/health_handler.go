// Package http provides HTTP delivery handlers for health and readiness checks.
package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/spf13/viper"
)

const checkTimeout = 2 * time.Second

// DBPinger abstracts database pinging for health checks.
type DBPinger interface {
	PingContext(ctx context.Context) error
}

// RedisPinger abstracts Redis pinging for health checks.
type RedisPinger interface {
	Ping(ctx context.Context) error
}

// RabbitMQPinger abstracts RabbitMQ connection checking for health checks.
type RabbitMQPinger interface {
	IsConnected() bool
}

// ComponentHealth represents health information for a single subsystem.
type ComponentHealth struct {
	Status  string `json:"status"`
	Details string `json:"details,omitempty"`
	Latency string `json:"latency,omitempty"`
}

// HealthResponse conforms to the IETF health check format.
type HealthResponse struct {
	Status    string                     `json:"status"`
	Timestamp string                     `json:"timestamp"`
	Version   string                     `json:"version,omitempty"`
	Checks    map[string]ComponentHealth `json:"checks,omitempty"`
}

// HealthHandler manages liveness, readiness, and composite health endpoints.
type HealthHandler struct {
	db       DBPinger
	redis    RedisPinger
	rabbitmq RabbitMQPinger
	version  string
}

// NewHealthHandler creates and registers health check routes on the Echo instance.
func NewHealthHandler(e *echo.Echo, db DBPinger, redis RedisPinger, rabbitmq RabbitMQPinger, version string) *HealthHandler {
	handler := &HealthHandler{
		db:       db,
		redis:    redis,
		rabbitmq: rabbitmq,
		version:  version,
	}

	e.GET("/health", handler.CompositeHealth)
	e.GET("/health/live", handler.Liveness)
	e.GET("/health/ready", handler.Readiness)

	return handler
}

// Liveness reports whether the application process is running.
// It never fails due to external dependencies to avoid unnecessary container restart loops.
func (h *HealthHandler) Liveness(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status":    "UP",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// Readiness reports whether the instance is ready to receive traffic by verifying essential downstream dependencies.
func (h *HealthHandler) Readiness(c *echo.Context) error {
	checks, allHealthy := h.runChecks(c.Request().Context())

	status := "READY"
	httpStatus := http.StatusOK
	if !allHealthy {
		status = "UNREADY"
		httpStatus = http.StatusServiceUnavailable
	}

	return c.JSON(httpStatus, map[string]interface{}{
		"status":    status,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"checks":    checks,
	})
}

// CompositeHealth returns an in-depth diagnostic report conforming to IETF HTTP Health Check standards.
func (h *HealthHandler) CompositeHealth(c *echo.Context) error {
	checks, allHealthy := h.runChecks(c.Request().Context())

	status := "pass"
	httpStatus := http.StatusOK
	if !allHealthy {
		status = "fail"
		httpStatus = http.StatusServiceUnavailable
	}

	appVersion := h.version
	if appVersion == "" {
		appVersion = viper.GetString("APP_VERSION")
		if appVersion == "" {
			appVersion = "1.0.0"
		}
	}

	resp := HealthResponse{
		Status:    status,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Version:   appVersion,
		Checks:    checks,
	}

	return c.JSON(httpStatus, resp)
}

func (h *HealthHandler) runChecks(parentCtx context.Context) (map[string]ComponentHealth, bool) {
	checks := make(map[string]ComponentHealth)
	allHealthy := true

	// Check Database
	if h.db != nil {
		ctx, cancel := context.WithTimeout(parentCtx, checkTimeout)
		start := time.Now()
		err := h.db.PingContext(ctx)
		cancel()

		latency := time.Since(start).Round(time.Millisecond).String()
		if err != nil {
			allHealthy = false
			checks["database"] = ComponentHealth{
				Status:  "down",
				Details: "database ping failed",
				Latency: latency,
			}
		} else {
			checks["database"] = ComponentHealth{
				Status:  "up",
				Latency: latency,
			}
		}
	}

	// Check Redis
	if h.redis != nil {
		ctx, cancel := context.WithTimeout(parentCtx, checkTimeout)
		start := time.Now()
		err := h.redis.Ping(ctx)
		cancel()

		latency := time.Since(start).Round(time.Millisecond).String()
		if err != nil {
			// Redis failure is logged as down; depending on requirements it could be a warning,
			// but we mark unready if caching is required.
			allHealthy = false
			checks["redis"] = ComponentHealth{
				Status:  "down",
				Details: "redis ping failed",
				Latency: latency,
			}
		} else {
			checks["redis"] = ComponentHealth{
				Status:  "up",
				Latency: latency,
			}
		}
	}

	// Check RabbitMQ
	if h.rabbitmq != nil {
		if !h.rabbitmq.IsConnected() {
			allHealthy = false
			checks["rabbitmq"] = ComponentHealth{
				Status:  "down",
				Details: "rabbitmq connection is closed",
			}
		} else {
			checks["rabbitmq"] = ComponentHealth{
				Status: "up",
			}
		}
	}

	return checks, allHealthy
}

// RedisWrapper adapts redis.Client to the RedisPinger interface.
type RedisWrapper struct {
	Client interface {
		Ping(ctx context.Context) error
	}
}

// AMQPWrapper adapts amqp.Connection to the RabbitMQPinger interface.
type AMQPWrapper struct {
	Connection interface {
		IsClosed() bool
	}
}

// IsConnected returns true if the RabbitMQ connection is active.
func (w AMQPWrapper) IsConnected() bool {
	if w.Connection == nil {
		return false
	}
	return !w.Connection.IsClosed()
}

// RedisClientWrapper wraps a real *redis.Client for RedisPinger.
type RedisClientWrapper struct {
	PingFunc func(ctx context.Context) error
}

// Ping executes the ping function.
func (w RedisClientWrapper) Ping(ctx context.Context) error {
	if w.PingFunc == nil {
		return fmt.Errorf("nil ping function")
	}
	return w.PingFunc(ctx)
}
