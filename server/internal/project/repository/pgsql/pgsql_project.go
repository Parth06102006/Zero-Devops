// Package pgsql provides PostgreSQL repository implementations for projects.
package pgsql

import (
	"Zero_Devops/server/internal/domain"
	appmiddleware "Zero_Devops/server/internal/middleware"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/lib/pq"
	"go.uber.org/zap"
)

type pgSQLProjectRepository struct {
	Conn *sql.DB
}

// NewPgSQLProjectRepository creates a new ProjectRepository backed by PostgreSQL.
func NewPgSQLProjectRepository(conn *sql.DB) domain.ProjectRepository {
	return &pgSQLProjectRepository{conn}
}

// Store inserts a project row and populates p.ID via RETURNING. Identity columns
// (user_id, installation_id, github_repository_id, owner/name/full_name) and
// created_at/updated_at come from the caller; the usecase is responsible for
// setting them before calling Store.
func (m *pgSQLProjectRepository) Store(ctx context.Context, p *domain.Project) error {
	buildConfig, err := json.Marshal(p.BuildConfiguration)
	if err != nil {
		return fmt.Errorf("marshal build configuration: %w", err)
	}
	scanResult, err := json.Marshal(p.CommandScanResult)
	if err != nil {
		return fmt.Errorf("marshal command scan result: %w", err)
	}

	query := `
		INSERT INTO projects (
			user_id, github_installation_id, github_repository_id,
			repository_owner, repository_name, repository_full_name,
			configured_branch, project_webhook_enabled, repository_available, desired_revision_generation,
			build_configuration, configuration_version, command_policy_version,
			command_scan_result, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id
	`
	if err := m.Conn.QueryRowContext(ctx, query,
		p.UserID, p.InstallationID, p.GitHubRepositoryID,
		p.RepositoryOwner, p.RepositoryName, p.RepositoryFullName,
		p.ConfiguredBranch, p.ProjectWebhookEnabled, p.RepositoryAvailable, p.DesiredRevisionGeneration,
		buildConfig, p.ConfigurationVersion, p.CommandPolicyVersion,
		scanResult, p.CreatedAt, p.UpdatedAt,
	).Scan(&p.ID); err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return domain.ErrConflict
		}
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to store project", zap.Error(err))
		return err
	}
	return nil
}

