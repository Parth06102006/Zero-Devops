package middleware

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

// RequestIDContextKey is the context key used to store the request ID
const RequestIDContextKey = "request_id"

// RequestIDHeader is the HTTP header used for request ID propagation
const RequestIDHeader = "X-Request-Id"

// RequestIDMiddleware propagates the incoming request ID or generates one when absent.
// It stores the ID in the Echo context and sets it on the response header before invoking the next handler.
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

// GetRequestID retrieves the request ID stored in the Echo context, or an empty string if none is available.
func GetRequestID(c *echo.Context) string {
	id, _ := c.Get(RequestIDContextKey).(string)
	return id
}
