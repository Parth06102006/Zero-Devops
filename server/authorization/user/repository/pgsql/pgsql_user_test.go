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

const userTestDriverName = "user_repo_test_driver"

var (
	userDriverOnce sync.Once
	userState      = &userDBState{}
)

type userDBState struct {
	mu          sync.Mutex
	lastID      int64
	queryErr    error
	prepareErr  error
	execErr     error
}

type userDriver struct{}
type userConn struct{}
type userStmt struct{}
type userRows struct {
	cols []string
	vals [][]driver.Value
	idx  int
}
type userResult struct{ rowsAffected int64 }

func registerUserDriver() {
	userDriverOnce.Do(func() {
		sql.Register(userTestDriverName, &userDriver{})
	})
}

func (d *userDriver) Open(name string) (driver.Conn, error) { return &userConn{}, nil }
func (c *userConn) Close() error                             { return nil }
func (c *userConn) Begin() (driver.Tx, error)                { return nil, errors.New("tx not supported") }
func (c *userConn) Prepare(query string) (driver.Stmt, error) { return &userStmt{}, nil }
func (c *userConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	return &userStmt{}, nil
}

func (c *userConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	userState.mu.Lock()
	defer userState.mu.Unlock()
	if userState.queryErr != nil {
		return nil, userState.queryErr
	}
	if len(args) > 0 {
		if v, ok := args[0].Value.(int64); ok {
			userState.lastID = v
		}
	}
	if contains(query, "RETURNING id") {
		return &userRows{
			cols: []string{"id"},
			vals: [][]driver.Value{{int64(1)}},
		}, nil
	}
	return &userRows{
		cols: []string{"id", "provider_id", "provider", "username", "email", "avatar_url", "created_at", "refresh_token"},
		vals: [][]driver.Value{{
			int64(1), int64(99), "github", "octocat", "octo@example.com", "https://example.com/a.png", time.Now(), "refresh",
		}},
	}, nil
}

func (c *userConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return userResult{rowsAffected: 1}, nil
}

func (s *userStmt) Close() error { return nil }
func (s *userStmt) NumInput() int { return -1 }
func (s *userStmt) Exec(args []driver.Value) (driver.Result, error) { return userResult{rowsAffected: 1}, nil }
func (s *userStmt) Query(args []driver.Value) (driver.Rows, error) { return &userRows{}, nil }
func (r userResult) LastInsertId() (int64, error) { return 1, nil }
func (r userResult) RowsAffected() (int64, error) { return r.rowsAffected, nil }
func (r *userRows) Columns() []string { return r.cols }
func (r *userRows) Close() error { return nil }
func (r *userRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.vals) {
		return io.EOF
	}
	for i, v := range r.vals[r.idx] {
		dest[i] = v
	}
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

func newUserTestDB(t *testing.T) *sql.DB {
	t.Helper()
	registerUserDriver()
	db, err := sql.Open(userTestDriverName, "")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

func resetUserState() {
	userState.mu.Lock()
	defer userState.mu.Unlock()
	userState.lastID = 0
	userState.queryErr = nil
	userState.prepareErr = nil
	userState.execErr = nil
}

func TestGetByID(t *testing.T) {
	resetUserState()
	db := newUserTestDB(t)
	defer db.Close()

	repo := NewPgSqlUserRepository(db)
	got, err := repo.GetByID(context.Background(), 123)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got.ID != 1 || got.Username != "octocat" {
		t.Fatalf("unexpected user: %+v", got)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	resetUserState()
	userState.queryErr = sql.ErrNoRows

	db := newUserTestDB(t)
	defer db.Close()

	repo := NewPgSqlUserRepository(db)
	_, err := repo.GetByID(context.Background(), 123)
	if err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestStore(t *testing.T) {
	resetUserState()
	db := newUserTestDB(t)
	defer db.Close()

	repo := NewPgSqlUserRepository(db)
	u := &domain.User{ProviderId: 55, Provider: "github", Username: "octocat", Email: "octo@example.com", CreatedAt: time.Now()}
	if err := repo.Store(context.Background(), u); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if u.ID == 0 {
		t.Fatal("expected user ID to be set")
	}
}

func TestUpdateRefreshToken(t *testing.T) {
	resetUserState()
	db := newUserTestDB(t)
	defer db.Close()

	repo := NewPgSqlUserRepository(db)
	if err := repo.UpdateRefreshToken(context.Background(), 1, "new-token"); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}
