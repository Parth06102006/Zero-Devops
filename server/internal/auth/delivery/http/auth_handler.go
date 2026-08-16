// Package http provides HTTP delivery handlers for the authentication system
package http

import (
	"Zero_Devops/server/internal/domain"
	"Zero_Devops/server/internal/helper"
	"Zero_Devops/server/internal/middleware"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// OAuth Payload
type oauthStatePayload struct {
	State    string `json:"state"`
	ReturnTo string `json:"return_to"`
}

// AuthHandler handles authentication-related HTTP requests
type AuthHandler struct {
	AUsecase domain.AuthUsecase
}

const (
	stateBytesLength  = 32
	stateCookieMaxAge = 600
)

//nolint:gosec
func writeCookie(token, cookieName string, expiryTime time.Duration, sameSite http.SameSite) *http.Cookie {
	return &http.Cookie{
		Name:     cookieName,
		Value:    token,
		MaxAge:   int(expiryTime.Seconds()),
		Secure:   true,
		HttpOnly: true,
		SameSite: sameSite, // added the isNone during development since the client and server are on different origins
		Path:     "/",
		Domain:   cookieDomain(),
	}
}

// cookieDomain returns the Domain attribute used for cookies. When the client
// and server live on different subdomains of the same registrable domain
// (e.g. ghost.parthgarg.me and dev.parthgarg.me) it MUST be set to the parent
// domain (".parthgarg.me") so the browser shares the cookies across subdomains.
// An empty value keeps the cookies host-only, which is correct for localhost.
func cookieDomain() string {
	return viper.GetString("COOKIE_DOMAIN")
}

func readCookie(c *echo.Context, cookieName string) (string, error) {
	cookie, err := c.Cookie(cookieName)

	if err != nil {
		return "", err
	}

	return cookie.Value, nil
}

func generateRandomState() (string, error) {
	b := make([]byte, stateBytesLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// NewAuthHandler registers authentication routes on the Echo instance
func NewAuthHandler(e *echo.Echo, us domain.AuthUsecase) {
	handler := &AuthHandler{
		AUsecase: us,
	}
	e.GET("/auth/github/login", handler.Login)
	e.GET("/auth/github/login/callback", handler.LoginCallback)
	e.POST("/auth/refresh", handler.Refresh)
	e.POST("/auth/logout", handler.Logout)
	e.GET("/auth/user/me", handler.GetUser)
}

// Login Handler Redirect to Oauth login
func (a *AuthHandler) Login(c *echo.Context) error {
	reqID := middleware.GetRequestID(c)
	log := middleware.LoggerFromContext(c.Request().Context())

	returnTo := c.QueryParam("return_to")

	if returnTo == "" {
		log.Warn("Missing returnTo parameter")
		returnTo = "/"
	}

	state, err := generateRandomState()

	if err != nil {
		log.Error("Error Generating State", zap.Error(err))
		return c.JSON(helper.GetStatusCode(err), helper.BuildErrorResponse(err.Error(), err, reqID))
	}

	payload := oauthStatePayload{State: state, ReturnTo: returnTo}

	payloadBytes, _ := json.Marshal(payload)
	encodedPayload := base64.URLEncoding.EncodeToString(payloadBytes)

	stateCookie := writeCookie(encodedPayload, "gh_oauth_state", stateCookieMaxAge, http.SameSiteLaxMode)

	c.SetCookie(stateCookie)

	authCodeURL, err := a.AUsecase.GithubOauthURL(c.Request().Context(), state)

	if err != nil {
		log.Error("Error Fetching AuthURL", zap.Error(err))
		return c.JSON(helper.GetStatusCode(err), helper.BuildErrorResponse(err.Error(), err, reqID))
	}

	return c.Redirect(http.StatusTemporaryRedirect, authCodeURL)
}

// LoginCallback handles the OAuth login callback
func (a *AuthHandler) LoginCallback(c *echo.Context) error {
	reqID := middleware.GetRequestID(c)
	log := middleware.LoggerFromContext(c.Request().Context())

	cookie, err := c.Cookie("gh_oauth_state")
	if err != nil {
		log.Warn("Missing state cookie", zap.Error(err))
		return c.JSON(http.StatusBadRequest, helper.BuildErrorResponse("missing state cookie", err, reqID))
	}

	payloadBytes, err := base64.URLEncoding.DecodeString(cookie.Value)
	if err != nil {
		log.Warn("Invalid state cookie encoding", zap.Error(err))
		return c.JSON(http.StatusBadRequest, helper.BuildErrorResponse("invalid state cookie", err, reqID))
	}

	var payload oauthStatePayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		log.Warn("Invalid state cookie format", zap.Error(err))
		return c.JSON(http.StatusBadRequest, helper.BuildErrorResponse("invalid state cookie", err, reqID))
	}

	if c.QueryParam("state") != payload.State {
		err := fmt.Errorf("state mismatch")
		log.Warn("State mismatch", zap.Error(err))
		return c.JSON(http.StatusForbidden, helper.BuildErrorResponse("state mismatch", err, reqID))
	}

	//nolint:gosec // clearing a cookie: intentionally no Secure/HttpOnly attributes needed
	stateCookie := writeCookie("", "gh_oauth_state", 0, http.SameSiteLaxMode)
	stateCookie.MaxAge = -1
	c.SetCookie(stateCookie)

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

	accessTokenCookie := writeCookie(tokens.AccessToken, "access_token", time.Duration(accessExpiry)*time.Hour, http.SameSiteNoneMode)
	refreshTokenCookie := writeCookie(tokens.RefreshToken, "refresh_token", time.Duration(refreshExpiry)*time.Hour, http.SameSiteNoneMode)

	c.SetCookie(accessTokenCookie)
	c.SetCookie(refreshTokenCookie)

	log.Info("User logged in successfully")

	redirectPage, err := url.Parse(payload.ReturnTo)

	frontendURL := viper.GetString("FRONTEND_URL")

	if err != nil || redirectPage.IsAbs() || redirectPage.Host != "" {
		redirectPage, _ = url.Parse("/") // block open-redirect via return_to
	}

	redirectURL := fmt.Sprintf("%s%s", frontendURL, redirectPage.String())

	return c.Redirect(http.StatusTemporaryRedirect, redirectURL)
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

	accessTokenCookie := writeCookie(tokens.AccessToken, "access_token", time.Duration(accessExpiry)*time.Hour, http.SameSiteNoneMode)
	refreshTokenCookie := writeCookie(tokens.RefreshToken, "refresh_token", time.Duration(refreshExpiry)*time.Hour, http.SameSiteNoneMode)

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
	accessTokenCookie := writeCookie("", "access_token", 0, http.SameSiteNoneMode)
	//nolint:gosec
	refreshTokenCookie := writeCookie("", "refresh_token", 0, http.SameSiteNoneMode)
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
