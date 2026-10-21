package task

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct{ client *redis.Client }

func NewRedisCache(address, password string, database int) *RedisCache {
	return &RedisCache{client: redis.NewClient(&redis.Options{Addr: address, Password: password, DB: database})}
}
func (c *RedisCache) Get(ctx context.Context, key string) (string, error) {
	value, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", ErrCacheMiss
	}
	return value, err
}
func (c *RedisCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}
func (c *RedisCache) DeletePrefix(ctx context.Context, prefix string) error {
	var cursor uint64
	for {
		keys, next, err := c.client.Scan(ctx, cursor, prefix+"*", 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}
func (c *RedisCache) Close() error { return c.client.Close() }
