package cache

import (
	"context"
	"hash/fnv"
	"time"
)

type Cache struct {
	HotShards  []*CacheShard
	ColdShards []*CacheShard

	hotShardCount  int
	coldShardCount int
	defaultTTL     time.Duration

	cms               *CountMinSketch
	hotReadPercentage float64

	hotThreshold        uint
	hotThresholdUpdated time.Time
	hotThresholdTTL     time.Duration

	ctx    context.Context
	cancel context.CancelFunc
}

func NewCache(defaultTTL time.Duration, hotShardCount, coldShardCount int, hotReadPercentage float64) *Cache {
	ctx, cancel := context.WithCancel(context.Background())

	c := &Cache{
		HotShards:         make([]*CacheShard, hotShardCount),
		ColdShards:        make([]*CacheShard, coldShardCount),
		hotShardCount:     hotShardCount,
		coldShardCount:    coldShardCount,
		defaultTTL:        defaultTTL,
		cms:               NewCountMinSketch(4, 50000),
		hotReadPercentage: hotReadPercentage,
		hotThresholdTTL:   2000 * time.Millisecond,
		ctx:               ctx,
		cancel:            cancel,
	}

	for i := 0; i < hotShardCount; i++ {
		c.HotShards[i] = NewCacheShard()
	}
	for i := 0; i < coldShardCount; i++ {
		c.ColdShards[i] = NewCacheShard()
	}

	StartColdCacheCleanup(c, 100*time.Millisecond, 0.25, 5)
	StartHotDemotion(c)

	return c
}

func getShardIndex(key string, shardCount int) int {
	h := fnv.New32a()
	if _, err := h.Write([]byte(key)); err != nil {
		return 0
	}
	return int(h.Sum32()) % shardCount
}

func (c *Cache) Set(key string, value interface{}, ttl ...time.Duration) {
	var expiration time.Time
	if len(ttl) > 0 && ttl[0] > 0 {
		expiration = time.Now().Add(ttl[0])
	} else if c.defaultTTL > 0 {
		expiration = time.Now().Add(c.defaultTTL)
	}

	entry := &Entry{
		Value:      value,
		Expiration: expiration,
	}

	hotShard := c.HotShards[getShardIndex(key, c.hotShardCount)]
	if _, ok := hotShard.Get(key); ok {
		hotShard.Set(key, entry)
		return
	}

	coldShard := c.ColdShards[getShardIndex(key, c.coldShardCount)]
	coldShard.Set(key, entry)
}

func (c *Cache) Get(key string) (interface{}, bool) {
	c.cms.Add(key)

	hotShard := c.HotShards[getShardIndex(key, c.hotShardCount)]
	if entry, ok := hotShard.Get(key); ok {
		if !entry.Expiration.IsZero() && time.Now().After(entry.Expiration) {
			hotShard.Delete(key)
			c.cms.Reset(key)
			return nil, false
		}

		return entry.Value, true
	}

	coldShard := c.ColdShards[getShardIndex(key, c.coldShardCount)]
	if entry, ok := coldShard.Get(key); ok {
		if !entry.Expiration.IsZero() && time.Now().After(entry.Expiration) {
			coldShard.Delete(key)
			c.cms.Reset(key)
			return nil, false
		}

		if c.IsHotKey(key) {
			c.promoteColdKey(key, entry)
		}

		return entry.Value, true
	}

	return nil, false
}

func (c *Cache) Delete(key string) {
	hotShard := c.HotShards[getShardIndex(key, c.hotShardCount)]
	hotShard.Delete(key)

	coldShard := c.ColdShards[getShardIndex(key, c.coldShardCount)]
	coldShard.Delete(key)

	c.cms.Reset(key)
}

func (c *Cache) IsHotKey(key string) bool {
	count := c.cms.Count(key)
	return count >= c.hotThreshold
}

func (c *Cache) promoteColdKey(key string, entry *Entry) {
	coldShard := c.ColdShards[getShardIndex(key, c.coldShardCount)]
	hotShard := c.HotShards[getShardIndex(key, c.hotShardCount)]

	hotShard.Set(key, entry)
	coldShard.Delete(key)
}

func (c *Cache) demoteHotKey(key string, entry *Entry) {
	hotShard := c.HotShards[getShardIndex(key, c.hotShardCount)]
	coldShard := c.ColdShards[getShardIndex(key, c.coldShardCount)]

	coldShard.Set(key, entry)
	hotShard.Delete(key)
}
