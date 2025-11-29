package cache

import (
	"math/rand"
	"time"
)

func StartColdCacheCleanup(c *Cache, interval time.Duration, threshold float64, maxRepeats int) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-c.ctx.Done():
				return
			case <-ticker.C:
				for _, shard := range c.coldShards {
					repeats := 0
					for repeats < maxRepeats {
						expiredFraction := cleanupShard(shard)
						if expiredFraction < threshold {
							break
						}
						repeats++
					}
				}
			}
		}
	}()
}

func cleanupShard(shard *CacheShard) float64 {
	shard.lock.RLock()
	if shard.keyCount == 0 {
		shard.lock.RUnlock()
		return 0
	}

	sampleSize := shard.keyCount / 20
	if sampleSize < 5 {
		sampleSize = 5
	}
	if sampleSize > 1000 {
		sampleSize = 1000
	}

	reservoir := make([]string, 0, sampleSize)
	i := 0
	for k := range shard.items {
		if i < sampleSize {
			reservoir = append(reservoir, k)
		} else {
			r := rand.Intn(i + 1)
			if r < sampleSize {
				reservoir[r] = k
			}
		}
		i++
	}
	shard.lock.RUnlock()

	expired := 0
	now := time.Now()

	for _, k := range reservoir {
		shard.lock.RLock()
		e, ok := shard.items[k]
		shard.lock.RUnlock()
		if !ok {
			continue
		}
		if !e.Expiration.IsZero() && now.After(e.Expiration) {
			shard.lock.Lock()
			delete(shard.items, k)
			shard.lock.Unlock()
			expired++
		}
	}

	return float64(expired) / float64(sampleSize)
}
