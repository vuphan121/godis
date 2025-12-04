package cache

import (
	"math/rand"
	"time"
)

func StartHotDemotion(c *Cache) {
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-c.ctx.Done():
				return
			case <-ticker.C:
				totalKeys := 1
				for _, shard := range c.HotShards {
					shard.lock.RLock()
					totalKeys += shard.keyCount
					shard.lock.RUnlock()
				}
				if totalKeys == 0 {
					totalKeys = 1
				}

				c.hotThreshold = c.cms.ApproxTopThreshold(c.hotReadPercentage, totalKeys)
				c.hotThresholdUpdated = time.Now()
				threshold := c.hotThreshold

				for _, shard := range c.HotShards {
					sampleSize := shard.keyCount / 20
					if sampleSize < 5 {
						sampleSize = 5
					}
					if sampleSize > 1000 {
						sampleSize = 1000
					}

					//TODO: run goroutine for demotion
					keys := shard.SampleKeysUnique(sampleSize)
					shard.lock.Lock()
					for _, key := range keys {
						entry, ok := shard.items[key]
						if !ok {
							continue
						}
						count := c.cms.Count(key)
						if count < threshold || (!entry.Expiration.IsZero() && time.Now().After(entry.Expiration)) {
							c.demoteHotKey(key, entry)
						}
					}
					shard.lock.Unlock()
				}
			}
		}
	}()
}

//TODO: reuse reservoir slice

func (s *CacheShard) SampleKeysUnique(n int) []string {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if n >= s.keyCount {
		keys := make([]string, 0, s.keyCount)
		for k := range s.items {
			keys = append(keys, k)
		}
		return keys
	}

	// Reservoir sampling
	reservoir := make([]string, 0, n)
	i := 0
	for k := range s.items {
		if i < n {
			reservoir = append(reservoir, k)
		} else {
			r := rand.Intn(i + 1)
			if r < n {
				reservoir[r] = k
			}
		}
		i++
	}

	return reservoir
}
