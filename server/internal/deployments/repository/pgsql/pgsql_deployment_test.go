package pgsql

import (
	"Zero_Devops/server/internal/domain"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

const deploymentsTestDriverName = "deployment_repo_test_driver"

var (
	deploymentsDriverOnce sync.Once
	deploymentsState      = &deploymentsDBState{}
)

type deploymentsDBState struct {
	mu            sync.Mutex
	lastUserID    string
	lastProjectID string
	emptyRows     bool
	lastStore     *domain.Deployment
	queryRowErr   error
	queryErr      error
	rowsErr       error
	getByIDErr    error
}

type deploymentsDriver struct{}
type deploymentsConn struct{}
type deploymentsStmt struct{}
type deploymentsRows struct {
	cols []string
	vals [][]driver.Value
	idx  int
}
type deploymentsResult struct{ rowsAffected int64 }

var fullDeploymentColumns = []string{
	"id", "user_id", "repo_id", "clone_url", "status",
	"project_id", "github_installation_id", "commit_sha", "requested_ref", "trigger",
	"desired_revision_generation", "configuration_snapshot", "configuration_version",
	"command_policy_version", "command_scan_result", "manual_idempotency_key",
	"output_url", "error_message", "created_at", "updated_at",
}

func fullDeploymentRow(userID, projectID string) []driver.Value {
	return []driver.Value{
		"1", userID, int64(22),
		"https://example.com/repo.git", string(domain.DeploymentStatusPending),
		projectID, "inst-1", "aabbccddeeff00112233445566778899aabbccdd", "refs/heads/main", "manual",
		int64(0), []byte(`{"executable":"npm","args":["run","build"],"working_dir":".","scanner_policy_version":"v1"}`), int32(1),
		"v1", []byte(`{"status":"approved","policy_version":"v1","source":"manual"}`), "idem-1",
		"", "", time.Now(), time.Now(),
	}
}

func registerDeploymentsDriver() {
	deploymentsDriverOnce.Do(func() {
		sql.Register(deploymentsTestDriverName, &deploymentsDriver{})
	})
}

func (d *deploymentsDriver) Open(_ string) (driver.Conn, error)  { return &deploymentsConn{}, nil }
func (c *deploymentsConn) Close() error                          { return nil }
func (c *deploymentsConn) Begin() (driver.Tx, error)             { return nil, errors.New("tx not supported") }
func (c *deploymentsConn) Prepare(_ string) (driver.Stmt, error) { return &deploymentsStmt{}, nil }
func (c *deploymentsConn) PrepareContext(_ context.Context, _ string) (driver.Stmt, error) {
	return &deploymentsStmt{}, nil
}

func (c *deploymentsConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	deploymentsState.mu.Lock()
	defer deploymentsState.mu.Unlock()
	if deploymentsState.queryErr != nil {
		return nil, deploymentsState.queryErr
	}
	if len(args) > 0 {
		if v, ok := args[0].Value.(string); ok {
			if contains(query, "project_id = $1") {
				deploymentsState.lastProjectID = v
			} else {
				deploymentsState.lastUserID = v
			}
		}
		if len(args) > 1 {
			if v, ok := args[1].Value.(string); ok {
				deploymentsState.lastUserID = v
			}
		}
	}
	if deploymentsState.rowsErr != nil {
		return nil, deploymentsState.rowsErr
	}
	if contains(query, "RETURNING id") {
		return &deploymentsRows{
			cols: []string{"id"},
			vals: [][]driver.Value{{"1"}},
		}, nil
	}
	if deploymentsState.queryRowErr == sql.ErrNoRows || deploymentsState.emptyRows {
		return &deploymentsRows{
			cols: fullDeploymentColumns,
			vals: [][]driver.Value{},
		}, nil
	}
	return &deploymentsRows{
		cols: fullDeploymentColumns,
		vals: [][]driver.Value{fullDeploymentRow(deploymentsState.lastUserID, deploymentsState.lastProjectID)},
	}, nil
}

func (c *deploymentsConn) Query(_ string, _ []driver.Value) (driver.Rows, error) {
	return &deploymentsRows{}, nil
}
func (c *deploymentsConn) ExecContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	return deploymentsResult{rowsAffected: 1}, nil
}

func (s *deploymentsStmt) Close() error  { return nil }
func (s *deploymentsStmt) NumInput() int { return -1 }
func (s *deploymentsStmt) Exec(_ []driver.Value) (driver.Result, error) {
	return deploymentsResult{rowsAffected: 1}, nil
}
func (s *deploymentsStmt) Query(_ []driver.Value) (driver.Rows, error) {
	return &deploymentsRows{}, nil
}

