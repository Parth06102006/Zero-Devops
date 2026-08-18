// Package cache provides the repository-listing cache used by the GitHub SCM
// usecase. Redis is a cache only: a miss or any backend failure must be treated
// as "go to GitHub", never as an authorization decision.
package cache

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"Zero_Devops/server/internal/domain"

	"github.com/redis/go-redis/v9"
)

// KeyPrefix is the stable Redis key prefix for cached installation repository
// lists. Installation-scoped invalidation scans for this prefix plus the
// installation ID, so changing the key shape must keep this prefix intact.
const KeyPrefix = "github:installation:"

// RepositoryListCache is the minimal cache surface used for repository listing.
// Implementations must treat both a missing key and a backend failure as a miss
// (Get returns a nil list); they must never cache tokens, secrets, or cookies.
type RepositoryListCache interface {
	Get(ctx context.Context, key string) (*domain.RepositoryList, error)
	Set(ctx context.Context, key string, value *domain.RepositoryList, ttl time.Duration) error
	InvalidateInstallation(ctx context.Context, installationID int64) error
}

// RedisRepositoryListCache implements RepositoryListCache over go-redis.
type RedisRepositoryListCache struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisRepositoryListCache wraps a *redis.Client. ttl is the default cache
// lifetime; callers may override per Set.
func NewRedisRepositoryListCache(client *redis.Client, ttl time.Duration) *RedisRepositoryListCache {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &RedisRepositoryListCache{client: client, ttl: ttl}
}

// Get returns the cached list or nil on miss/error. The caller distinguishes a
// deliberate miss (nil, nil) from a backend error (nil, err) only to decide
// whether to log; either way it falls through to GitHub.
func (c *RedisRepositoryListCache) Get(ctx context.Context, key string) (*domain.RepositoryList, error) {
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}
	var list domain.RepositoryList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	return &list, nil
}

// Set stores the picker list. It never stores installation tokens or secrets.
func (c *RedisRepositoryListCache) Set(ctx context.Context, key string, value *domain.RepositoryList, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if ttl <= 0 {
		ttl = c.ttl
	}
	return c.client.Set(ctx, key, data, ttl).Err()
}

// InvalidateInstallation deletes every cached repository list for the given
// installation. TTL is the fallback for missed webhook events; Redis must never
// decide authorization.
func (c *RedisRepositoryListCache) InvalidateInstallation(ctx context.Context, installationID int64) error {
	pattern := KeyPrefix + strconv.FormatInt(installationID, 10) + ":repositories:v1:*"
	iter := c.client.Scan(ctx, 0, pattern, 0).Iterator()
	var firstErr error
	for iter.Next(ctx) {
		if err := c.client.Del(ctx, iter.Val()).Err(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if err := iter.Err(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}
