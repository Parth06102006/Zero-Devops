// Package http provides HTTP handlers for SCM integration
package http

import (
	authmiddleware "Zero_Devops/server/internal/auth/delivery/http/middleware"
	"Zero_Devops/server/internal/domain"
	"Zero_Devops/server/internal/helper"
	"Zero_Devops/server/internal/middleware"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const maxRedirects = 10

// SCMHandler handles SCM integration HTTP requests
type SCMHandler struct {
	scmUsecase domain.GithubUsecase
}

// NewSCMHandler creates a new SCM handler and registers routes
func NewSCMHandler(e *echo.Echo, gh domain.GithubUsecase) {
	handler := &SCMHandler{
		scmUsecase: gh,
	}

	// GitHub App installation start (redirects to GitHub App new installation)
	e.GET("/integrations/scm/github/install", handler.StartInstallation)

	// GitHub App installation callback (from GitHub redirect)
	e.GET("/github/install/callback", handler.GithubInstallCallback)

	// Uniform SCM GitHub routes
	e.GET("/integrations/scm/github/installation", handler.GetInstallationStatus)
	e.GET("/integrations/scm/github/installations", handler.ListInstallations)
	e.POST("/integrations/scm/github/installation", handler.Installation)
	e.PATCH("/integrations/scm/github/installation", handler.UpdateConnectionStatus)
	e.DELETE("/integrations/scm/github/installation", handler.DeleteInstallation)
	e.GET("/integrations/scm/github/repositories", handler.ListRepositories)
	e.GET("/integrations/scm/github/repositories/:id/directories", handler.ListRepositoryDirectories)

	// Backward-compatible aliases
	e.POST("/integrations/scm/github/install", handler.Installation)
	e.POST("/integration/scm/github/install", handler.Installation)
	e.GET("/integration/scm/github", handler.GetInstallation)
	e.GET("/integration/scm/github/", handler.GetInstallation)
	e.GET("/integrations/github/installations", handler.ListInstallations)
	e.PATCH("/integrations/github/installation", handler.UpdateConnectionStatus)
	e.DELETE("/integration/scm/github/delete", handler.DeleteInstallation)
	e.GET("/integrations/github/repositories", handler.ListRepositories)
	e.GET("/integrations/github/repositories/:id/directories", handler.ListRepositoryDirectories)
	e.GET("/integrations/github/installation", handler.GetInstallationStatus)
}

// Installation handles GitHub App installation callback
func (inst *SCMHandler) Installation(c *echo.Context) error {
	reqID := middleware.GetRequestID(c)
	log := middleware.LoggerFromContext(c.Request().Context())

	code := strings.TrimSpace(c.QueryParam("code"))

	if code == "" {
		log.Warn("Missing OAuth code parameter for SCM installation")
		return c.JSON(helper.GetStatusCode(domain.ErrNotFound), helper.BuildErrorResponse("Code not Present", domain.ErrNotFound, reqID))
	}

	ctx := c.Request().Context()

	userID, ok := authmiddleware.GetUserID(c)

	if !ok {
		log.Warn("User ID not found in context")
		return c.JSON(http.StatusUnauthorized, helper.BuildErrorResponse("user id not found", fmt.Errorf("user id not found in context"), reqID))
	}

	client := createProductionClient()

	err := inst.scmUsecase.InstallGithubApp(ctx, client, code, userID)

	if err != nil {
		log.Error("Failed to install GitHub app", zap.Error(err), zap.String("user_id", userID))
		return c.JSON(helper.GetStatusCode(err), helper.BuildErrorResponse(err.Error(), err, reqID))
	}

	log.Info("GitHub App installed successfully", zap.String("user_id", userID))
	return c.JSON(http.StatusOK, helper.BuildSuccessResponse(nil, "", reqID, helper.WithMessage("Github App Installed Successfully")))
}

// StartInstallation redirects the authenticated user to install the GitHub App
func (inst *SCMHandler) StartInstallation(c *echo.Context) error {
	reqID := middleware.GetRequestID(c)
	appName := viper.GetString("GITHUB_APP_NAME")
	if appName == "" {
		return c.JSON(http.StatusInternalServerError, helper.BuildErrorResponse("github app name not configured", fmt.Errorf("GITHUB_APP_NAME missing"), reqID))
	}
	installURL := fmt.Sprintf("https://github.com/apps/%s/installations/new", appName)
	return c.Redirect(http.StatusTemporaryRedirect, installURL)
}

// GithubInstallCallback handles the GitHub App installation redirect from GitHub.
// It completes the installation server-side and redirects to the frontend.
func (inst *SCMHandler) GithubInstallCallback(c *echo.Context) error {
	reqID := middleware.GetRequestID(c)
	log := middleware.LoggerFromContext(c.Request().Context())

	code := strings.TrimSpace(c.QueryParam("code"))
	installationID := c.QueryParam("installation_id")
	setupAction := c.QueryParam("setup_action")
	returnTo := c.QueryParam("return_to")

	if returnTo == "" {
		returnTo = "/projects"
	}

	frontendURL := viper.GetString("FRONTEND_URL")
	if frontendURL == "" {
		log.Error("FRONTEND_URL not configured")
		return c.JSON(http.StatusInternalServerError, helper.BuildErrorResponse("frontend not configured", fmt.Errorf("FRONTEND_URL not set"), reqID))
	}

	// Handle installation update flow (repo selection changed) - no code exchange needed
	if installationID != "" && setupAction == "update" {
		redirectURL := fmt.Sprintf("%s%s", frontendURL, returnTo)
		log.Info("GitHub App installation updated, redirecting to frontend", zap.String("redirect_url", redirectURL))
		return c.Redirect(http.StatusTemporaryRedirect, redirectURL)
	}

	// Handle initial OAuth flow - complete installation server-side
	if code != "" {
		ctx := c.Request().Context()

		userID, ok := authmiddleware.GetUserID(c)
		if !ok || userID == "" {
			log.Warn("User ID not found in session for GitHub installation callback")
			loginURL := fmt.Sprintf("%s/login?return_to=%s", frontendURL, url.QueryEscape(returnTo))
			return c.Redirect(http.StatusTemporaryRedirect, loginURL)
		}

		log.Info("Installing GitHub App", zap.String("user_id", userID))
		client := createProductionClient()

		err := inst.scmUsecase.InstallGithubApp(ctx, client, code, userID)
		if err != nil {
			log.Error("Failed to install GitHub app", zap.Error(err), zap.String("user_id", userID))
			// Redirect to frontend with error
			errorURL := fmt.Sprintf("%s/github?error=%s", frontendURL, url.QueryEscape(err.Error()))
			return c.Redirect(http.StatusTemporaryRedirect, errorURL)
		}

		log.Info("GitHub App installed successfully", zap.String("user_id", userID))
		redirectURL := fmt.Sprintf("%s%s", frontendURL, returnTo)
		return c.Redirect(http.StatusTemporaryRedirect, redirectURL)
	}

	// No code - redirect to GitHub settings page on frontend
	redirectURL := fmt.Sprintf("%s/github", frontendURL)
	log.Info("No code provided, redirecting to GitHub settings", zap.String("redirect_url", redirectURL))
	return c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

// GetInstallation returns the current user's GitHub App installation
func (inst *SCMHandler) GetInstallation(c *echo.Context) error {
	reqID := middleware.GetRequestID(c)
	log := middleware.LoggerFromContext(c.Request().Context())

	userID, ok := authmiddleware.GetUserID(c)

	if !ok {
		log.Warn("User ID not found in context")
		return c.JSON(http.StatusUnauthorized, helper.BuildErrorResponse("user id not found", fmt.Errorf("user id not found in context"), reqID))
	}

	ctx := c.Request().Context()

	installation, err := inst.scmUsecase.GetGithubAppInstallation(ctx, userID)

	if err != nil {
		log.Error("Failed to get GitHub app installation", zap.Error(err), zap.String("user_id", userID))
		return c.JSON(helper.GetStatusCode(err), helper.BuildErrorResponse(err.Error(), err, reqID))
	}

	return c.JSON(http.StatusOK, helper.BuildSuccessResponse(installation, "", reqID))
}

// GetInstallationStatus returns the current user's GitHub App installation status.
func (inst *SCMHandler) GetInstallationStatus(c *echo.Context) error {
	reqID := middleware.GetRequestID(c)
	log := middleware.LoggerFromContext(c.Request().Context())

	userID, ok := authmiddleware.GetUserID(c)
	if !ok {
		log.Warn("User ID not found in context")
		return c.JSON(http.StatusUnauthorized, helper.BuildErrorResponse("user id not found", fmt.Errorf("user id not found in context"), reqID))
	}

	installation, err := inst.scmUsecase.GetGithubAppInstallation(c.Request().Context(), userID)
	if err != nil {
		log.Error("Failed to get GitHub app installation status", zap.Error(err), zap.String("user_id", userID))
		if errors.Is(err, domain.ErrNotFound) {
			resp := helper.BuildErrorResponse("github installation not found", err, reqID)
			resp.Error.Code = http.StatusNotFound
			return c.JSON(http.StatusNotFound, resp)
		}
		resp := helper.BuildErrorResponse(err.Error(), err, reqID)
		resp.Error.Code = http.StatusInternalServerError
		return c.JSON(http.StatusInternalServerError, resp)
	}

	return c.JSON(http.StatusOK, helper.BuildSuccessResponse(installation, "", reqID))
}

// ListRepositories returns repositories available to the current user's installation
func (inst *SCMHandler) ListRepositories(c *echo.Context) error {
	reqID := middleware.GetRequestID(c)
	userID, ok := authmiddleware.GetUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, helper.BuildErrorResponse("user id not found", fmt.Errorf("user id not found in context"), reqID))
	}

	const maxRepositoriesPerPage = 100

	perPage := 30
	if raw := strings.TrimSpace(c.QueryParam("per_page")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			return c.JSON(http.StatusBadRequest, helper.BuildErrorResponse("invalid per_page", domain.ErrBadParamInput, reqID))
		}
		perPage = parsed
	}
	if perPage > maxRepositoriesPerPage {
		perPage = maxRepositoriesPerPage
	}

	result, err := inst.scmUsecase.ListRepositories(c.Request().Context(), userID, strings.TrimSpace(c.QueryParam("cursor")), c.QueryParam("query"), perPage)
	if err != nil {
		status := helper.GetStatusCode(err)
		if errors.Is(err, domain.ErrBadParamInput) {
			status = http.StatusBadRequest
		}
		if errors.Is(err, domain.ErrInvalidStatus) {
			status = http.StatusConflict
		}
		resp := helper.BuildErrorResponse(err.Error(), err, reqID)
		resp.Error.Code = status
		return c.JSON(status, resp)
	}
	return c.JSON(http.StatusOK, helper.BuildSuccessResponse(result, "", reqID))
}

