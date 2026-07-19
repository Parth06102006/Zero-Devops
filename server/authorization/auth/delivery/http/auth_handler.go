// Package http provides HTTP delivery handlers for the authentication system
package http

import (
	"Zero_Devops/server/domain"
	"Zero_Devops/server/helper"
	"Zero_Devops/server/middleware"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// AuthHandler handles authentication-related HTTP requests
type AuthHandler struct {
	AUsecase domain.AuthUsecase
}

//nolint:gosec
func writeCookie(token, cookieName string, expiryTime time.Duration) *http.Cookie {
	return &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Expires:  time.Now().Add(expiryTime),
		Secure:   viper.GetBool("IS_PRODUCTION_ENV"),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	}
}

func readCookie(c *echo.Context, cookieName string) (string, error) {
	cookie, err := c.Cookie(cookieName)

	if err != nil {
		return "", err
	}

	return cookie.Value, nil
}

// NewAuthHandler registers authentication routes on the Echo instance
func NewAuthHandler(e *echo.Echo, us domain.AuthUsecase) {
	handler := &AuthHandler{
		AUsecase: us,
	}
	e.GET("/auth/github/login", handler.Login)
	e.POST("/auth/refresh", handler.Refresh)
	e.POST("/auth/logout", handler.Logout)
	e.GET("/auth/user/me", handler.GetUser)
}

// Login handles the OAuth login callback
func (a *AuthHandler) Login(c *echo.Context) error {
	reqID := middleware.GetRequestID(c)
	log := middleware.LoggerFromContext(c.Request().Context())

	code := c.QueryParam("code")
	if code == "" {
		log.Warn("Missing OAuth code parameter")
		return c.JSON(http.StatusBadRequest, helper.BuildErrorResponse("Code is Required", fmt.Errorf("missing code query parameter"), reqID))
	}

	ctx := c.Request().Context()
	tokens, err := a.AUsecase.HandleOAuthCallback(ctx, code, "github")

	if err != nil {
		log.Error("OAuth callback failed", zap.Error(err))
		return c.JSON(helper.GetStatusCode(err), helper.BuildErrorResponse(err.Error(), err, reqID))
	}

	accessExpiry, err := strconv.Atoi(viper.GetString("ACCESS_TOKEN_EXPIRY"))
	if err != nil || accessExpiry <= 0 {
		accessExpiry = 1
	}

	refreshExpiry, err := strconv.Atoi(viper.GetString("REFRESH_TOKEN_EXPIRY"))
	if err != nil || refreshExpiry <= 0 {
		refreshExpiry = 720
	}

	accessTokenCookie := writeCookie(tokens.AccessToken, "access_token", time.Duration(accessExpiry)*time.Hour)
	refreshTokenCookie := writeCookie(tokens.RefreshToken, "refresh_token", time.Duration(refreshExpiry)*time.Hour)

	c.SetCookie(accessTokenCookie)
	c.SetCookie(refreshTokenCookie)

	log.Info("User logged in successfully")
	return c.JSON(http.StatusOK, helper.BuildSuccessResponse(nil, "", reqID, helper.WithMessage("User Logged in Successfully")))
}

// Refresh handles token refresh requests
func (a *AuthHandler) Refresh(c *echo.Context) error {
	reqID := middleware.GetRequestID(c)
	log := middleware.LoggerFromContext(c.Request().Context())

	refreshToken, err := readCookie(c, "refresh_token")

	if err != nil {
		log.Warn("Failed to read refresh token cookie", zap.Error(err))
		return c.JSON(helper.GetStatusCode(err), helper.BuildErrorResponse(err.Error(), err, reqID))
	}

	ctx := c.Request().Context()
	tokens, err := a.AUsecase.RefreshToken(ctx, refreshToken)

	if err != nil {
		log.Error("Failed to refresh token", zap.Error(err))
		return c.JSON(helper.GetStatusCode(err), helper.BuildErrorResponse(err.Error(), err, reqID))
	}

	accessExpiry, err := strconv.Atoi(viper.GetString("ACCESS_TOKEN_EXPIRY"))
	if err != nil || accessExpiry <= 0 {
		accessExpiry = 1
	}

	refreshExpiry, err := strconv.Atoi(viper.GetString("REFRESH_TOKEN_EXPIRY"))
	if err != nil || refreshExpiry <= 0 {
		refreshExpiry = 720
	}

	accessTokenCookie := writeCookie(tokens.AccessToken, "access_token", time.Duration(accessExpiry)*time.Hour)
	refreshTokenCookie := writeCookie(tokens.RefreshToken, "refresh_token", time.Duration(refreshExpiry)*time.Hour)

	c.SetCookie(accessTokenCookie)
	c.SetCookie(refreshTokenCookie)

	log.Info("User token refreshed successfully")
	return c.JSON(http.StatusOK, helper.BuildSuccessResponse(nil, "", reqID, helper.WithMessage("User Token Refreshed Successfully")))
}

// Logout handles user logout
func (a *AuthHandler) Logout(c *echo.Context) error {
	reqID := middleware.GetRequestID(c)
	log := middleware.LoggerFromContext(c.Request().Context())

	ctx := c.Request().Context()
	accessToken, err := readCookie(c, "access_token")
	if err != nil {
		log.Warn("Failed to read access token cookie on logout", zap.Error(err))
		return c.JSON(helper.GetStatusCode(err), helper.BuildErrorResponse(err.Error(), err, reqID))
	}

	err = a.AUsecase.Logout(ctx, accessToken)
	if err != nil {
		log.Error("Failed to logout user", zap.Error(err))
		return c.JSON(helper.GetStatusCode(err), helper.BuildErrorResponse(err.Error(), err, reqID))
	}

	//nolint:gosec
	accessTokenCookie := writeCookie("", "access_token", 0)
	//nolint:gosec
	refreshTokenCookie := writeCookie("", "refresh_token", 0)
	accessTokenCookie.MaxAge = -1
	refreshTokenCookie.MaxAge = -1
	c.SetCookie(accessTokenCookie)
	c.SetCookie(refreshTokenCookie)

	log.Info("User logged out successfully")
	return c.JSON(http.StatusOK, helper.BuildSuccessResponse(nil, "", reqID, helper.WithMessage("User Logged Out Successfully")))
}

// GetUser returns the current authenticated user's details
func (a *AuthHandler) GetUser(c *echo.Context) error {
	reqID := middleware.GetRequestID(c)
	log := middleware.LoggerFromContext(c.Request().Context())

	accessToken, err := readCookie(c, "access_token")

	if err != nil {
		log.Warn("Failed to read access token cookie on get user", zap.Error(err))
		return c.JSON(helper.GetStatusCode(err), helper.BuildErrorResponse(err.Error(), err, reqID))
	}
	ctx := c.Request().Context()

	userResponse, err := a.AUsecase.GetCurrentUser(ctx, accessToken)

	if err != nil {
		log.Error("Failed to get current user", zap.Error(err))
		return c.JSON(helper.GetStatusCode(err), helper.BuildErrorResponse(err.Error(), err, reqID))
	}

	return c.JSON(http.StatusOK, helper.BuildSuccessResponse(userResponse, "", reqID, helper.WithMessage("user details fetched successfully")))
}
