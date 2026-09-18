package cache

import (
	"context"
	"errors"
	"hash/fnv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vuphan121/godis/config"
)

var (
	ErrClosed     = errors.New("cache is closed")
	ErrInvalidTTL = errors.New("TTL must be non-negative and specified at most once")
	ErrCapacity   = errors.New("cache is at capacity")
)

type Cache struct {
	hotShards  []*cacheShard
	coldShards []*cacheShard

	hotShardCount  int
	coldShardCount int
	defaultTTL     time.Duration
	maxEntries     int
	evictionSample int

	cms               *CountMinSketch
	hotReadPercentage float64
	minHotTTL         time.Duration
	hotMinHits        uint
	hotThreshold      atomic.Uint64

	tierMu sync.RWMutex
	size   int

	ctx       context.Context
	cancel    context.CancelFunc
	workers   sync.WaitGroup
	closeOnce sync.Once
	closed    atomic.Bool
	done      chan struct{}

	metrics cacheMetrics
	now     func() time.Time
}

func NewCache(options ...config.Option) (*Cache, error) {
	cfg := config.DefaultConfig()
	for _, option := range options {
		if option == nil {
			return nil, errors.New("cache option must not be nil")
		}
		option(&cfg)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	cms, err := NewCountMinSketch(cfg.CMSDepth, cfg.CMSWidth)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	c := &Cache{
		hotShards:         make([]*cacheShard, cfg.HotShardCount),
		coldShards:        make([]*cacheShard, cfg.ColdShardCount),
		hotShardCount:     cfg.HotShardCount,
		coldShardCount:    cfg.ColdShardCount,
		defaultTTL:        cfg.DefaultTTL,
		maxEntries:        cfg.MaxEntries,
		evictionSample:    cfg.EvictionSampleSize,
		cms:               cms,
		hotReadPercentage: cfg.HotReadPercentage,
		minHotTTL:         cfg.HotThresholdTTL,
		hotMinHits:        cfg.HotMinHits,
		ctx:               ctx,
		cancel:            cancel,
		done:              make(chan struct{}),
		now:               time.Now,
	}
	c.hotThreshold.Store(uint64(cfg.HotMinHits))
	for index := range c.hotShards {
		c.hotShards[index] = newCacheShard()
	}
	for index := range c.coldShards {
		c.coldShards[index] = newCacheShard()
	}

	c.startColdCleanup(cfg)
	c.startHotDemotion(cfg)
	return c, nil
}

func getShardIndex(key string, shardCount int) int {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(key))
	return int(hash.Sum32() % uint32(shardCount))
}

func (c *Cache) Set(key string, value any, ttl ...time.Duration) error {
	if c.closed.Load() {
		return ErrClosed
	}
	if len(ttl) > 1 {
		return ErrInvalidTTL
	}
	duration := c.defaultTTL
	if len(ttl) == 1 {
		duration = ttl[0]
	}
	if duration < 0 {
		return ErrInvalidTTL
	}
	now := c.now()
	item := &entry{value: value, createdAt: now}
	if duration > 0 {
		item.expiration = now.Add(duration)
	}

	c.tierMu.RLock()
	if c.closed.Load() {
		c.tierMu.RUnlock()
		return ErrClosed
	}
	if c.replaceExisting(key, item) {
		c.tierMu.RUnlock()
		return nil
	}
	c.tierMu.RUnlock()

	c.tierMu.Lock()
	defer c.tierMu.Unlock()
	if c.closed.Load() {
		return ErrClosed
	}
	if c.replaceExisting(key, item) {
		return nil
	}
	if c.size >= c.maxEntries && !c.evictOneLocked(now) {
		return ErrCapacity
	}
	c.coldShard(key).set(key, item)
	c.size++
	return nil
}

func (c *Cache) replaceExisting(key string, item *entry) bool {
	hot := c.hotShard(key)
	if _, ok := hot.get(key); ok {
		hot.set(key, item)
		return true
	}
	cold := c.coldShard(key)
	if _, ok := cold.get(key); ok {
		cold.set(key, item)
		return true
	}
	return false
}

