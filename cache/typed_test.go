package cache

import (
	"testing"
	"time"

	"github.com/vuphan121/godis/config"
)

func TestTypedCache(t *testing.T) {
	cache, err := NewTypedCache[int](
		config.WithCapacity(10, 10),
		config.WithColdCleanup(time.Hour, 1, 1),
		config.WithHotDemotion(time.Hour, 1, 1, 10),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cache.Close)
	if err := cache.Set("answer", 42); err != nil {
		t.Fatal(err)
	}
	if value, ok := cache.Get("answer"); !ok || value != 42 {
		t.Fatalf("Get = (%d, %v), want (42, true)", value, ok)
	}
	if !cache.Delete("answer") || cache.Size() != 0 {
		t.Fatal("typed deletion failed")
	}
}

func TestTypedCacheStoresNil(t *testing.T) {
	cache, err := NewTypedCache[*int](
		config.WithColdCleanup(time.Hour, 1, 1),
		config.WithHotDemotion(time.Hour, 1, 1, 10),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cache.Close)
	if err := cache.Set("nil", nil); err != nil {
		t.Fatal(err)
	}
	if value, ok := cache.Get("nil"); !ok || value != nil {
		t.Fatalf("Get = (%v, %v), want (nil, true)", value, ok)
	}
}
