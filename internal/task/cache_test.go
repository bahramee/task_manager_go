package task

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type memoryListCache struct{ values map[string]string }

func newMemoryListCache() *memoryListCache { return &memoryListCache{values: map[string]string{}} }
func (c *memoryListCache) Get(_ context.Context, key string) (string, error) {
	value, ok := c.values[key]
	if !ok {
		return "", ErrCacheMiss
	}
	return value, nil
}
func (c *memoryListCache) Set(_ context.Context, key, value string, _ time.Duration) error {
	c.values[key] = value
	return nil
}
func (c *memoryListCache) DeletePrefix(_ context.Context, prefix string) error {
	for key := range c.values {
		if strings.HasPrefix(key, prefix) {
			delete(c.values, key)
		}
	}
	return nil
}
func (*memoryListCache) Close() error { return nil }

func TestCachedRepositoryCachesFilteredPagesAndInvalidates(t *testing.T) {
	ctx := context.Background()
	underlying := &fakeRepo{getTask: Task{ID: "task", Title: "from db"}}
	cache := newMemoryListCache()
	repo := NewCachedRepository(underlying, cache, time.Minute)
	options := ListOptions{Status: Pending, Assignee: "Alex", Page: 1, PageSize: 10}
	first, total, err := repo.List(ctx, options)
	if err != nil || total != 1 || len(first) != 1 || underlying.listCalls != 1 {
		t.Fatalf("first List = %+v, %d, %v; calls=%d", first, total, err, underlying.listCalls)
	}
	if _, _, err := repo.List(ctx, options); err != nil || underlying.listCalls != 1 {
		t.Fatalf("second List missed cache: calls=%d, err=%v", underlying.listCalls, err)
	}
	otherOptions := options
	otherOptions.Page = 2
	if _, _, err := repo.List(ctx, otherOptions); err != nil || underlying.listCalls != 2 {
		t.Fatalf("different page reused cache: calls=%d, err=%v", underlying.listCalls, err)
	}
	if _, err := repo.Create(ctx, Input{Title: "new"}); err != nil || len(cache.values) != 0 {
		t.Fatalf("Create did not invalidate lists: err=%v cache=%v", err, cache.values)
	}
	_, _, _ = repo.List(ctx, options)
	if _, err := repo.Update(ctx, "task", Input{Title: "changed"}); err != nil || len(cache.values) != 0 {
		t.Fatalf("Update did not invalidate lists: err=%v cache=%v", err, cache.values)
	}
	_, _, _ = repo.List(ctx, options)
	if err := repo.Delete(ctx, "task"); err != nil || len(cache.values) != 0 {
		t.Fatalf("Delete did not invalidate lists: err=%v cache=%v", err, cache.values)
	}
}

func TestCachedRepositoryFallsBackOnRedisErrors(t *testing.T) {
	cache := &failingListCache{}
	underlying := &fakeRepo{getTask: Task{Title: "db"}}
	repo := NewCachedRepository(underlying, cache, time.Minute)
	items, total, err := repo.List(context.Background(), ListOptions{Page: 1, PageSize: 20})
	if err != nil || total != 1 || len(items) != 1 || underlying.listCalls != 1 {
		t.Fatalf("List did not fall back to repository: %+v %d %v", items, total, err)
	}
}

type failingListCache struct{}

func (*failingListCache) Get(context.Context, string) (string, error) {
	return "", errors.New("redis offline")
}
func (*failingListCache) Set(context.Context, string, string, time.Duration) error {
	return errors.New("redis offline")
}
func (*failingListCache) DeletePrefix(context.Context, string) error {
	return errors.New("redis offline")
}
func (*failingListCache) Close() error { return nil }

func BenchmarkListTasksCacheHit(b *testing.B) {
	underlying := &fakeRepo{getTask: Task{ID: "task", Title: "benchmark"}}
	repo := NewCachedRepository(underlying, newMemoryListCache(), time.Hour)
	options := ListOptions{Status: Pending, Page: 1, PageSize: 20}
	ctx := context.Background()
	if _, _, err := repo.List(ctx, options); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := repo.List(ctx, options); err != nil {
			b.Fatal(err)
		}
	}
}