func (r deploymentsResult) LastInsertId() (int64, error) { return 1, nil }
func (r deploymentsResult) RowsAffected() (int64, error) { return r.rowsAffected, nil }
func (r *deploymentsRows) Columns() []string             { return r.cols }
func (r *deploymentsRows) Close() error                  { return nil }
func (r *deploymentsRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.vals) {
		return io.EOF
	}
	copy(dest, r.vals[r.idx])
	r.idx++
	return nil
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func newDeploymentsTestDB(t *testing.T) *sql.DB {
	t.Helper()
	registerDeploymentsDriver()
	db, err := sql.Open(deploymentsTestDriverName, "")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

func resetDeploymentsState() {
	deploymentsState.mu.Lock()
	defer deploymentsState.mu.Unlock()
	deploymentsState.lastUserID = ""
	deploymentsState.lastProjectID = ""
	deploymentsState.emptyRows = false
	deploymentsState.lastStore = nil
	deploymentsState.queryRowErr = nil
	deploymentsState.queryErr = nil
	deploymentsState.rowsErr = nil
	deploymentsState.getByIDErr = nil
}

func TestStore(t *testing.T) {
	resetDeploymentsState()
	db := newDeploymentsTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewPgSQLDeploymentRepository(db)
	d := &domain.Deployment{
		UserID: "7", RepoID: 8, CloneURL: "https://example.com/x.git",
		Status: domain.DeploymentStatusPending, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := repo.Store(context.Background(), d); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if d.ID == "" {
		t.Fatal("expected ID to be set")
	}
}

func TestGetByUserID(t *testing.T) {
	resetDeploymentsState()
	db := newDeploymentsTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewPgSQLDeploymentRepository(db)
	got, err := repo.GetByUserID(context.Background(), "44")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(got) != 1 || got[0].UserID != "44" {
		t.Fatalf("unexpected deployments: %+v", got)
	}
}

func TestGetByUserID_ReturnsFullDeployment(t *testing.T) {
	resetDeploymentsState()
	db := newDeploymentsTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewPgSQLDeploymentRepository(db)
	got, err := repo.GetByUserID(context.Background(), "44")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 deployment, got %d", len(got))
	}
	if got[0].ProjectID != "" {
		t.Fatalf("expected empty project ID for user-scoped query, got %q", got[0].ProjectID)
	}
	if got[0].UserID != "44" {
		t.Fatalf("expected user ID 44, got %q", got[0].UserID)
	}
	if got[0].CommitSHA == "" {
		t.Fatal("expected commit SHA to be populated")
	}
	if got[0].ConfigurationSnapshot.Executable != "npm" {
		t.Fatalf("expected executable npm, got %q", got[0].ConfigurationSnapshot.Executable)
	}
	if got[0].CommandScanResult.Status != "approved" {
		t.Fatalf("expected approved scan status, got %q", got[0].CommandScanResult.Status)
	}
}

func TestGetByProjectID_ReturnsScopedDeployments(t *testing.T) {
	resetDeploymentsState()
	db := newDeploymentsTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewPgSQLDeploymentRepository(db)
	got, err := repo.GetByProjectID(context.Background(), "44", "p9")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 deployment, got %d", len(got))
	}
	if got[0].UserID != "44" {
		t.Fatalf("expected user ID 44, got %q", got[0].UserID)
	}
	if got[0].ProjectID != "p9" {
		t.Fatalf("expected project ID p9, got %q", got[0].ProjectID)
	}
}

func TestGetByProjectID_EmptyRows(t *testing.T) {
	resetDeploymentsState()
	db := newDeploymentsTestDB(t)
	defer func() { _ = db.Close() }()

	deploymentsState.mu.Lock()
	deploymentsState.emptyRows = true
	deploymentsState.mu.Unlock()

	repo := NewPgSQLDeploymentRepository(db)
	got, err := repo.GetByProjectID(context.Background(), "44", "p9")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %+v", got)
	}
}

func TestGetByID_ReturnsFullDeployment(t *testing.T) {
	resetDeploymentsState()
	db := newDeploymentsTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewPgSQLDeploymentRepository(db)
	got, err := repo.GetByID(context.Background(), "44", "1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got.ID != "1" {
		t.Fatalf("expected ID 1, got %q", got.ID)
	}
	if got.UserID != "44" {
		t.Fatalf("expected user ID 44, got %q", got.UserID)
	}
	if got.CommitSHA == "" {
		t.Fatal("expected commit SHA to be populated")
	}
}

func TestGetByID_NotFound(t *testing.T) {
	resetDeploymentsState()
	deploymentsState.queryRowErr = sql.ErrNoRows

	db := newDeploymentsTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewPgSQLDeploymentRepository(db)
	_, err := repo.GetByID(context.Background(), "1", "2")
	if err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
