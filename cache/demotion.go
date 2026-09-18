package cache

import (
	"time"

	"github.com/vuphan121/godis/config"
)

func (c *Cache) startHotDemotion(cfg config.Config) {
	c.workers.Add(1)
	go func() {
		defer c.workers.Done()
		ticker := time.NewTicker(cfg.HotDemotionInterval)
		defer ticker.Stop()
		for {
			select {
			case <-c.ctx.Done():
				return
			case <-ticker.C:
				c.maintainHot(cfg.HotDemotionPercent, cfg.HotDemotionMinSample, cfg.HotDemotionMaxSample)
			}
		}
	}()
}

func (c *Cache) maintainHot(percentage float64, minSample, maxSample int) {
	now := time.Now()
	c.tierMu.Lock()
	defer c.tierMu.Unlock()

	c.cms.Decay()
	threshold := c.cms.TopThreshold(c.residentKeysLocked(), c.hotReadPercentage, c.hotMinHits)
	c.hotThreshold.Store(uint64(threshold))

	for _, shard := range c.hotShards {
		limit := maintenanceSampleSize(shard.len(), percentage, minSample, maxSample)
		for _, candidate := range shard.sample(limit) {
			if candidate.entry.expired(now) {
				if shard.deleteIf(candidate.key, candidate.entry) {
					c.size--
					c.metrics.expirations.Add(1)
				}
				continue
			}
			if c.cms.Count(candidate.key) >= threshold && candidate.entry.hotEligible(now, c.minHotTTL) {
				continue
			}
			if !shard.deleteIf(candidate.key, candidate.entry) {
				continue
			}
			c.coldShard(candidate.key).set(candidate.key, candidate.entry)
			c.metrics.demotions.Add(1)
		}
	}
}

func (c *Cache) residentKeysLocked() []string {
	keys := make([]string, 0, c.size)
	for _, shard := range c.hotShards {
		keys = append(keys, shard.keys()...)
	}
	for _, shard := range c.coldShards {
		keys = append(keys, shard.keys()...)
	}
	return keys
}
