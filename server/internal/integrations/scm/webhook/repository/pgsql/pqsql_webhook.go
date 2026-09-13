// Package pgsql stores GitHub webhook delivery records in PostgreSQL.
package pgsql

import (
	"Zero_Devops/server/internal/domain"
	appmiddleware "Zero_Devops/server/internal/middleware"
	"context"
	"database/sql"

	"go.uber.org/zap"
)

type webhookRepository struct {
	Conn *sql.DB
}

// NewPGSQLWebhookRepository creates a PostgreSQL webhook repository.
func NewPGSQLWebhookRepository(conn *sql.DB) domain.WebhookRepository {
	return &webhookRepository{conn}
}

func (r *webhookRepository) InsertDelivery(ctx context.Context, delivery domain.WebhookDelivery) error {
	query := `
		INSERT INTO webhook_deliveries 
		(delivery_id, event_name, event_action, github_installation_external_id, github_installation_db_id, github_repository_id, processing_status,
  		received_at, processed_at, processing_error, payload_reference) 
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
    `

	_, err := r.Conn.ExecContext(ctx, query,
		delivery.DeliveryID,
		delivery.EventName,
		nullString(delivery.EventAction),
		nullInt64(delivery.GitHubInstallationExternalID),
		nullString(delivery.GitHubInstallationDBID),
		nullInt64(delivery.GitHubRepositoryID),
		delivery.ProcessingStatus,
		delivery.ReceivedAt,
		delivery.ProcessedAt,
		nullString(delivery.ProcessingError),
		nullString(delivery.PayloadReference),
	)
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to insert webhook delivery", zap.Error(err))
		return err
	}

	return nil
}

func (r *webhookRepository) UpdateDeliveryStatus(ctx context.Context, deliveryID string, status domain.ProcessingStatus, errMsg *string) error {
	query := `
		 UPDATE webhook_deliveries
		   SET processing_status = $1,
		       processing_error = $2,
		       processed_at = NOW()
		   WHERE delivery_id = $3
		`

	res, err := r.Conn.ExecContext(ctx, query,
		status,
		errMsg,
		deliveryID,
	)

	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to update webhook delivery status", zap.Error(err))
		return err
	}

	affectedRow, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if affectedRow == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *webhookRepository) UpdateDeliveryMetadata(ctx context.Context, deliveryID string, delivery domain.WebhookDelivery) error {
	query := `
		UPDATE webhook_deliveries
		SET event_action = $1,
		    github_installation_external_id = $2,
		    github_installation_db_id = $3,
		    github_repository_id = $4
		WHERE delivery_id = $5
	`

	res, err := r.Conn.ExecContext(ctx, query,
		nullString(delivery.EventAction),
		nullInt64(delivery.GitHubInstallationExternalID),
		nullString(delivery.GitHubInstallationDBID),
		nullInt64(delivery.GitHubRepositoryID),
		deliveryID,
	)
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to update webhook delivery metadata", zap.Error(err))
		return err
	}

	affectedRows, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if affectedRows == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *webhookRepository) GetDeliveryID(ctx context.Context, deliveryID string) (*domain.WebhookDelivery, error) {
	query := `
		SELECT id, delivery_id, event_name, event_action, github_installation_external_id, github_installation_db_id, github_repository_id, processing_status,
  		received_at, processed_at, processing_error, payload_reference FROM webhook_deliveries WHERE delivery_id = $1
	`

	res := r.Conn.QueryRowContext(ctx, query, deliveryID)

	webD := domain.WebhookDelivery{}
	var eventAction sql.NullString
	var installationID sql.NullInt64
	var installationDBID sql.NullString
	var repositoryID sql.NullInt64
	var processedAt sql.NullTime
	var processingError sql.NullString
	var payloadReference sql.NullString

	err := res.Scan(
		&webD.ID,
		&webD.DeliveryID,
		&webD.EventName,
		&eventAction,
		&installationID,
		&installationDBID,
		&repositoryID,
		&webD.ProcessingStatus,
		&webD.ReceivedAt,
		&processedAt,
		&processingError,
		&payloadReference,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to get webhook delivery", zap.Error(err))
		return nil, err
	}

	if eventAction.Valid {
		webD.EventAction = eventAction.String
	}
	if installationID.Valid {
		webD.GitHubInstallationExternalID = installationID.Int64
	}
	if installationDBID.Valid {
		webD.GitHubInstallationDBID = installationDBID.String
	}
	if repositoryID.Valid {
		webD.GitHubRepositoryID = repositoryID.Int64
	}
	if processedAt.Valid {
		webD.ProcessedAt = &processedAt.Time
	}
	if processingError.Valid {
		webD.ProcessingError = processingError.String
	}
	if payloadReference.Valid {
		webD.PayloadReference = payloadReference.String
	}

	return &webD, nil
}

func (r *webhookRepository) TryInsertDelivery(ctx context.Context, delivery domain.WebhookDelivery) (inserted bool, deliveryDBID string, err error) {
	query := `
			INSERT INTO webhook_deliveries 
			(delivery_id, event_name, event_action, github_installation_external_id, github_installation_db_id, github_repository_id, processing_status,
  		received_at, processed_at, processing_error, payload_reference) 
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) ON CONFLICT (delivery_id) DO NOTHING
			RETURNING id
    `

	err = r.Conn.QueryRowContext(ctx, query,
		delivery.DeliveryID,
		delivery.EventName,
		nullString(delivery.EventAction),
		nullInt64(delivery.GitHubInstallationExternalID),
		nullString(delivery.GitHubInstallationDBID),
		nullInt64(delivery.GitHubRepositoryID),
		delivery.ProcessingStatus,
		delivery.ReceivedAt,
		delivery.ProcessedAt,
		nullString(delivery.ProcessingError),
		nullString(delivery.PayloadReference),
	).Scan(&deliveryDBID)

	if err != nil {
		// ON CONFLICT DO NOTHING + RETURNING yields no row for a duplicate
		// delivery: report it as "not inserted" rather than an error.
		if err == sql.ErrNoRows {
			return false, "", nil
		}
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to insert webhook delivery", zap.Error(err))
		return false, "", err
	}

	return true, deliveryDBID, nil
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullInt64(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}
