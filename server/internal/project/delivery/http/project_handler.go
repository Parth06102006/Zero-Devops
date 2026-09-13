// Package http provides HTTP handlers for project endpoints.
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

// ProjectHandler handles selected/configured project HTTP endpoints.
type ProjectHandler struct {
	pUsecase domain.ProjectUsecase
}

// NewProjectHandler creates a new project handler and registers authenticated
// project routes. Every route is scoped to the authenticated user by the
// handler/usecase; a project owned by a different user is returned as 404.
func NewProjectHandler(e *echo.Echo, pu domain.ProjectUsecase) {
	handler := &ProjectHandler{
		pUsecase: pu,
	}

	e.GET("/projects", handler.GetProjectList)
	e.POST("/projects", handler.CreateProject)
	e.GET("/projects/:id", handler.GetProjectByID)
	e.PATCH("/projects/:id", handler.UpdateProject)
	e.DELETE("/projects/:id", handler.DeleteProject)
}

// GetProjectList returns the authenticated user's configured projects.
func (p *ProjectHandler) GetProjectList(c *echo.Context) error {
	reqID := middleware.GetRequestID(c)
	log := middleware.LoggerFromContext(c.Request().Context())

	userID, ok := authmiddleware.GetUserID(c)
	if !ok {
		log.Warn("User ID not found in context")
		return projectError(c, http.StatusUnauthorized, "user id not found", fmt.Errorf("user id not found in context"), reqID)
	}

	projects, err := p.pUsecase.ListProjects(c.Request().Context(), userID)
	if err != nil {
		log.Error("failed to list projects", zap.Error(err), zap.String("user_id", userID))
		return handleProjectUsecaseError(c, err, reqID)
	}

	return c.JSON(http.StatusOK, helper.BuildSuccessResponse(projects, "", reqID))
}

// GetProjectByID returns one authenticated user's project by ID.
func (p *ProjectHandler) GetProjectByID(c *echo.Context) error {
	reqID := middleware.GetRequestID(c)
	log := middleware.LoggerFromContext(c.Request().Context())

	userID, ok := authmiddleware.GetUserID(c)
	if !ok {
		log.Warn("User ID not found in context")
		return projectError(c, http.StatusUnauthorized, "user id not found", fmt.Errorf("user id not found in context"), reqID)
	}

	projectID := strings.TrimSpace(c.Param("id"))
	if projectID == "" {
		return projectError(c, http.StatusBadRequest, "project id is required", domain.ErrBadParamInput, reqID)
	}

	project, err := p.pUsecase.GetProject(c.Request().Context(), userID, projectID)
	if err != nil {
		log.Error("failed to get project", zap.Error(err), zap.String("project_id", projectID), zap.String("user_id", userID))
		return handleProjectUsecaseError(c, err, reqID)
	}

	return c.JSON(http.StatusOK, helper.BuildSuccessResponse(project, "", reqID))
}

// CreateProject creates a durable selected project. The frontend supplies only
// repository_id, configured_branch, webhook enabled flag, and build command; the
// usecase verifies repository visibility with GitHub and stores the normalized
// metadata/configuration snapshot.
func (p *ProjectHandler) CreateProject(c *echo.Context) error {
	reqID := middleware.GetRequestID(c)
	log := middleware.LoggerFromContext(c.Request().Context())

	userID, ok := authmiddleware.GetUserID(c)
	if !ok {
		log.Warn("User ID not found in context")
		return projectError(c, http.StatusUnauthorized, "user id not found", fmt.Errorf("user id not found in context"), reqID)
	}

	var req domain.CreateProjectParams
	if err := c.Bind(&req); err != nil {
		log.Warn("invalid request body for project creation", zap.Error(err))
		return projectError(c, http.StatusBadRequest, "invalid request body", err, reqID)
	}

	if err := validateCreateProjectRequest(req); err != nil {
		log.Warn("invalid project creation request", zap.Error(err))
		return projectError(c, http.StatusBadRequest, err.Error(), err, reqID)
	}

	project, err := p.pUsecase.CreateProject(c.Request().Context(), userID, req)
	if err != nil {
		log.Error("failed to create project", zap.Error(err), zap.String("user_id", userID), zap.Int64("repository_id", req.RepositoryID))
		return handleProjectUsecaseError(c, err, reqID)
	}

	return c.JSON(http.StatusCreated, helper.BuildSuccessResponse(project, "", reqID, helper.WithMessage("project created successfully")))
}

