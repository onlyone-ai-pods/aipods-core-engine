package rag

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisOpTimeout bounds every cache call so a slow Redis never stalls a chat.
const redisOpTimeout = 300 * time.Millisecond

// RedisCacheBackend persists the semantic cache in Redis (SPEC-CORE-71.R5).
type RedisCacheBackend struct {
	client *redis.Client
}

// NewRedisCacheBackend accepts "host:port" (as in docker-compose) or a
// redis:// / rediss:// URL.
func NewRedisCacheBackend(redisURL string) (*RedisCacheBackend, error) {
	redisURL = strings.TrimSpace(redisURL)
	if redisURL == "" {
		return nil, errors.New("redis cache: REDIS_URL is empty")
	}
	var opts *redis.Options
	if strings.Contains(redisURL, "://") {
		parsed, err := redis.ParseURL(redisURL)
		if err != nil {
			return nil, fmt.Errorf("redis cache: invalid REDIS_URL: %w", err)
		}
		opts = parsed
	} else {
		opts = &redis.Options{Addr: redisURL}
	}
	return &RedisCacheBackend{client: redis.NewClient(opts)}, nil
}

// Ping checks connectivity (used at boot for a log line only).
func (r *RedisCacheBackend) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return r.client.Ping(ctx).Err()
}

func (r *RedisCacheBackend) Close() error { return r.client.Close() }

func (r *RedisCacheBackend) Get(ctx context.Context, key string) ([]byte, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, redisOpTimeout)
	defer cancel()
	v, err := r.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("redis get: %w", err)
	}
	return v, true, nil
}

func (r *RedisCacheBackend) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, redisOpTimeout)
	defer cancel()
	if err := r.client.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}
	return nil
}

func (r *RedisCacheBackend) Del(ctx context.Context, key string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, redisOpTimeout)
	defer cancel()
	n, err := r.client.Del(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("redis del: %w", err)
	}
	return n > 0, nil
}