// DeleteInstallation handles removal of the current user's GitHub App installation
func (inst *SCMHandler) DeleteInstallation(c *echo.Context) error {
	reqID := middleware.GetRequestID(c)
	log := middleware.LoggerFromContext(c.Request().Context())

	userID, ok := authmiddleware.GetUserID(c)

	if !ok {
		log.Warn("User ID not found in context")
		return c.JSON(http.StatusUnauthorized, helper.BuildErrorResponse("user id not found", fmt.Errorf("user id not found in context"), reqID))
	}

	ctx := c.Request().Context()

	err := inst.scmUsecase.DeleteGithubApp(ctx, userID)

	if err != nil {
		log.Error("Failed to delete GitHub app installation", zap.Error(err), zap.String("user_id", userID))
		return c.JSON(helper.GetStatusCode(err), helper.BuildErrorResponse(err.Error(), err, reqID))
	}

	log.Info("GitHub App uninstalled successfully", zap.String("user_id", userID))
	return c.JSON(http.StatusOK, helper.BuildSuccessResponse(nil, "", reqID, helper.WithMessage("GitHub App uninstalled successfully")))
}

// ListInstallations returns all saved GitHub App installations for the current user
func (inst *SCMHandler) ListInstallations(c *echo.Context) error {
	reqID := middleware.GetRequestID(c)
	log := middleware.LoggerFromContext(c.Request().Context())

	userID, ok := authmiddleware.GetUserID(c)
	if !ok {
		log.Warn("User ID not found in context")
		return c.JSON(http.StatusUnauthorized, helper.BuildErrorResponse("user id not found", fmt.Errorf("user id not found in context"), reqID))
	}

	ctx := c.Request().Context()
	installations, err := inst.scmUsecase.ListGithubAppInstallations(ctx, userID)
	if err != nil {
		log.Error("Failed to list GitHub app installations", zap.Error(err), zap.String("user_id", userID))
		status := helper.GetStatusCode(err)
		resp := helper.BuildErrorResponse(err.Error(), err, reqID)
		resp.Error.Code = status
		return c.JSON(status, resp)
	}

	if installations == nil {
		installations = []*domain.GithubInstallation{}
	}

	return c.JSON(http.StatusOK, helper.BuildSuccessResponse(installations, "", reqID))
}