// UpdateProject applies PATCH fields to a project owned by the authenticated
// user. Only non-nil fields in UpdateProjectParams are applied by the usecase.
func (p *ProjectHandler) UpdateProject(c *echo.Context) error {
	reqID := middleware.GetRequestID(c)
	log := middleware.LoggerFromContext(c.Request().Context())

	userID, ok := authmiddleware.GetUserID(c)
	if !ok {
		log.Warn("User ID not found in context")
		return projectError(c, http.StatusUnauthorized, "user id not found", fmt.Errorf("user id not found in context"), reqID)
	}

	projectID := strings.TrimSpace(c.Param("id"))
	if projectID == "" {
		return projectError(c, http.StatusBadRequest, "project id is required", domain.ErrBadParamInput, reqID)
	}

	var req domain.UpdateProjectParams
	if err := c.Bind(&req); err != nil {
		log.Warn("invalid request body for project update", zap.Error(err), zap.String("project_id", projectID))
		return projectError(c, http.StatusBadRequest, "invalid request body", err, reqID)
	}

	if err := validateUpdateProjectRequest(req); err != nil {
		log.Warn("invalid project update request", zap.Error(err), zap.String("project_id", projectID))
		return projectError(c, http.StatusBadRequest, err.Error(), err, reqID)
	}

	project, err := p.pUsecase.UpdateProject(c.Request().Context(), userID, projectID, req)
	if err != nil {
		log.Error("failed to update project", zap.Error(err), zap.String("project_id", projectID), zap.String("user_id", userID))
		return handleProjectUsecaseError(c, err, reqID)
	}

	return c.JSON(http.StatusOK, helper.BuildSuccessResponse(project, "", reqID, helper.WithMessage("project updated successfully")))
}

// DeleteProject deletes a project owned by the authenticated user.
func (p *ProjectHandler) DeleteProject(c *echo.Context) error {
	reqID := middleware.GetRequestID(c)
	log := middleware.LoggerFromContext(c.Request().Context())

	userID, ok := authmiddleware.GetUserID(c)
	if !ok {
		log.Warn("User ID not found in context")
		return projectError(c, http.StatusUnauthorized, "user id not found", fmt.Errorf("user id not found in context"), reqID)
	}

	projectID := strings.TrimSpace(c.Param("id"))
	if projectID == "" {
		return projectError(c, http.StatusBadRequest, "project id is required", domain.ErrBadParamInput, reqID)
	}

	if err := p.pUsecase.DeleteProject(c.Request().Context(), userID, projectID); err != nil {
		log.Error("failed to delete project", zap.Error(err), zap.String("project_id", projectID), zap.String("user_id", userID))
		return handleProjectUsecaseError(c, err, reqID)
	}

	return c.JSON(http.StatusOK, helper.BuildSuccessResponse(nil, "", reqID, helper.WithMessage("project deleted successfully")))
}

func validateCreateProjectRequest(req domain.CreateProjectParams) error {
	if req.RepositoryID <= 0 {
		return fmt.Errorf("repository_id is required")
	}
	if strings.TrimSpace(req.ConfiguredBranch) == "" {
		return fmt.Errorf("configured_branch is required")
	}
	return validateBuildConfiguration(req.BuildConfiguration)
}

func validateUpdateProjectRequest(req domain.UpdateProjectParams) error {
	if req.ConfiguredBranch == nil && req.ProjectWebhookEnabled == nil && req.BuildConfiguration == nil {
		return fmt.Errorf("at least one project field is required")
	}
	if req.ConfiguredBranch != nil && strings.TrimSpace(*req.ConfiguredBranch) == "" {
		return fmt.Errorf("configured_branch cannot be empty")
	}
	if req.BuildConfiguration != nil {
		return validateBuildConfiguration(*req.BuildConfiguration)
	}
	return nil
}

func validateBuildConfiguration(cfg domain.BuildConfiguration) error {
	if strings.TrimSpace(cfg.Executable) == "" {
		return fmt.Errorf("build_configuration.executable is required")
	}
	if cfg.Args == nil {
		return fmt.Errorf("build_configuration.args is required")
	}
	if strings.TrimSpace(cfg.WorkingDir) == "" {
		return fmt.Errorf("build_configuration.working_dir is required")
	}
	return nil
}

func handleProjectUsecaseError(c *echo.Context, err error, reqID string) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return projectError(c, http.StatusNotFound, "project not found", err, reqID)
	case errors.Is(err, domain.ErrConflict):
		return projectError(c, http.StatusConflict, "project already exists", err, reqID)
	case errors.Is(err, domain.ErrBadParamInput):
		return projectError(c, http.StatusBadRequest, err.Error(), err, reqID)
	case errors.Is(err, domain.ErrCommandDenied):
		return projectError(c, http.StatusUnprocessableEntity, err.Error(), err, reqID)
	default:
		return projectError(c, http.StatusInternalServerError, err.Error(), err, reqID)
	}
}

func projectError(c *echo.Context, status int, message string, err error, reqID string) error {
	resp := helper.BuildErrorResponse(message, err, reqID)
	// helper.GetStatusCode intentionally maps only a small set of domain errors;
	// keep this handler's HTTP status and response body code consistent.
	resp.Error.Code = status
	return c.JSON(status, resp)
}
