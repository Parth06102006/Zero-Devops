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

type AuthHandler struct {
	AUsecase domain.AuthUsecase
}

func writeCookie(token string, cookie_name string, expiry_time time.Duration) *http.Cookie {
	cookie := new(http.Cookie)
	cookie.Name = cookie_name
	cookie.Value = token
	cookie.Expires = time.Now().Add(expiry_time)

	if !viper.GetBool("IS_PRODUCTION_ENV") {
		cookie.Secure = false
	} else {
		cookie.Secure = true
	}
	cookie.HttpOnly = true
	cookie.SameSite = http.SameSiteLaxMode
	cookie.Path = "/"

	return cookie
}

func readCookie(c *echo.Context, cookie_name string) (string, error) {
	cookie, err := c.Cookie(cookie_name)

	if err != nil {
		return "", err
	}

	return cookie.Value, nil
}

func NewAuthHandler(e *echo.Echo, us domain.AuthUsecase) {
	handler := &AuthHandler{
		AUsecase: us,
	}
	e.GET("/auth/github/login", handler.Login)
	e.POST("/auth/refresh", handler.Refresh)
	e.POST("/auth/logout", handler.Logout)
	e.GET("/auth/user/me", handler.GetUser)
}

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

	access_token_cookie := writeCookie(tokens.AccessToken, "access_token", time.Duration(accessExpiry)*time.Hour)
	refresh_token_cookie := writeCookie(tokens.RefreshToken, "refresh_token", time.Duration(refreshExpiry)*time.Hour)

	c.SetCookie(access_token_cookie)
	c.SetCookie(refresh_token_cookie)

	log.Info("User logged in successfully")
	return c.JSON(http.StatusOK, helper.BuildSuccessResponse(nil, "", reqID, helper.WithMessage("User Logged in Successfully")))
}

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

	access_token_cookie := writeCookie(tokens.AccessToken, "access_token", time.Duration(accessExpiry)*time.Hour)
	refresh_token_cookie := writeCookie(tokens.RefreshToken, "refresh_token", time.Duration(refreshExpiry)*time.Hour)

	c.SetCookie(access_token_cookie)
	c.SetCookie(refresh_token_cookie)

	log.Info("User token refreshed successfully")
	return c.JSON(http.StatusOK, helper.BuildSuccessResponse(nil, "", reqID, helper.WithMessage("User Token Refreshed Successfully")))
}

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

	access_token_cookie := writeCookie("", "access_token", time.Duration(0)*time.Hour)
	refresh_token_cookie := writeCookie("", "refresh_token", time.Duration(0)*time.Hour)
	access_token_cookie.MaxAge = -1
	refresh_token_cookie.MaxAge = -1
	c.SetCookie(access_token_cookie)
	c.SetCookie(refresh_token_cookie)

	log.Info("User logged out successfully")
	return c.JSON(http.StatusOK, helper.BuildSuccessResponse(nil, "", reqID, helper.WithMessage("User Logged Out Successfully")))
}

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
