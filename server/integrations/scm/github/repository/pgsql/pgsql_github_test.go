package pgsql

import (
	"Zero_Devops/server/domain"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

const githubTestDriverName = "github_repo_test_driver"

var (
	githubTestDriverOnce sync.Once
	githubTestState      = &githubTestDBState{}
)

type githubTestDBState struct {
	mu             sync.Mutex
	lastQueryUserID int64
	lastDeleteUser  int64
	lastUpdateUser  int64
	lastUpdateStatus string
	queryRowErr     error
	deleteErr       error
}

type githubTestDriver struct{}
type githubTestConn struct{}
type githubTestStmt struct {
	query string
}
type githubTestResult struct {
	rowsAffected int64
	insertID     int64
}
type githubTestRows struct {
	cols []string
	vals [][]driver.Value
	idx  int
}

func registerGithubTestDriver() {
	githubTestDriverOnce.Do(func() {
		sql.Register(githubTestDriverName, &githubTestDriver{})
	})
}

func (d *githubTestDriver) Open(name string) (driver.Conn, error) {
	return &githubTestConn{}, nil
}

func (c *githubTestConn) Prepare(query string) (driver.Stmt, error) {
	return &githubTestStmt{query: query}, nil
}

func (c *githubTestConn) Close() error { return nil }

func (c *githubTestConn) Begin() (driver.Tx, error) { return nil, errors.New("transactions not supported") }

func (c *githubTestConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	githubTestState.mu.Lock()
	defer githubTestState.mu.Unlock()

	if githubTestState.queryRowErr != nil {
		return nil, githubTestState.queryRowErr
	}

	if len(args) > 0 {
		if v, ok := args[0].Value.(int64); ok {
			githubTestState.lastQueryUserID = v
		}
	}

	switch {
	case queryContains(query, "INSERT INTO github_installations"):
		return &githubTestRows{
			cols: []string{"id"},
			vals: [][]driver.Value{{int64(1)}},
		}, nil
	case queryContains(query, "FROM github_installations"):
		return &githubTestRows{
			cols: []string{"id", "user_id", "installation_id", "account_type", "account_login", "status", "created_at", "updated_at"},
			vals: [][]driver.Value{{
				int64(1),
				githubTestState.lastQueryUserID,
				int64(99),
				"User",
				"octocat",
				domain.GithubInstallationStatusActive,
				time.Now(),
				time.Now(),
			}},
		}, nil
	default:
		return &githubTestRows{cols: []string{"id"}}, nil
	}
}

func (c *githubTestConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	githubTestState.mu.Lock()
	defer githubTestState.mu.Unlock()

	if githubTestState.deleteErr != nil {
		return nil, githubTestState.deleteErr
	}

	if len(args) > 0 {
		if v, ok := args[0].Value.(int64); ok {
			githubTestState.lastDeleteUser = v
		}
	}

	return githubTestResult{rowsAffected: 1}, nil
}

func (c *githubTestConn) Ping(ctx context.Context) error { return nil }

func (c *githubTestConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	return &githubTestStmt{query: query}, nil
}

func (s *githubTestStmt) Close() error { return nil }

func (s *githubTestStmt) NumInput() int { return -1 }

func (s *githubTestStmt) Exec(args []driver.Value) (driver.Result, error) {
	githubTestState.mu.Lock()
	defer githubTestState.mu.Unlock()

	if githubTestState.deleteErr != nil {
		return nil, githubTestState.deleteErr
	}

	if len(args) > 0 {
		if v, ok := args[0].(int64); ok {
			githubTestState.lastDeleteUser = v
		}
	}

	return githubTestResult{rowsAffected: 1}, nil
}

func (s *githubTestStmt) Query(args []driver.Value) (driver.Rows, error) {
	return &githubTestRows{}, nil
}

func (r githubTestResult) LastInsertId() (int64, error) { return r.insertID, nil }

func (r githubTestResult) RowsAffected() (int64, error) { return r.rowsAffected, nil }

func (r *githubTestRows) Columns() []string { return r.cols }

func (r *githubTestRows) Close() error { return nil }

func (r *githubTestRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.vals) {
		return io.EOF
	}
	row := r.vals[r.idx]
	for i := range row {
		dest[i] = row[i]
	}
	r.idx++
	return nil
}

func queryContains(query string, needle string) bool {
	return len(query) >= len(needle) && (contains(query, needle) || contains(query, needle+"\n"))
}

func contains(s, substr string) bool {
	return indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func newGithubTestDB(t *testing.T) *sql.DB {
	t.Helper()
	registerGithubTestDriver()
	db, err := sql.Open(githubTestDriverName, "")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	return db
}

func resetGithubTestState() {
	githubTestState.mu.Lock()
	defer githubTestState.mu.Unlock()
	githubTestState.lastQueryUserID = 0
	githubTestState.lastDeleteUser = 0
	githubTestState.lastUpdateUser = 0
	githubTestState.lastUpdateStatus = ""
	githubTestState.queryRowErr = nil
	githubTestState.deleteErr = nil
}

func TestStoreInstallation(t *testing.T) {
	resetGithubTestState()
	db := newGithubTestDB(t)
	defer db.Close()

	repo := NewPgSqlGithubRepository(db)
	inst := &domain.GithubInstallation{
		UserID:         7,
		InstallationID: 88,
		AccountType:   "User",
		AccountLogin:  "octocat",
		Status:         domain.GithubInstallationStatusActive,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := repo.StoreInstallation(context.Background(), inst); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if inst.ID == 0 {
		t.Fatal("expected returned ID to be set")
	}
}

func TestGetInstallationByUserID(t *testing.T) {
	resetGithubTestState()
	db := newGithubTestDB(t)
	defer db.Close()

	repo := NewPgSqlGithubRepository(db)
	inst, err := repo.GetInstallationByUserID(context.Background(), 123)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if inst.UserID != 123 {
		t.Fatalf("expected userID 123, got %d", inst.UserID)
	}
	if inst.InstallationID != 99 {
		t.Fatalf("expected installationID 99, got %d", inst.InstallationID)
	}
	if inst.Status != domain.GithubInstallationStatusActive {
		t.Fatalf("expected status %s, got %s", domain.GithubInstallationStatusActive, inst.Status)
	}
}

func TestGetInstallationByUserID_NotFound(t *testing.T) {
	resetGithubTestState()
	githubTestState.queryRowErr = sql.ErrNoRows

	db := newGithubTestDB(t)
	defer db.Close()

	repo := NewPgSqlGithubRepository(db)
	_, err := repo.GetInstallationByUserID(context.Background(), 123)
	if err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteInstallationByUserID(t *testing.T) {
	resetGithubTestState()
	db := newGithubTestDB(t)
	defer db.Close()

	repo := NewPgSqlGithubRepository(db)
	if err := repo.DeleteInstallationByUserID(context.Background(), 55); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestUpdateInstallationStatus(t *testing.T) {
	resetGithubTestState()
	db := newGithubTestDB(t)
	defer db.Close()

	repo := NewPgSqlGithubRepository(db)
	if err := repo.UpdateInstallationStatus(context.Background(), 55, domain.GithubInstallationStatusSuspended); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestUpdateInstallationStatus_InvalidStatus(t *testing.T) {
	resetGithubTestState()
	db := newGithubTestDB(t)
	defer db.Close()

	repo := NewPgSqlGithubRepository(db)
	if err := repo.UpdateInstallationStatus(context.Background(), 55, "broken"); err != domain.ErrInvalidStatus {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}
