package http

import (
	middleware "Zero_Devops/server/authorization/auth/delivery/http/middleware"
	"Zero_Devops/server/domain"
	"net/http"

	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
)

type ResponseError struct {
	Message string `json:"message"`
}

type DeploymentHandler struct {
	dUsecase domain.DeploymentUsecase
}

func NewDeploymentHandler(e *echo.Echo, du domain.DeploymentUsecase) {
	handler := &DeploymentHandler{
		dUsecase: du,
	}
	e.POST("/deploy", handler.CreateDeployment)
}

type createDeploymentRequest struct {
	RepoID int64 `json:"repo_id"`
}

func (h *DeploymentHandler) CreateDeployment(c echo.Context) error {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, ResponseError{Message: "user id not found"})
	}

	var req createDeploymentRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ResponseError{Message: "invalid request body"})
	}

	if req.RepoID == 0 {
		return c.JSON(http.StatusBadRequest, ResponseError{Message: "repo_id is required"})
	}

	ctx := c.Request().Context()
	deployment, err := h.dUsecase.CreateDeployment(ctx, userID, req.RepoID)
	if err != nil {
		return c.JSON(getStatusCode(err), ResponseError{Message: err.Error()})
	}

	return c.JSON(http.StatusCreated, deployment)
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