// ListByUserID returns all projects owned by the user, newest first.
func (m *pgSQLProjectRepository) ListByUserID(ctx context.Context, userID string) ([]domain.Project, error) {
	query := `
		SELECT id, user_id, github_installation_id, github_repository_id,
			repository_owner, repository_name, repository_full_name,
			configured_branch, project_webhook_enabled, repository_available, desired_revision_generation,
			build_configuration, configuration_version, command_policy_version,
			command_scan_result, created_at, updated_at
		FROM projects
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := m.Conn.QueryContext(ctx, query, userID)
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to query projects by user ID", zap.Error(err))
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			appmiddleware.LoggerFromContext(ctx).Error("failed to close rows", zap.Error(err))
		}
	}()

	var projects []domain.Project
	for rows.Next() {
		var p domain.Project
		var buildConfig, scanResult []byte
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.InstallationID, &p.GitHubRepositoryID,
			&p.RepositoryOwner, &p.RepositoryName, &p.RepositoryFullName,
			&p.ConfiguredBranch, &p.ProjectWebhookEnabled, &p.RepositoryAvailable, &p.DesiredRevisionGeneration,
			&buildConfig, &p.ConfigurationVersion, &p.CommandPolicyVersion,
			&scanResult, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			log := appmiddleware.LoggerFromContext(ctx)
			log.Error("failed to scan project", zap.Error(err))
			return nil, err
		}
		if err := json.Unmarshal(buildConfig, &p.BuildConfiguration); err != nil {
			return nil, fmt.Errorf("unmarshal build configuration: %w", err)
		}
		if err := json.Unmarshal(scanResult, &p.CommandScanResult); err != nil {
			return nil, fmt.Errorf("unmarshal command scan result: %w", err)
		}
		projects = append(projects, p)
	}

	if projects == nil {
		projects = []domain.Project{}
	}
	return projects, nil
}

// GetByID returns a single project scoped to the user, or ErrNotFound.
func (m *pgSQLProjectRepository) GetByID(ctx context.Context, userID, id string) (*domain.Project, error) {
	query := `
		SELECT id, user_id, github_installation_id, github_repository_id,
			repository_owner, repository_name, repository_full_name,
			configured_branch, project_webhook_enabled, repository_available, desired_revision_generation,
			build_configuration, configuration_version, command_policy_version,
			command_scan_result, created_at, updated_at
		FROM projects
		WHERE id = $1 AND user_id = $2
	`
	var p domain.Project
	var buildConfig, scanResult []byte
	if err := m.Conn.QueryRowContext(ctx, query, id, userID).Scan(
		&p.ID, &p.UserID, &p.InstallationID, &p.GitHubRepositoryID,
		&p.RepositoryOwner, &p.RepositoryName, &p.RepositoryFullName,
		&p.ConfiguredBranch, &p.ProjectWebhookEnabled, &p.RepositoryAvailable, &p.DesiredRevisionGeneration,
		&buildConfig, &p.ConfigurationVersion, &p.CommandPolicyVersion,
		&scanResult, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to get project by ID", zap.Error(err))
		return nil, err
	}
	if err := json.Unmarshal(buildConfig, &p.BuildConfiguration); err != nil {
		return nil, fmt.Errorf("unmarshal build configuration: %w", err)
	}
	if err := json.Unmarshal(scanResult, &p.CommandScanResult); err != nil {
		return nil, fmt.Errorf("unmarshal command scan result: %w", err)
	}
	return &p, nil
}

// Update persists the mutable columns of an existing project. It writes only:
//
//	configured_branch, project_webhook_enabled, repository_available, build_configuration,
//	configuration_version, command_policy_version, command_scan_result, updated_at
//
// Identity/immutable columns (id, user_id, installation_id, github_repository_id,
// repository_owner/name/full_name, created_at) are never written. The update is
// scoped to the owning user via WHERE id=$1 AND user_id=$2, and updated_at is set
// to NOW(). A missing row yields ErrNotFound.
func (m *pgSQLProjectRepository) Update(ctx context.Context, p *domain.Project) error {
	buildConfig, err := json.Marshal(p.BuildConfiguration)
	if err != nil {
		return fmt.Errorf("marshal build configuration: %w", err)
	}
	scanResult, err := json.Marshal(p.CommandScanResult)
	if err != nil {
		return fmt.Errorf("marshal command scan result: %w", err)
	}

	query := `
		UPDATE projects SET
			configured_branch = $1,
			project_webhook_enabled = $2,
			repository_available = $3,
			build_configuration = $4,
			configuration_version = $5,
			command_policy_version = $6,
			command_scan_result = $7,
			updated_at = NOW()
		WHERE id = $8 AND user_id = $9
	`
	res, err := m.Conn.ExecContext(ctx, query,
		p.ConfiguredBranch, p.ProjectWebhookEnabled, p.RepositoryAvailable, buildConfig,
		p.ConfigurationVersion, p.CommandPolicyVersion, scanResult, p.ID, p.UserID,
	)
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to update project", zap.Error(err))
		return err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Delete removes a project scoped to the owning user. A missing row yields
// ErrNotFound so callers can distinguish "not yours" from "deleted".
func (m *pgSQLProjectRepository) Delete(ctx context.Context, userID, id string) error {
	query := `DELETE FROM projects WHERE id = $1 AND user_id = $2`
	res, err := m.Conn.ExecContext(ctx, query, id, userID)
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to delete project", zap.Error(err))
		return err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// //// ****** THIS FUNCTION WAS NOT MADE BY ME IT WAS MADE BY GLM 5.3 THE CASE LOOKS FINE AND IT HAS ADDED OTHER LOGGER AS WELL WHICH I HAVE TO ADD
// GetByInstallationAndRepositoryID returns the project selected for the given
// repository under the given installation, or ErrNotFound. This is the webhook
// push lookup: it is deliberately NOT user-scoped because webhooks carry no
// user context — installation identity plus GitHub repository ID is the durable
// key (one installation belongs to one user in V1, so at most one row matches).
func (m *pgSQLProjectRepository) GetByInstallationAndRepositoryID(ctx context.Context, githubInstallationID string, githubRepositoryID int64) (*domain.Project, error) {
	query := `
		SELECT id, user_id, github_installation_id, github_repository_id,
			repository_owner, repository_name, repository_full_name,
			configured_branch, project_webhook_enabled, repository_available, desired_revision_generation,
			build_configuration, configuration_version, command_policy_version,
			command_scan_result, created_at, updated_at
		FROM projects
		WHERE github_installation_id = $1 AND github_repository_id = $2
	`
	var p domain.Project
	var buildConfig, scanResult []byte
	if err := m.Conn.QueryRowContext(ctx, query, githubInstallationID, githubRepositoryID).Scan(
		&p.ID, &p.UserID, &p.InstallationID, &p.GitHubRepositoryID,
		&p.RepositoryOwner, &p.RepositoryName, &p.RepositoryFullName,
		&p.ConfiguredBranch, &p.ProjectWebhookEnabled, &p.RepositoryAvailable, &p.DesiredRevisionGeneration,
		&buildConfig, &p.ConfigurationVersion, &p.CommandPolicyVersion,
		&scanResult, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to get project by installation and repository ID", zap.Error(err))
		return nil, err
	}
	if err := json.Unmarshal(buildConfig, &p.BuildConfiguration); err != nil {
		return nil, fmt.Errorf("unmarshal build configuration: %w", err)
	}
	if err := json.Unmarshal(scanResult, &p.CommandScanResult); err != nil {
		return nil, fmt.Errorf("unmarshal command scan result: %w", err)
	}
	return &p, nil
}

func (m *pgSQLProjectRepository) GetProjectRepoAvailability(ctx context.Context, githubInstallationID string) (map[int64]bool, error) {
	query := `
		SELECT github_repository_id, repository_available
		FROM projects
		WHERE github_installation_id = $1
	`

	rows, err := m.Conn.QueryContext(ctx, query, githubInstallationID)
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to query project repo availability", zap.Error(err))
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			appmiddleware.LoggerFromContext(ctx).Error("failed to close rows", zap.Error(err))
		}
	}()

	repositories := make(map[int64]bool)
	for rows.Next() {
		var repositoryID int64
		var isAvailable bool
		if err := rows.Scan(&repositoryID, &isAvailable); err != nil {
			log := appmiddleware.LoggerFromContext(ctx)
			log.Error("failed to scan project repo availability row", zap.Error(err))
			return nil, err
		}
		repositories[repositoryID] = isAvailable
	}
	if err := rows.Err(); err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to iterate project repo availability rows", zap.Error(err))
		return nil, err
	}
	return repositories, nil
}

func (m *pgSQLProjectRepository) UpdateProjectRepoAvailability(ctx context.Context, githubInstallationID string, listOfRepos map[int64]bool) error {
	if len(listOfRepos) == 0 {
		return nil
	}

	availabilityJSON, err := json.Marshal(listOfRepos)
	if err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to marshal repo availability", zap.Error(err))
		return err
	}

	query := `
		UPDATE projects AS p
		SET repository_available = availability.is_available::boolean,
			updated_at = NOW()
		FROM jsonb_each_text($1::jsonb)
			AS availability(repository_id, is_available)
		WHERE p.github_installation_id = $2
			AND p.github_repository_id = availability.repository_id::bigint
	`

	if _, err := m.Conn.ExecContext(ctx, query, availabilityJSON, githubInstallationID); err != nil {
		log := appmiddleware.LoggerFromContext(ctx)
		log.Error("failed to update repo availability", zap.Error(err))
		return err
	}

	return nil
}
