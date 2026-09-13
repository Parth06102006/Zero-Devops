// Package http provides HTTP handlers for deployment endpoints
package http

import (
	authmiddleware "Zero_Devops/server/internal/auth/delivery/http/middleware"
	"Zero_Devops/server/internal/domain"
	"Zero_Devops/server/internal/helper"
	"Zero_Devops/server/internal/middleware"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
	"go.uber.org/zap"
)

// DeploymentHandler handles deployment HTTP requests
type DeploymentHandler struct {
	dUsecase domain.DeploymentUsecase
}

// NewDeploymentHandler creates a new deployment HTTP handler and registers routes
func NewDeploymentHandler(e *echo.Echo, du domain.DeploymentUsecase) {
	handler := &DeploymentHandler{
		dUsecase: du,
	}
	// (Legacy POST /deploy and its handler were removed 2026-09-06: the
	// repo_id-only request could not produce the immutable deploy.jobs V1
	// inputs, and the endpoint failed closed anyway. Builds are created
	// exclusively through the project build API below and webhook pushes.)
	e.POST("/projects/:id/builds", handler.CreateProjectBuild)
	e.GET("/projects/:id/builds", handler.ListProjectBuilds)
	e.GET("/builds/:id", handler.GetBuild)
}

type createProjectBuildRequest struct {
	ShaOrRef       string `json:"sha_or_ref"`
	IdempotencyKey string `json:"idempotency_key"`
}

// CreateProjectBuild handles manual project build creation requests
func (h *DeploymentHandler) CreateProjectBuild(c *echo.Context) error {
	reqID := middleware.GetRequestID(c)
	log := middleware.LoggerFromContext(c.Request().Context())

	userID, ok := authmiddleware.GetUserID(c)
	if !ok {
		log.Warn("User ID not found in context")
		return c.JSON(http.StatusUnauthorized, helper.BuildErrorResponse("user id not found", fmt.Errorf("user id not found in context"), reqID))
	}

	projectID := c.Param("id")
	if projectID == "" {
		return c.JSON(http.StatusBadRequest, helper.BuildErrorResponse("project id is required", domain.ErrBadParamInput, reqID))
	}

	var req createProjectBuildRequest
	if err := c.Bind(&req); err != nil {
		log.Warn("Invalid request body for project build")
		return c.JSON(http.StatusBadRequest, helper.BuildErrorResponse("invalid request body", err, reqID))
	}
	if req.ShaOrRef == "" || req.IdempotencyKey == "" {
		return c.JSON(http.StatusBadRequest, helper.BuildErrorResponse("sha_or_ref and idempotency_key are required", domain.ErrBadParamInput, reqID))
	}

	// The inline publisher was removed here: the usecase now durably records the
	// build plus its deploy.jobs V1 outbox event, so publishing happens through
	// the outbox dispatcher rather than directly from the handler.
	deployment, err := h.dUsecase.CreateProjectBuild(c.Request().Context(), userID, domain.CreateProjectBuildParams{
		ProjectID:      projectID,
		ShaOrRef:       req.ShaOrRef,
		IdempotencyKey: req.IdempotencyKey,
		CorrelationID:  reqID,
	})
	if err != nil {
		log.Error("Failed to create project build", zap.Error(err), zap.String("project_id", projectID), zap.String("user_id", userID))
		return handleDeploymentUsecaseError(c, err, reqID, "project not found")
	}

	return c.JSON(http.StatusCreated, helper.BuildSuccessResponse(deployment, "", reqID, helper.WithMessage("build created successfully")))
}

// ListProjectBuilds returns builds for one project, scoped to the authenticated user.
func (h *DeploymentHandler) ListProjectBuilds(c *echo.Context) error {
	reqID := middleware.GetRequestID(c)
	log := middleware.LoggerFromContext(c.Request().Context())

	userID, ok := authmiddleware.GetUserID(c)
	if !ok {
		log.Warn("User ID not found in context")
		return deploymentError(c, http.StatusUnauthorized, "user id not found", fmt.Errorf("user id not found in context"), reqID)
	}

	projectID := strings.TrimSpace(c.Param("id"))
	if projectID == "" {
		return deploymentError(c, http.StatusBadRequest, "project id is required", domain.ErrBadParamInput, reqID)
	}

	builds, err := h.dUsecase.ListProjectBuilds(c.Request().Context(), userID, projectID)
	if err != nil {
		log.Error("Failed to list project builds", zap.Error(err), zap.String("project_id", projectID), zap.String("user_id", userID))
		return handleDeploymentUsecaseError(c, err, reqID, "project not found or has no builds")
	}

	return c.JSON(http.StatusOK, helper.BuildSuccessResponse(builds, "", reqID))
}

// GetBuild returns a single build scoped to the authenticated user.
func (h *DeploymentHandler) GetBuild(c *echo.Context) error {
	reqID := middleware.GetRequestID(c)
	log := middleware.LoggerFromContext(c.Request().Context())

	userID, ok := authmiddleware.GetUserID(c)
	if !ok {
		log.Warn("User ID not found in context")
		return deploymentError(c, http.StatusUnauthorized, "user id not found", fmt.Errorf("user id not found in context"), reqID)
	}

	buildID := strings.TrimSpace(c.Param("id"))
	if buildID == "" {
		return deploymentError(c, http.StatusBadRequest, "build id is required", domain.ErrBadParamInput, reqID)
	}

	build, err := h.dUsecase.GetBuild(c.Request().Context(), userID, buildID)
	if err != nil {
		log.Error("Failed to get build", zap.Error(err), zap.String("build_id", buildID), zap.String("user_id", userID))
		return handleDeploymentUsecaseError(c, err, reqID, "build not found")
	}

	return c.JSON(http.StatusOK, helper.BuildSuccessResponse(build, "", reqID))
}

func handleDeploymentUsecaseError(c *echo.Context, err error, reqID, notFoundMessage string) error {
	switch {
	case errors.Is(err, domain.ErrBadParamInput):
		return deploymentError(c, http.StatusBadRequest, err.Error(), err, reqID)
	case errors.Is(err, domain.ErrNotFound):
		return deploymentError(c, http.StatusNotFound, notFoundMessage, err, reqID)
	case errors.Is(err, domain.ErrConflict):
		return deploymentError(c, http.StatusConflict, "build already exists for this idempotency key", err, reqID)
	default:
		return deploymentError(c, http.StatusInternalServerError, err.Error(), err, reqID)
	}
}

func deploymentError(c *echo.Context, status int, message string, err error, reqID string) error {
	resp := helper.BuildErrorResponse(message, err, reqID)
	resp.Error.Code = status
	return c.JSON(status, resp)
}

// (Legacy CreateDeployment handler removed 2026-09-06 together with the
// POST /deploy route; see NewDeploymentHandler.)
