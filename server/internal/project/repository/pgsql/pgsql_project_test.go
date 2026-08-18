package pgsql

import (
	"Zero_Devops/server/internal/domain"
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// newTestProjectDB returns a *sql.DB for integration tests. It skips unless a
// PostgreSQL DSN is provided via DATABASE_TEST_URL (with migrations applied).
func newTestProjectDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_TEST_URL")
	if dsn == "" {
		t.Skip("DATABASE_TEST_URL not set; skipping pgsql project repository integration tests")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
	return db
}

// seedProjectParents creates the minimal parent rows required by projects FKs and
// registers cleanup so the test never pollutes the database.
func seedProjectParents(t *testing.T, db *sql.DB) (userID, installationUUID uuid.UUID, installationID int64) {
	t.Helper()
	ctx := context.Background()
	now := time.Now()

	userID = uuid.New()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO users (id, provider_id, provider, username, created_at) VALUES ($1,$2,$3,$4,$5)`,
		userID, 1, "github", "tester-"+userID.String(), now); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	installationUUID = uuid.New()
	installationID = now.UnixNano() % 1000000000
	if _, err := db.ExecContext(ctx,
		`INSERT INTO github_installations (id, user_id, installation_id, account_type, account_login, created_at, updated_at, status)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		installationUUID, userID, installationID, "User", "tester", now, now, "active"); err != nil {
		t.Fatalf("seed installation: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE user_id = $1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM github_installations WHERE id = $1`, installationUUID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	})
	return userID, installationUUID, installationID
}

func TestPgSQLProjectRepository_StoreGetAndDuplicateConflict(t *testing.T) {
	db := newTestProjectDB(t)
	defer db.Close()

	userID, installationUUID, _ := seedProjectParents(t, db)
	repo := NewPgSQLProjectRepository(db)
	ctx := context.Background()

	proj := &domain.Project{
		UserID:                    userID.String(),
		InstallationID:            installationUUID.String(),
		GitHubRepositoryID:        12345,
		RepositoryOwner:           "o",
		RepositoryName:            "n",
		RepositoryFullName:        "o/n",
		ConfiguredBranch:          "refs/heads/main",
		ProjectWebhookEnabled:     true,
		DesiredRevisionGeneration: 0,
		BuildConfiguration:        domain.BuildConfiguration{Executable: "go", Args: []string{"build"}, WorkingDir: "/app"},
		ConfigurationVersion:      1,
		CommandPolicyVersion:      "v1",
		CommandScanResult:         domain.CommandScanResult{Status: domain.CommandScanStatusApproved},
		CreatedAt:                 time.Now(),
		UpdatedAt:                 time.Now(),
	}
	if err := repo.Store(ctx, proj); err != nil {
		t.Fatalf("store: %v", err)
	}
	if proj.ID == "" {
		t.Fatal("expected generated project ID")
	}

	got, err := repo.GetByID(ctx, userID.String(), proj.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.GitHubRepositoryID != 12345 || got.ConfiguredBranch != "refs/heads/main" {
		t.Fatalf("unexpected persisted project: %+v", got)
	}

	dup := *proj
	dup.ID = ""
	if err := repo.Store(ctx, &dup); err == nil || !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected ErrConflict for duplicate (user_id, github_repository_id), got %v", err)
	}
}
