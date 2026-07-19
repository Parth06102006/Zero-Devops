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

const deploymentsTestDriverName = "deployment_repo_test_driver"

var (
	deploymentsDriverOnce sync.Once
	deploymentsState      = &deploymentsDBState{}
)

type deploymentsDBState struct {
	mu          sync.Mutex
	lastUserID  int64
	lastStore   *domain.Deployment
	queryRowErr error
	queryErr    error
	rowsErr     error
	getByIDErr  error
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
		if v, ok := args[0].Value.(int64); ok {
			deploymentsState.lastUserID = v
		}
	}
	if deploymentsState.rowsErr != nil {
		return nil, deploymentsState.rowsErr
	}
	if contains(query, "RETURNING id") {
		return &deploymentsRows{
			cols: []string{"id"},
			vals: [][]driver.Value{{int64(1)}},
		}, nil
	}
	if deploymentsState.queryRowErr == sql.ErrNoRows {
		return &deploymentsRows{
			cols: []string{"id", "user_id", "repo_id", "clone_url", "status", "created_at", "updated_at"},
			vals: [][]driver.Value{},
		}, nil
	}
	return &deploymentsRows{
		cols: []string{"id", "user_id", "repo_id", "clone_url", "status", "created_at", "updated_at"},
		vals: [][]driver.Value{{
			int64(1), deploymentsState.lastUserID, int64(22),
			"https://example.com/repo.git", domain.DeploymentStatusPending, time.Now(), time.Now(),
		}},
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
	deploymentsState.lastUserID = 0
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
		UserID: 7, RepoID: 8, CloneURL: "https://example.com/x.git",
		Status: domain.DeploymentStatusPending, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := repo.Store(context.Background(), d); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if d.ID == 0 {
		t.Fatal("expected ID to be set")
	}
}

func TestGetByUserID(t *testing.T) {
	resetDeploymentsState()
	db := newDeploymentsTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewPgSQLDeploymentRepository(db)
	got, err := repo.GetByUserID(context.Background(), 44)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(got) != 1 || got[0].UserID != 44 {
		t.Fatalf("unexpected deployments: %+v", got)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	resetDeploymentsState()
	deploymentsState.queryRowErr = sql.ErrNoRows

	db := newDeploymentsTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewPgSQLDeploymentRepository(db)
	_, err := repo.GetByID(context.Background(), 1, 2)
	if err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
