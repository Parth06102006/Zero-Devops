// Package http provides HTTP handlers for deployment endpoints
package http

import (
	authmiddleware "Zero_Devops/server/internal/auth/delivery/http/middleware"
	"Zero_Devops/server/internal/domain"
	"Zero_Devops/server/internal/helper"
	"Zero_Devops/server/internal/middleware"
	"fmt"
	"net/http"

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
	e.POST("/deploy", handler.CreateDeployment)
	e.POST("/projects/:id/builds", handler.CreateProjectBuild)
}

type createDeploymentRequest struct {
	RepoID int64 `json:"repo_id"`
}

type createProjectBuildRequest struct {
	ShaOrRef       string `json:"sha_or_ref"`
	IdempotencyKey string `json:"idempotency_key"`
}

// CreateDeployment handles deployment creation requests
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

	deployment, err := h.dUsecase.CreateProjectBuild(c.Request().Context(), userID, domain.CreateProjectBuildParams{
		ProjectID:      projectID,
		ShaOrRef:       req.ShaOrRef,
		IdempotencyKey: req.IdempotencyKey,
		CorrelationID:  reqID,
	})
	if err != nil {
		log.Error("Failed to create project build", zap.Error(err))
		status := helper.GetStatusCode(err)
		if err == domain.ErrBadParamInput {
			status = http.StatusBadRequest
		}
		if err == domain.ErrInvalidStatus {
			status = http.StatusConflict
		}
		resp := helper.BuildErrorResponse(err.Error(), err, reqID)
		resp.Error.Code = status
		return c.JSON(status, resp)
	}

	return c.JSON(http.StatusCreated, helper.BuildSuccessResponse(deployment, "", reqID, helper.WithMessage("build created successfully")))
}

func (h *DeploymentHandler) CreateDeployment(c *echo.Context) error {
	reqID := middleware.GetRequestID(c)
	log := middleware.LoggerFromContext(c.Request().Context())

	_, ok := authmiddleware.GetUserID(c)
	if !ok {
		log.Warn("User ID not found in context")
		return c.JSON(http.StatusUnauthorized, helper.BuildErrorResponse("user id not found", fmt.Errorf("user id not found in context"), reqID))
	}

	var req createDeploymentRequest
	if err := c.Bind(&req); err != nil {
		log.Warn("Invalid request body for deployment")
		return c.JSON(http.StatusBadRequest, helper.BuildErrorResponse("invalid request body", err, reqID))
	}

	if req.RepoID == 0 {
		log.Warn("Missing repo_id in deployment request")
		return c.JSON(http.StatusBadRequest, helper.BuildErrorResponse("repo_id is required", fmt.Errorf("repo_id is required"), reqID))
	}

	// This legacy endpoint supplies only a repository ID. It cannot produce the
	// complete, immutable deploy.jobs V1 contract, so publishing here would be
	// unsafe. A project/manual-build API must supply all V1 fields first.
	return c.JSON(http.StatusConflict, helper.BuildErrorResponse(
		"legacy deploy requests are disabled: submit a complete V1 build request through the project build API",
		fmt.Errorf("deploy.jobs V1 requires project, commit, configuration, scanner, and event metadata"), reqID,
	))

}
