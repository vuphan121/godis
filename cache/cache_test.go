package cache

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/vuphan121/godis/config"
)

func newTestCache(t *testing.T, options ...config.Option) *Cache {
	t.Helper()
	base := []config.Option{
		config.WithShardCounts(2, 2),
		config.WithCMS(4, 1_024),
		config.WithCapacity(100, 100),
		config.WithHotThresholdTTL(0),
		config.WithHotMinHits(2),
		config.WithColdCleanup(time.Hour, 0.25, 1),
		config.WithColdCleanupMaxSample(100),
		config.WithHotDemotion(time.Hour, 0.25, 1, 100),
	}
	cache, err := NewCache(append(base, options...)...)
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	t.Cleanup(cache.Close)
	return cache
}

func TestSetGetOverwriteAndDelete(t *testing.T) {
	cache := newTestCache(t)
	if err := cache.Set("answer", 42); err != nil {
		t.Fatal(err)
	}
	if value, ok := cache.Get("answer"); !ok || value != 42 {
		t.Fatalf("Get = (%v, %v), want (42, true)", value, ok)
	}
	if err := cache.Set("answer", 43); err != nil {
		t.Fatal(err)
	}
	if value, ok := cache.Get("answer"); !ok || value != 43 {
		t.Fatalf("Get after overwrite = (%v, %v), want (43, true)", value, ok)
	}
	if !cache.Delete("answer") || cache.Delete("answer") {
		t.Fatal("Delete should report true once, then false")
	}
	if cache.Size() != 0 {
		t.Fatalf("size = %d, want 0", cache.Size())
	}
}

func TestTTLAndDefaultTTLOverride(t *testing.T) {
	cache := newTestCache(t, config.WithDefaultTTL(15*time.Millisecond))
	if err := cache.Set("default", "expires"); err != nil {
		t.Fatal(err)
	}
	if err := cache.Set("forever", "value", 0); err != nil {
		t.Fatal(err)
	}
	eventually(t, time.Second, func() bool {
		_, ok := cache.Get("default")
		return !ok
	})
	if value, ok := cache.Get("forever"); !ok || value != "value" {
		t.Fatalf("zero TTL override = (%v, %v), want (value, true)", value, ok)
	}
	if err := cache.Set("bad", 1, -time.Second); !errors.Is(err, ErrInvalidTTL) {
		t.Fatalf("negative TTL error = %v, want ErrInvalidTTL", err)
	}
	if err := cache.Set("bad", 1, time.Second, time.Second); !errors.Is(err, ErrInvalidTTL) {
		t.Fatalf("multiple TTL error = %v, want ErrInvalidTTL", err)
	}
}

func TestPromotionAndDemotion(t *testing.T) {
	cache := newTestCache(t)
	if err := cache.Set("popular", "value"); err != nil {
		t.Fatal(err)
	}
	cache.Get("popular")
	if got := cache.Stats().Promotions; got != 0 {
		t.Fatalf("promotions after first hit = %d, want 0", got)
	}
	cache.Get("popular")
	if got := cache.Stats().Promotions; got != 1 {
		t.Fatalf("promotions after second hit = %d, want 1", got)
	}
	if _, ok := cache.hotShard("popular").get("popular"); !ok {
		t.Fatal("popular key was not moved to hot tier")
	}

	cache.maintainHot(1, 1, 100)
	if got := cache.Stats().Demotions; got != 1 {
		t.Fatalf("demotions = %d, want 1", got)
	}
	if _, ok := cache.coldShard("popular").get("popular"); !ok {
		t.Fatal("cooled key was not moved back to cold tier")
	}
}

func TestCapacityPrefersColdLowFrequencyEntry(t *testing.T) {
	cache := newTestCache(t, config.WithCapacity(2, 10))
	if err := cache.Set("hot", 1); err != nil {
		t.Fatal(err)
	}
	cache.Get("hot")
	cache.Get("hot")
	if err := cache.Set("cold", 2); err != nil {
		t.Fatal(err)
	}
	if err := cache.Set("new", 3); err != nil {
		t.Fatal(err)
	}
	if cache.Size() != 2 {
		t.Fatalf("size = %d, want 2", cache.Size())
	}
	if _, ok := cache.Get("hot"); !ok {
		t.Fatal("hot key should survive cold-first eviction")
	}
	if _, ok := cache.Get("cold"); ok {
		t.Fatal("cold key should have been evicted")
	}
	if _, ok := cache.Get("new"); !ok {
		t.Fatal("new key is missing")
	}
	if got := cache.Stats().Evictions; got != 1 {
		t.Fatalf("evictions = %d, want 1", got)
	}
}

func TestBackgroundCleanupUpdatesSize(t *testing.T) {
	cache := newTestCache(t,
		config.WithColdCleanup(2*time.Millisecond, 1, 1),
		config.WithColdCleanupMaxSample(10),
	)
	if err := cache.Set("short", 1, 5*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	eventually(t, time.Second, func() bool { return cache.Size() == 0 })
	if got := cache.Stats().Expirations; got != 1 {
		t.Fatalf("expirations = %d, want 1", got)
	}
}

func TestCloseIsIdempotentAndRejectsWrites(t *testing.T) {
	cache := newTestCache(t)
	var wait sync.WaitGroup
	for index := 0; index < 8; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			cache.Close()
		}()
	}
	wait.Wait()
	if err := cache.Set("closed", 1); !errors.Is(err, ErrClosed) {
		t.Fatalf("Set after Close = %v, want ErrClosed", err)
	}
	if _, ok := cache.Get("closed"); ok {
		t.Fatal("Get after Close should miss")
	}
}

func TestStats(t *testing.T) {
	cache := newTestCache(t)
	if err := cache.Set("hit", 1); err != nil {
		t.Fatal(err)
	}
	cache.Get("hit")
	cache.Get("miss")
	stats := cache.Stats()
	if stats.Size != 1 || stats.Hits != 1 || stats.Misses != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestConcurrentAccessStaysBounded(t *testing.T) {
	cache := newTestCache(t,
		config.WithCapacity(50, 20),
		config.WithColdCleanup(2*time.Millisecond, 0.5, 1),
		config.WithColdCleanupMaxSample(20),
		config.WithHotDemotion(2*time.Millisecond, 0.5, 1, 20),
	)
	var wait sync.WaitGroup
	for worker := 0; worker < 12; worker++ {
		worker := worker
		wait.Add(1)
		go func() {
			defer wait.Done()
			for operation := 0; operation < 500; operation++ {
				key := fmt.Sprintf("key-%d", (worker+operation)%64)
				switch operation % 3 {
				case 0:
					_ = cache.Set(key, operation, time.Second)
				case 1:
					cache.Get(key)
				case 2:
					cache.Delete(key)
				}
			}
		}()
	}
	wait.Wait()
	assertCacheInvariants(t, cache)
	if size := cache.Size(); size < 0 || size > 50 {
		t.Fatalf("size = %d, want [0, 50]", size)
	}
}

func TestNewCacheRejectsInvalidOptions(t *testing.T) {
	if _, err := NewCache(nil); err == nil {
		t.Fatal("nil option should fail")
	}
	if _, err := NewCache(config.WithShardCounts(0, 1)); err == nil {
		t.Fatal("zero shard count should fail")
	}
}

func eventually(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}
