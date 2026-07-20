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
}

type createDeploymentRequest struct {
	RepoID int64 `json:"repo_id"`
}

// CreateDeployment handles deployment creation requests
func (h *DeploymentHandler) CreateDeployment(c *echo.Context) error {
	reqID := middleware.GetRequestID(c)
	log := middleware.LoggerFromContext(c.Request().Context())

	userID, ok := authmiddleware.GetUserID(c)
	if !ok {
		log.Warn("User ID not found in context")
		return c.JSON(http.StatusUnauthorized, helper.BuildErrorResponse("user id not found", fmt.Errorf("user id not found in context"), reqID))
	}

	var req createDeploymentRequest
	if err := c.Bind(&req); err != nil {
		log.Warn("Invalid request body for deployment", zap.Error(err))
		return c.JSON(http.StatusBadRequest, helper.BuildErrorResponse("invalid request body", err, reqID))
	}

	if req.RepoID == 0 {
		log.Warn("Missing repo_id in deployment request")
		return c.JSON(http.StatusBadRequest, helper.BuildErrorResponse("repo_id is required", fmt.Errorf("repo_id is required"), reqID))
	}

	ctx := c.Request().Context()
	deployment, err := h.dUsecase.CreateDeployment(ctx, userID, req.RepoID, reqID)
	if err != nil {
		log.Error("Failed to create deployment", zap.Error(err), zap.String("user_id", userID), zap.Int64("repo_id", req.RepoID))
		return c.JSON(helper.GetStatusCode(err), helper.BuildErrorResponse(err.Error(), err, reqID))
	}

	log.Info("Deployment created successfully", zap.String("deployment_id", deployment.ID))
	return c.JSON(http.StatusCreated, helper.BuildSuccessResponse(deployment, "Deployment created successfully", reqID))
}
