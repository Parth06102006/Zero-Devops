package http

import (
	middleware "Zero_Devops/server/authorization/auth/delivery/http/middleware"
	"Zero_Devops/server/domain"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
)

type ResponseError struct {
	Message string `json:"message"`
}


type SCMHandler struct {
	scmUsecase domain.GithubUsecase
}

func NewSCMHandler(e *echo.Echo, gh domain.GithubUsecase){
	handler := &SCMHandler{
		scmUsecase : gh,
	}

	e.POST("/integration/scm/github/install",handler.Installation)
	e.GET("/integration/scm/github/",handler.GetInstallation)
	e.DELETE("/integration/scm/github/delete",handler.DeleteInstallation)
}


func (inst *SCMHandler) Installation(c echo.Context) error {
	/*
		I need to add the Github Installtion here since I need to install the github app for now in order to do it how can i perform it 
	*/
	code := strings.TrimSpace(c.QueryParam("code"))

	if(code == ""){
		return c.JSON(getStatusCode(domain.ErrNotFound), ResponseError{Message: "Code not Present"})
	}

	ctx := c.Request().Context()

	userID, ok := middleware.GetUserID(c)

	if !ok {
		return c.JSON(http.StatusUnauthorized, ResponseError{Message: "user id not found"})
	}

	client := createProductionClient()

	err := inst.scmUsecase.InstallGithubApp(ctx,client,code,userID)

	if (err != nil){
		return c.JSON(getStatusCode(err), ResponseError{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "Github App Installed Successfully",
	})

}

func (inst *SCMHandler) GetInstallation(c echo.Context) error {
    userID, ok := middleware.GetUserID(c)
    if !ok {
        return c.JSON(http.StatusUnauthorized, ResponseError{Message: "user id not found"})
    }
    ctx := c.Request().Context()
    installation, err := inst.scmUsecase.GetGithubAppInstallation(ctx, userID)
    if err != nil {
        return c.JSON(getStatusCode(err), ResponseError{Message: err.Error()})
    }
    return c.JSON(http.StatusOK, installation)
}
func (inst *SCMHandler) DeleteInstallation(c echo.Context) error {
    userID, ok := middleware.GetUserID(c)
    if !ok {
        return c.JSON(http.StatusUnauthorized, ResponseError{Message: "user id not found"})
    }
    ctx := c.Request().Context()
    err := inst.scmUsecase.DeleteGithubApp(ctx, userID)
    if err != nil {
        return c.JSON(getStatusCode(err), ResponseError{Message: err.Error()})
    }
    return c.JSON(http.StatusOK, map[string]string{"message": "GitHub App uninstalled successfully"})
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
