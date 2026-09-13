// Package http exposes GitHub webhook HTTP handlers.
package http

import (
	"Zero_Devops/server/internal/domain"
	"Zero_Devops/server/internal/helper"
	_appMiddleware "Zero_Devops/server/internal/middleware"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"go.uber.org/zap"
)

const maxWebhookPayloadSize = 10_485_760

// WebhookHandler handles GitHub webhook HTTP requests.
type WebhookHandler struct {
	// right now no use case function
	WebHookUsecase domain.WebhookUsecase
}

// NewWebhookHandler registers GitHub webhook routes.
func NewWebhookHandler(e *echo.Echo, webHus domain.WebhookUsecase) {
	handler := &WebhookHandler{
		WebHookUsecase: webHus,
	}

	e.POST("/webhooks/github", handler.WebhookGithub, middleware.BodyLimit(maxWebhookPayloadSize))
}

// WebhookGithub processes a single GitHub webhook delivery.
func (wh *WebhookHandler) WebhookGithub(c *echo.Context) error {
	reqID := _appMiddleware.GetRequestID(c)
	log := _appMiddleware.LoggerFromContext(c.Request().Context())

	_, err := wh.WebHookUsecase.HandleGithubWebhook(c.Request().Context(), c.Request())
	if err != nil {
		log.Error("Failed to parse GitHub webhook", zap.Error(err))
		return c.JSON(helper.GetStatusCode(err), helper.BuildErrorResponse(err.Error(), err, reqID))
	}

	log.Info("GitHub webhook processed successfully")
	return c.JSON(http.StatusOK, helper.BuildSuccessResponse(nil, "", reqID, helper.WithMessage("webhook processed successfully")))
}
