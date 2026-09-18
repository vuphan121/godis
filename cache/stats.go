package cache

import "sync/atomic"

type Stats struct {
	Size        int
	Hits        uint64
	Misses      uint64
	Promotions  uint64
	Demotions   uint64
	Evictions   uint64
	Expirations uint64
}

type cacheMetrics struct {
	hits        atomic.Uint64
	misses      atomic.Uint64
	promotions  atomic.Uint64
	demotions   atomic.Uint64
	evictions   atomic.Uint64
	expirations atomic.Uint64
}
