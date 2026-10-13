package http

import (
	"Zero_Devops/server/internal/deployments/credentials"
	"Zero_Devops/server/internal/domain"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
)

type WorkerCredentialHandler struct {
	credentials credentials.Usecase
}

func NewWorkerCredentialHandler(e *echo.Echo, usecase credentials.Usecase, workerAuth echo.MiddlewareFunc) {
	h := &WorkerCredentialHandler{credentials: usecase}
	group := e.Group("/internal", workerAuth)
	group.POST("/deployments/:id/clone-credential", h.GetCloneCredential)
}

func (h *WorkerCredentialHandler) GetCloneCredential(c *echo.Context) error {
	credential, err := h.credentials.GetCloneCredential(c.Request().Context(), strings.TrimSpace(c.Param("id")))
	if err != nil {
		switch err {
		case domain.ErrBadParamInput:
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "deployment id is required"})
		case domain.ErrNotFound:
			return c.JSON(http.StatusNotFound, map[string]string{"error": "deployment not found"})
		case domain.ErrInvalidStatus:
			return c.JSON(http.StatusForbidden, map[string]string{"error": "clone credential unavailable"})
		default:
			return c.JSON(http.StatusBadGateway, map[string]string{"error": "could not create clone credential"})
		}
	}
	// This response is intentionally the only place the installation token exists.
	return c.JSON(http.StatusOK, map[string]string{"username": credential.Username, "token": credential.Token})
}
