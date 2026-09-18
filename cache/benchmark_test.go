package cache

import (
	"strconv"
	"testing"
	"time"

	"github.com/vuphan121/godis/config"
)

func BenchmarkCacheReadMostly(b *testing.B) {
	cache, err := NewCache(
		config.WithCapacity(10_000, 32),
		config.WithColdCleanup(time.Hour, 0.25, 5),
		config.WithHotDemotion(time.Hour, 0.05, 5, 1_000),
	)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(cache.Close)
	for index := 0; index < 1_000; index++ {
		if err := cache.Set(strconv.Itoa(index), index); err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	b.RunParallel(func(parallel *testing.PB) {
		index := 0
		for parallel.Next() {
			cache.Get(strconv.Itoa(index % 1_000))
			index++
		}
	})
}

func BenchmarkCacheWriteChurn(b *testing.B) {
	cache, err := NewCache(
		config.WithCapacity(1_000, 32),
		config.WithColdCleanup(time.Hour, 0.25, 5),
		config.WithHotDemotion(time.Hour, 0.05, 5, 1_000),
	)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(cache.Close)
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if err := cache.Set(strconv.Itoa(index%2_000), index); err != nil {
			b.Fatal(err)
		}
	}
}
