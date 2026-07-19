package http

import (
	authmiddleware "Zero_Devops/server/authorization/auth/delivery/http/middleware"
	"Zero_Devops/server/domain"
	"Zero_Devops/server/helper"
	"Zero_Devops/server/middleware"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"go.uber.org/zap"
)

type SCMHandler struct {
	scmUsecase domain.GithubUsecase
}

func NewSCMHandler(e *echo.Echo, gh domain.GithubUsecase) {
	handler := &SCMHandler{
		scmUsecase: gh,
	}
	// e.POST("/integration/scm/github/webhook", handler.HandleWebhook)
	e.POST("/integration/scm/github/install", handler.Installation)
	e.GET("/integration/scm/github/", handler.GetInstallation)
	e.DELETE("/integration/scm/github/delete", handler.DeleteInstallation)
}

func (inst *SCMHandler) Installation(c *echo.Context) error {
	/*
		I need to add the Github Installtion here since I need to install the github app for now in order to do it how can i perform it
	*/
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
		log.Error("Failed to install GitHub app", zap.Error(err), zap.Int64("user_id", userID))
		return c.JSON(helper.GetStatusCode(err), helper.BuildErrorResponse(err.Error(), err, reqID))
	}

	log.Info("GitHub App installed successfully", zap.Int64("user_id", userID))
	return c.JSON(http.StatusOK, helper.BuildSuccessResponse(nil, "", reqID, helper.WithMessage("Github App Installed Successfully")))
}

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
		log.Error("Failed to get GitHub app installation", zap.Error(err), zap.Int64("user_id", userID))
		return c.JSON(helper.GetStatusCode(err), helper.BuildErrorResponse(err.Error(), err, reqID))
	}

	return c.JSON(http.StatusOK, helper.BuildSuccessResponse(installation, "", reqID))
}

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
		log.Error("Failed to delete GitHub app installation", zap.Error(err), zap.Int64("user_id", userID))
		return c.JSON(helper.GetStatusCode(err), helper.BuildErrorResponse(err.Error(), err, reqID))
	}

	log.Info("GitHub App uninstalled successfully", zap.Int64("user_id", userID))
	return c.JSON(http.StatusOK, helper.BuildSuccessResponse(nil, "", reqID, helper.WithMessage("GitHub App uninstalled successfully")))
}

// Production HTTP client with advanced configuration
func createProductionClient() *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        100,              // Maximum idle connections
		MaxIdleConnsPerHost: 10,               // Maximum idle connections per host
		IdleConnTimeout:     90 * time.Second, // Idle connection timeout
		DisableCompression:  false,            // Enable compression
		DisableKeepAlives:   false,            // Enable keep-alives
	}

	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Custom redirect handling
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			return nil
		},
	}
}
