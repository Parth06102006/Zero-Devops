package http

import (
	"Zero_Devops/server/internal/deployments/credentials"
	"Zero_Devops/server/internal/domain"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
)

const errKey = "error"

// WorkerCredentialHandler exposes control-plane endpoints for worker credentials.
type WorkerCredentialHandler struct {
	credentials credentials.Usecase
}

// NewWorkerCredentialHandler registers internal deployment credential routes.
func NewWorkerCredentialHandler(e *echo.Echo, usecase credentials.Usecase, workerAuth echo.MiddlewareFunc) {
	h := &WorkerCredentialHandler{credentials: usecase}
	group := e.Group("/internal", workerAuth)
	group.POST("/deployments/:id/clone-credential", h.GetCloneCredential)
}

// GetCloneCredential returns short-lived clone credentials for a deployment.
func (h *WorkerCredentialHandler) GetCloneCredential(c *echo.Context) error {
	credential, err := h.credentials.GetCloneCredential(c.Request().Context(), strings.TrimSpace(c.Param("id")))
	if err != nil {
		switch err {
		case domain.ErrBadParamInput:
			return c.JSON(http.StatusBadRequest, map[string]string{errKey: "deployment id is required"})
		case domain.ErrNotFound:
			return c.JSON(http.StatusNotFound, map[string]string{errKey: "deployment not found"})
		case domain.ErrInvalidStatus:
			return c.JSON(http.StatusForbidden, map[string]string{errKey: "clone credential unavailable"})
		default:
			return c.JSON(http.StatusBadGateway, map[string]string{errKey: "could not create clone credential"})
		}
	}
	// This response is intentionally the only place the installation token exists.
	return c.JSON(http.StatusOK, map[string]string{"username": credential.Username, "token": credential.Token})
}