type updateConnectionStatusRequest struct {
	ID               string                        `json:"id"`
	ConnectionStatus domain.GithubConnectionStatus `json:"connection_status"`
}

// UpdateConnectionStatus updates the user's connection status preference for an installation
func (inst *SCMHandler) UpdateConnectionStatus(c *echo.Context) error {
	reqID := middleware.GetRequestID(c)
	log := middleware.LoggerFromContext(c.Request().Context())

	userID, ok := authmiddleware.GetUserID(c)
	if !ok {
		log.Warn("User ID not found in context")
		return c.JSON(http.StatusUnauthorized, helper.BuildErrorResponse("user id not found", fmt.Errorf("user id not found in context"), reqID))
	}

	var req updateConnectionStatusRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, helper.BuildErrorResponse("invalid request body", domain.ErrBadParamInput, reqID))
	}

	if !req.ConnectionStatus.IsValid() {
		msg := "invalid connection_status, must be connected or disconnected"
		return c.JSON(http.StatusBadRequest, helper.BuildErrorResponse(msg, domain.ErrBadParamInput, reqID))
	}

	ctx := c.Request().Context()
	updated, err := inst.scmUsecase.UpdateConnectionStatus(ctx, userID, req.ID, req.ConnectionStatus)
	if err != nil {
		log.Error("Failed to update connection status", zap.Error(err), zap.String("user_id", userID))
		status := helper.GetStatusCode(err)
		resp := helper.BuildErrorResponse(err.Error(), err, reqID)
		resp.Error.Code = status
		return c.JSON(status, resp)
	}

	return c.JSON(http.StatusOK, helper.BuildSuccessResponse(updated, "", reqID, helper.WithMessage("connection status updated successfully")))
}

