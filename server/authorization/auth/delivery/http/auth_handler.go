package http

import (
	"Zero_Devops/server/domain"
	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"net/http"
	"strconv"
	"time"
)

type ResponseError struct {
	Message string `json:"message"`
}

type UserResponseMessage struct {
	Message string              `json:"message"`
	Data    domain.UserResponse `json:"data"`
}

type AuthHandler struct {
	AUsecase domain.AuthUsecase
}

func writeCookie(token string, cookie_name string, expiry_time time.Duration) *http.Cookie {
	cookie := new(http.Cookie)
	cookie.Name = cookie_name
	cookie.Value = token
	cookie.Expires = time.Now().Add(expiry_time)

	IS_PRODUCTION_ENV := viper.GetBool("IS_PRODUCTION_ENV")
	if IS_PRODUCTION_ENV == false {
		cookie.Secure = false
	} else {
		cookie.Secure = true
	}
	cookie.HttpOnly = true
	cookie.SameSite = http.SameSiteLaxMode
	cookie.Path = "/"

	return cookie
}

func readCookie(c echo.Context, cookie_name string) (string, error) {
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

func (a *AuthHandler) Login(c echo.Context) error {
	code := c.QueryParam("code")
	if code == "" {
		return c.JSON(http.StatusBadRequest, ResponseError{Message: "Code is Required"})
	}

	ctx := c.Request().Context()
	tokens, err := a.AUsecase.HandleOAuthCallback(ctx, code, "github")

	if err != nil {
		return c.JSON(getStatusCode(err), ResponseError{Message: err.Error()})
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

	return c.JSON(http.StatusOK, map[string]string{
		"message": "User Logged in Successfully",
	})
}

func (a *AuthHandler) Refresh(c echo.Context) error {
	refreshToken, err := readCookie(c, "refresh_token")

	if err != nil {
		return c.JSON(getStatusCode(err), ResponseError{Message: err.Error()})
	}

	ctx := c.Request().Context()
	tokens, err := a.AUsecase.RefreshToken(ctx, refreshToken)

	if err != nil {
		return c.JSON(getStatusCode(err), ResponseError{Message: err.Error()})
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

	return c.JSON(http.StatusOK, map[string]string{
		"message": "User Token Refreshed Successfully",
	})
}

func (a *AuthHandler) Logout(c echo.Context) error {
	ctx := c.Request().Context()
	accessToken, err := readCookie(c, "access_token")
	if err != nil {
		return c.JSON(getStatusCode(err), ResponseError{Message: err.Error()})
	}

	err = a.AUsecase.Logout(ctx, accessToken)
	if err != nil {
		return c.JSON(getStatusCode(err), ResponseError{Message: err.Error()})
	}

	access_token_cookie := writeCookie("", "access_token", time.Duration(0)*time.Hour)
	refresh_token_cookie := writeCookie("", "refresh_token", time.Duration(0)*time.Hour)
	access_token_cookie.MaxAge = -1
	refresh_token_cookie.MaxAge = -1
	c.SetCookie(access_token_cookie)
	c.SetCookie(refresh_token_cookie)

	return c.JSON(http.StatusOK, map[string]string{"message": "User Logged Out Successfully"})
}

func (a *AuthHandler) GetUser(c echo.Context) error {
	accessToken, err := readCookie(c, "access_token")

	if err != nil {
		return c.JSON(getStatusCode(err), ResponseError{Message: err.Error()})
	}
	ctx := c.Request().Context()

	userResponse, err := a.AUsecase.GetCurrentUser(ctx, accessToken)

	if err != nil {
		return c.JSON(getStatusCode(err), ResponseError{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, UserResponseMessage{Message: "user details fetched successfully", Data: userResponse})
}

func getStatusCode(err error) int {
	if err == nil {
		return http.StatusOK
	}

	logrus.Error(err)
	switch err {
	case domain.ErrInternalServerError:
		return http.StatusInternalServerError
	case domain.ErrNotFound:
		return http.StatusNotFound
	case domain.ErrConflict:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
