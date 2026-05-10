package http

import (
	"net/http"
	"strconv"
	
	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
	validator "github.com/go-playground/validator.v9"
	"server/domain"
)

type ResponseError struct {
	Message string `json:"message"`
}

type AuthHandler struct {
	AUsecase domain.AuthUsecase
}

func NewAuthHandler(e *echo.Echo , us domain.AuthUsecase){
	handler := &AuthHandler{
		AUsecase: us
	}
	e.POST("/login", handler.Login)
	e.GET("/user/:id", handler.GetUser)
	e.POST("/logout", handler.Logout)
}

func (a* AuthHandler) Login(c echo.Context) error {
}