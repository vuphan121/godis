package cache

import (
	"fmt"
	"strconv"
	"sync/atomic"
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

func BenchmarkTypedCacheReadMostly(b *testing.B) {
	cache, err := NewTypedCache[int](
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

func BenchmarkCacheParallelWrites(b *testing.B) {
	for _, shardCount := range []int{1, 4, 16, 64} {
		b.Run(fmt.Sprintf("shards-%d", shardCount), func(b *testing.B) {
			benchmarkCacheParallelWrites(b, shardCount)
		})
	}
}

func benchmarkCacheParallelWrites(b *testing.B, shardCount int) {
	cache, err := NewCache(
		config.WithShardCounts(shardCount, shardCount),
		config.WithCapacity(10_000, 64),
		config.WithColdCleanup(time.Hour, 0.25, 5),
		config.WithHotDemotion(time.Hour, 0.05, 5, 1_000),
	)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(cache.Close)
	for index := 0; index < 1_024; index++ {
		if err := cache.Set(strconv.Itoa(index), index); err != nil {
			b.Fatal(err)
		}
	}
	var sequence atomic.Uint64
	b.ResetTimer()
	b.RunParallel(func(parallel *testing.PB) {
		for parallel.Next() {
			value := sequence.Add(1)
			if err := cache.Set(strconv.FormatUint(value%1_024, 10), value); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkCountMinDecay(b *testing.B) {
	sketch, err := NewCountMinSketch(4, 50_000)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		sketch.Decay()
	}
}

func BenchmarkCountMinEagerDecayReference(b *testing.B) {
	table := make([][]uint, 4)
	for row := range table {
		table[row] = make([]uint, 50_000)
		for column := range table[row] {
			table[row][column] = ^uint(0)
		}
	}
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		for row := range table {
			for column := range table[row] {
				table[row][column] /= 2
			}
		}
	}
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
