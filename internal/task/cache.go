package task

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

var ErrCacheMiss = errors.New("cache miss")

// ListCache is the small Redis surface needed for task list caching.
type ListCache interface {
	Get(context.Context, string) (string, error)
	Set(context.Context, string, string, time.Duration) error
	DeletePrefix(context.Context, string) error
	Close() error
}

type CachedRepository struct {
	next  Repository
	cache ListCache
	ttl   time.Duration
}

func NewCachedRepository(next Repository, cache ListCache, ttl time.Duration) *CachedRepository {
	return &CachedRepository{next: next, cache: cache, ttl: ttl}
}

func listCacheKey(options ListOptions) string {
	serialized, _ := json.Marshal(options)
	sum := sha256.Sum256(serialized)
	return "task:list:" + hex.EncodeToString(sum[:])
}

func (r *CachedRepository) List(ctx context.Context, options ListOptions) ([]Task, int, error) {
	key := listCacheKey(options)
	if value, err := r.cache.Get(ctx, key); err == nil {
		var result cachedList
		if json.Unmarshal([]byte(value), &result) == nil {
			return result.Items, result.Total, nil
		}
		_ = r.cache.DeletePrefix(ctx, key)
	}
	items, total, err := r.next.List(ctx, options)
	if err != nil {
		return nil, 0, err
	}
	value, err := json.Marshal(cachedList{Items: items, Total: total})
	if err == nil {
		_ = r.cache.Set(ctx, key, string(value), r.ttl)
	}
	return items, total, nil
}

type cachedList struct {
	Items []Task `json:"items"`
	Total int    `json:"total"`
}

func (r *CachedRepository) invalidate(ctx context.Context) {
	_ = r.cache.DeletePrefix(ctx, "task:list:")
}

func (r *CachedRepository) Create(ctx context.Context, input Input) (Task, error) {
	created, err := r.next.Create(ctx, input)
	if err == nil {
		r.invalidate(ctx)
	}
	return created, err
}
func (r *CachedRepository) Get(ctx context.Context, id string) (Task, error) {
	return r.next.Get(ctx, id)
}
func (r *CachedRepository) Update(ctx context.Context, id string, input Input) (Task, error) {
	updated, err := r.next.Update(ctx, id, input)
	if err == nil {
		r.invalidate(ctx)
	}
	return updated, err
}
func (r *CachedRepository) Delete(ctx context.Context, id string) error {
	err := r.next.Delete(ctx, id)
	if err == nil {
		r.invalidate(ctx)
	}
	return err
}
func (r *CachedRepository) Count(ctx context.Context) (int, error) {
	return r.next.Count(ctx)
}
