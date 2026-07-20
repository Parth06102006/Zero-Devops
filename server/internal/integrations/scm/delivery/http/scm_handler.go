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
	"strings"
	"time"

	"github.com/labstack/echo/v5"
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
	e.POST("/integration/scm/github/install", handler.Installation)
	e.GET("/integration/scm/github/", handler.GetInstallation)
	e.DELETE("/integration/scm/github/delete", handler.DeleteInstallation)
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

// DeleteInstallation removes the current user's GitHub App installation
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
