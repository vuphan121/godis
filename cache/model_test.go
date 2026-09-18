package cache

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/vuphan121/godis/config"
)

func TestSequentialModel(t *testing.T) {
	cache := newTestCache(t, config.WithCapacity(128, 32))
	model := make(map[string]int)
	random := rand.New(rand.NewSource(42))
	for operation := 0; operation < 10_000; operation++ {
		key := fmt.Sprintf("key-%d", random.Intn(64))
		switch random.Intn(3) {
		case 0:
			value := random.Int()
			if err := cache.Set(key, value); err != nil {
				t.Fatal(err)
			}
			model[key] = value
		case 1:
			actual, actualOK := cache.Get(key)
			expected, expectedOK := model[key]
			if actualOK != expectedOK || actualOK && actual != expected {
				t.Fatalf("Get(%q) = (%v, %v), want (%v, %v)", key, actual, actualOK, expected, expectedOK)
			}
		case 2:
			actual := cache.Delete(key)
			_, expected := model[key]
			if actual != expected {
				t.Fatalf("Delete(%q) = %v, want %v", key, actual, expected)
			}
			delete(model, key)
		}
		if operation%100 == 0 {
			assertCacheInvariants(t, cache)
		}
	}
	assertCacheInvariants(t, cache)
}

func FuzzSequentialModel(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5})
	f.Add([]byte{255, 0, 255, 1, 254, 2, 253})
	f.Fuzz(func(t *testing.T, operations []byte) {
		if len(operations) > 4_096 {
			operations = operations[:4_096]
		}
		cache, err := NewCache(
			config.WithShardCounts(2, 2),
			config.WithCMS(2, 128),
			config.WithCapacity(16, 16),
			config.WithHotThresholdTTL(0),
			config.WithColdCleanup(time.Hour, 0.5, 1),
			config.WithColdCleanupMaxSample(16),
			config.WithHotDemotion(time.Hour, 0.5, 1, 16),
		)
		if err != nil {
			t.Fatal(err)
		}
		defer cache.Close()
		model := make(map[string]byte)
		for index, operation := range operations {
			key := fmt.Sprintf("key-%d", operation%8)
			switch operation % 3 {
			case 0:
				if err := cache.Set(key, operation); err != nil {
					t.Fatal(err)
				}
				model[key] = operation
			case 1:
				actual, actualOK := cache.Get(key)
				expected, expectedOK := model[key]
				if actualOK != expectedOK || actualOK && actual != expected {
					t.Fatalf("operation %d: Get(%q) = (%v, %v), want (%v, %v)", index, key, actual, actualOK, expected, expectedOK)
				}
			case 2:
				actual := cache.Delete(key)
				_, expected := model[key]
				if actual != expected {
					t.Fatalf("operation %d: Delete(%q) = %v, want %v", index, key, actual, expected)
				}
				delete(model, key)
			}
		}
		assertCacheInvariants(t, cache)
	})
}

func assertCacheInvariants(t testing.TB, cache *Cache) {
	t.Helper()
	cache.tierMu.RLock()
	defer cache.tierMu.RUnlock()
	seen := make(map[string]struct{}, cache.size)
	for _, shards := range [][]*cacheShard{cache.hotShards, cache.coldShards} {
		for _, shard := range shards {
			for _, key := range shard.keys() {
				if _, duplicate := seen[key]; duplicate {
					t.Fatalf("key %q exists in more than one tier", key)
				}
				seen[key] = struct{}{}
			}
		}
	}
	if len(seen) != cache.size {
		t.Fatalf("resident keys = %d, tracked size = %d", len(seen), cache.size)
	}
	if cache.size < 0 || cache.size > cache.maxEntries {
		t.Fatalf("size = %d, capacity = %d", cache.size, cache.maxEntries)
	}
}
