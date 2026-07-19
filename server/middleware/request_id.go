package middleware

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

// RequestIDContextKey is the context key used to store the request ID
const RequestIDContextKey = "request_id"

// RequestIDHeader is the HTTP header used for request ID propagation
const RequestIDHeader = "X-Request-Id"

// RequestIDMiddleware is a middleware that assigns a unique request ID to each incoming request
func RequestIDMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		id := c.Request().Header.Get(RequestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}
		c.Set(RequestIDContextKey, id)
		c.Response().Header().Set(RequestIDHeader, id)
		return next(c)
	}
}

// GetRequestID retrieves the request ID from the echo context
func GetRequestID(c *echo.Context) string {
	id, _ := c.Get(RequestIDContextKey).(string)
	return id
}