func (c *Cache) Get(key string) (any, bool) {
	if c.closed.Load() {
		return nil, false
	}
	now := c.now()
	c.tierMu.RLock()
	if item, ok := c.hotShard(key).get(key); ok {
		if item.expired(now) {
			c.tierMu.RUnlock()
			c.removeExpired(key, item, true)
			c.metrics.misses.Add(1)
			return nil, false
		}
		value := item.value
		c.cms.Add(key)
		c.tierMu.RUnlock()
		c.metrics.hits.Add(1)
		return value, true
	}

	item, ok := c.coldShard(key).get(key)
	if !ok {
		c.tierMu.RUnlock()
		c.metrics.misses.Add(1)
		return nil, false
	}
	if item.expired(now) {
		c.tierMu.RUnlock()
		c.removeExpired(key, item, false)
		c.metrics.misses.Add(1)
		return nil, false
	}
	value := item.value
	c.cms.Add(key)
	shouldPromote := item.hotEligible(now, c.minHotTTL) && c.IsHotKey(key)
	c.tierMu.RUnlock()
	c.metrics.hits.Add(1)
	if shouldPromote {
		c.promote(key, item, now)
	}
	return value, true
}

func (c *Cache) Delete(key string) bool {
	if c.closed.Load() {
		return false
	}
	c.tierMu.Lock()
	defer c.tierMu.Unlock()
	removedHot := c.hotShard(key).delete(key)
	removedCold := c.coldShard(key).delete(key)
	if removedHot || removedCold {
		c.size--
		return true
	}
	return false
}

func (c *Cache) IsHotKey(key string) bool {
	return uint64(c.cms.Count(key)) >= c.hotThreshold.Load()
}

func (c *Cache) Size() int {
	c.tierMu.RLock()
	defer c.tierMu.RUnlock()
	return c.size
}

func (c *Cache) Stats() Stats {
	return Stats{
		Size:        c.Size(),
		Hits:        c.metrics.hits.Load(),
		Misses:      c.metrics.misses.Load(),
		Promotions:  c.metrics.promotions.Load(),
		Demotions:   c.metrics.demotions.Load(),
		Evictions:   c.metrics.evictions.Load(),
		Expirations: c.metrics.expirations.Load(),
	}
}

func (c *Cache) Close() {
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		c.cancel()
		c.workers.Wait()
		close(c.done)
	})
	<-c.done
}

func (c *Cache) hotShard(key string) *cacheShard {
	return c.hotShards[getShardIndex(key, c.hotShardCount)]
}

func (c *Cache) coldShard(key string) *cacheShard {
	return c.coldShards[getShardIndex(key, c.coldShardCount)]
}

func (c *Cache) promote(key string, expected *entry, now time.Time) {
	c.tierMu.Lock()
	defer c.tierMu.Unlock()
	if c.closed.Load() {
		return
	}
	cold := c.coldShard(key)
	current, ok := cold.get(key)
	if !ok || current != expected || current.expired(now) || !current.hotEligible(now, c.minHotTTL) || !c.IsHotKey(key) {
		return
	}
	if !cold.deleteIf(key, current) {
		return
	}
	c.hotShard(key).set(key, current)
	c.metrics.promotions.Add(1)
}

func (c *Cache) removeExpired(key string, expected *entry, hot bool) {
	c.tierMu.Lock()
	defer c.tierMu.Unlock()
	shard := c.coldShard(key)
	if hot {
		shard = c.hotShard(key)
	}
	if shard.deleteIf(key, expected) {
		c.size--
		c.metrics.expirations.Add(1)
	}
}

func (c *Cache) evictOneLocked(now time.Time) bool {
	candidate, ok := c.evictionCandidateLocked(c.coldShards, now)
	hot := false
	if !ok {
		candidate, ok = c.evictionCandidateLocked(c.hotShards, now)
		hot = true
	}
	if !ok {
		return false
	}
	shard := c.coldShard(candidate.key)
	if hot {
		shard = c.hotShard(candidate.key)
	}
	if !shard.deleteIf(candidate.key, candidate.entry) {
		return false
	}
	c.size--
	if candidate.entry.expired(now) {
		c.metrics.expirations.Add(1)
	} else {
		c.metrics.evictions.Add(1)
	}
	return true
}

func (c *Cache) evictionCandidateLocked(shards []*cacheShard, now time.Time) (sampledEntry, bool) {
	if len(shards) == 0 {
		return sampledEntry{}, false
	}
	var chosen sampledEntry
	chosenSet := false
	chosenCount := uint(0)
	for _, candidate := range sampleShards(shards, c.evictionSample) {
		if candidate.entry.expired(now) {
			return candidate, true
		}
		count := c.cms.Count(candidate.key)
		if !chosenSet || count < chosenCount || (count == chosenCount && candidate.entry.createdAt.Before(chosen.entry.createdAt)) {
			chosen = candidate
			chosenCount = count
			chosenSet = true
		}
	}
	return chosen, chosenSet
}
