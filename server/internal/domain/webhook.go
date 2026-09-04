package domain

import (
	"context"
	"net/http"
	"time"
)

// ProcessingStatus tracks the lifecycle of a webhook delivery.
type ProcessingStatus string

const (
	// ProcessingStatusReceived indicates the delivery was stored but not yet processed
	ProcessingStatusReceived ProcessingStatus = "received"
	// ProcessingStatusAccepted indicates the delivery passed validation and was accepted
	ProcessingStatusAccepted ProcessingStatus = "accepted"
	// ProcessingStatusProcessed indicates the delivery was fully processed
	ProcessingStatusProcessed ProcessingStatus = "processed"
	// ProcessingStatusFailed indicates the delivery failed during processing
	ProcessingStatusFailed ProcessingStatus = "failed"
)

// WebhookDelivery is the persisted record of one GitHub webhook delivery.
type WebhookDelivery struct {
	ID                           string           `json:"id"`
	DeliveryID                   string           `json:"delivery_id"`
	EventName                    string           `json:"event_name"`
	EventAction                  string           `json:"event_action,omitempty"`
	GitHubInstallationExternalID int64            `json:"github_installation_external_id,omitempty"`
	GitHubInstallationDBID       string           `json:"github_installation_db_id,omitempty"`
	GitHubRepositoryID           int64            `json:"github_repository_id,omitempty"`
	ProcessingStatus             ProcessingStatus `json:"processing_status"`
	ReceivedAt                   time.Time        `json:"received_at"`
	ProcessedAt                  *time.Time       `json:"processed_at,omitempty"`
	ProcessingError              string           `json:"processing_error,omitempty"`
	PayloadReference             string           `json:"payload_reference,omitempty"`
}

type WebhookUsecase interface {
	HandleGithubWebhook(ctx context.Context, r *http.Request) (interface{}, error)
}

type WebhookRepository interface {
	InsertDelivery(ctx context.Context, delivery WebhookDelivery) error
	UpdateDeliveryStatus(ctx context.Context, deliveryID string, status ProcessingStatus, errMsg *string) error
	UpdateDeliveryMetadata(ctx context.Context, deliveryID string, delivery WebhookDelivery) error
	GetDeliveryId(ctx context.Context, deliveryID string) (*WebhookDelivery, error)
	// TryInsertDelivery inserts a delivery keyed by its unique GitHub delivery
	// UUID. It reports whether the row was newly inserted and, when it was,
	// returns the new database row UUID (webhook_deliveries.id) for foreign-key
	// linkage. A duplicate returns inserted=false without an error.
	TryInsertDelivery(ctx context.Context, delivery WebhookDelivery) (inserted bool, deliveryDBID string, err error)
}
