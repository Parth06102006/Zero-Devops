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

	"github.com/golang-jwt/jwt/v5"
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

	// GitHub App installation callback (from GitHub redirect)
	e.GET("/github/install/callback", handler.GithubInstallCallback)

	// Uniform SCM GitHub routes
	e.GET("/integrations/scm/github/installation", handler.GetInstallationStatus)
	e.POST("/integrations/scm/github/installation", handler.Installation)
	e.DELETE("/integrations/scm/github/installation", handler.DeleteInstallation)
	e.GET("/integrations/scm/github/repositories", handler.ListRepositories)

	// Backward-compatible aliases
	e.POST("/integrations/scm/github/install", handler.Installation)
	e.POST("/integration/scm/github/install", handler.Installation)
	e.GET("/integration/scm/github", handler.GetInstallation)
	e.GET("/integration/scm/github/", handler.GetInstallation)
	e.DELETE("/integration/scm/github/delete", handler.DeleteInstallation)
	e.GET("/integrations/github/repositories", handler.ListRepositories)
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

// GithubInstallCallback handles the GitHub App installation redirect from GitHub.
// It completes the installation server-side and redirects to the frontend.
func (inst *SCMHandler) GithubInstallCallback(c *echo.Context) error {
	reqID := middleware.GetRequestID(c)
	log := middleware.LoggerFromContext(c.Request().Context())

	code := strings.TrimSpace(c.QueryParam("code"))
	installationID := c.QueryParam("installation_id")
	setupAction := c.QueryParam("setup_action")
	returnTo := c.QueryParam("return_to")
	userIDFromURL := c.QueryParam("user_id")

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

		// First try to get userID from URL parameter (passed by frontend)
		userID := userIDFromURL
		cookieValue := ""

		// Fallback: Extract userID from JWT cookie manually (since this path skips auth middleware)
		if userID == "" {
			if cookie, err := c.Cookie("access_token"); err == nil && cookie.Value != "" {
				cookieValue = cookie.Value
				secretKey := viper.GetString("JWT_SECRET")
				if secretKey != "" {
					if parsedToken, err := jwt.Parse(cookie.Value, func(t *jwt.Token) (interface{}, error) {
						if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
							return nil, fmt.Errorf("unexpected signing method")
						}
						return []byte(secretKey), nil
					}); err == nil && parsedToken.Valid {
						if claims, ok := parsedToken.Claims.(jwt.MapClaims); ok {
							if uid, ok := claims["user_id"].(string); ok {
								userID = uid
							}
						}
					} else {
						log.Warn("JWT parse failed", zap.Error(err))
					}
				} else {
					log.Warn("JWT_SECRET not configured")
				}
			} else {
				log.Warn("access_token cookie not found", zap.Error(err))
			}
		}

		if userID == "" {
			log.Warn("User ID not found in URL or cookie, redirecting to frontend for manual completion",
				zap.String("url_user_id_present", fmt.Sprintf("%v", userIDFromURL != "")),
				zap.String("cookie_present", fmt.Sprintf("%v", cookieValue != "")),
				zap.String("cookie_length", fmt.Sprintf("%d", len(cookieValue))),
			)
			callbackURL := fmt.Sprintf("%s/github/install/callback?code=%s&return_to=%s", frontendURL, url.QueryEscape(code), url.QueryEscape(returnTo))
			return c.Redirect(http.StatusTemporaryRedirect, callbackURL)
		}

		log.Info("Installing GitHub App", zap.String("user_id", userID), zap.String("source", func() string { if userIDFromURL != "" { return "url" }; return "cookie" }()))
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