// ListRepositoryDirectories returns the directories/submodules for a given repository.
func (inst *SCMHandler) ListRepositoryDirectories(c *echo.Context) error {
	reqID := middleware.GetRequestID(c)
	userID, ok := authmiddleware.GetUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, helper.BuildErrorResponse("user id not found", fmt.Errorf("user id not found in context"), reqID))
	}

	repoIDStr := strings.TrimSpace(c.Param("id"))
	repoID, err := strconv.ParseInt(repoIDStr, 10, 64)
	if err != nil || repoID <= 0 {
		return c.JSON(http.StatusBadRequest, helper.BuildErrorResponse("invalid repository id", domain.ErrBadParamInput, reqID))
	}

	branch := strings.TrimSpace(c.QueryParam("branch"))

	dirs, err := inst.scmUsecase.ListRepositoryDirectories(c.Request().Context(), userID, repoID, branch)
	if err != nil {
		status := helper.GetStatusCode(err)
		if errors.Is(err, domain.ErrBadParamInput) {
			status = http.StatusBadRequest
		}
		if errors.Is(err, domain.ErrInvalidStatus) {
			status = http.StatusConflict
		}
		resp := helper.BuildErrorResponse(err.Error(), err, reqID)
		resp.Error.Code = status
		return c.JSON(status, resp)
	}

	return c.JSON(http.StatusOK, helper.BuildSuccessResponse(dirs, "", reqID))
}

func createProductionClient() *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
		DisableKeepAlives:   false,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return errors.New("stopped after 10 redirects")
			}
			return nil
		},
	}
}
