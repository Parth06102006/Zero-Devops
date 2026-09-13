package cache

import (
	"context"
	"testing"

	"Zero_Devops/server/internal/domain"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newMiniRedisCache(t *testing.T) (*RedisRepositoryListCache, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewRedisRepositoryListCache(rdb, 0), mr
}

func TestRedisCache_MissReturnsNil(t *testing.T) {
	c, _ := newMiniRedisCache(t)
	got, err := c.Get(context.Background(), "github:installation:10:repositories:v1:deadbeef")
	if got != nil {
		t.Fatalf("expected nil list on cache miss, got %+v", got)
	}
	// A missing key surfaces as redis.Nil; the usecase treats any backend error
	// as a cache miss, so we only assert that no list was returned.
	if err == nil {
		t.Fatalf("expected redis.Nil error on miss, got nil")
	}
}

func TestRedisCache_SetThenGet(t *testing.T) {
	c, _ := newMiniRedisCache(t)
	key := "github:installation:10:repositories:v1:deadbeef"
	want := &domain.RepositoryList{Repositories: []domain.RepositoryPicker{{ID: 1, Name: "a"}}}

	if err := c.Set(context.Background(), key, want, 0); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := c.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil || len(got.Repositories) != 1 || got.Repositories[0].Name != "a" {
		t.Fatalf("unexpected cached value: %+v", got)
	}
}

func TestRedisCache_InvalidateInstallation(t *testing.T) {
	c, mr := newMiniRedisCache(t)
	ctx := context.Background()

	keyA1 := "github:installation:10:repositories:v1:aaaa"
	keyA2 := "github:installation:10:repositories:v1:bbbb"
	keyB := "github:installation:20:repositories:v1:cccc"
	if err := c.Set(ctx, keyA1, &domain.RepositoryList{}, 0); err != nil {
		t.Fatal(err)
	}
	if err := c.Set(ctx, keyA2, &domain.RepositoryList{}, 0); err != nil {
		t.Fatal(err)
	}
	if err := c.Set(ctx, keyB, &domain.RepositoryList{}, 0); err != nil {
		t.Fatal(err)
	}

	if err := c.InvalidateInstallation(ctx, 10); err != nil {
		t.Fatalf("invalidate: %v", err)
	}

	if mr.Exists(keyA1) {
		t.Fatalf("expected key for installation 10 to be evicted: %s", keyA1)
	}
	if mr.Exists(keyA2) {
		t.Fatalf("expected key for installation 10 to be evicted: %s", keyA2)
	}
	if !mr.Exists(keyB) {
		t.Fatalf("expected key for installation 20 to survive: %s", keyB)
	}
}
