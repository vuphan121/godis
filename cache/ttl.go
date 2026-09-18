package cache

import (
	"math"
	"time"

	"github.com/vuphan121/godis/config"
)

func (c *Cache) startColdCleanup(cfg config.Config) {
	c.workers.Add(1)
	go func() {
		defer c.workers.Done()
		ticker := time.NewTicker(cfg.ColdCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-c.ctx.Done():
				return
			case <-ticker.C:
				c.cleanupCold(cfg.ColdCleanupPercent, cfg.ColdCleanupMinSample, cfg.ColdCleanupMaxSample)
			}
		}
	}()
}

func (c *Cache) cleanupCold(percentage float64, minSample, maxSample int) {
	now := time.Now()
	c.tierMu.Lock()
	defer c.tierMu.Unlock()
	for _, shard := range c.coldShards {
		limit := maintenanceSampleSize(shard.len(), percentage, minSample, maxSample)
		for _, candidate := range shard.sample(limit) {
			if candidate.entry.expired(now) && shard.deleteIf(candidate.key, candidate.entry) {
				c.size--
				c.metrics.expirations.Add(1)
			}
		}
	}
}

func maintenanceSampleSize(total int, percentage float64, minimum, maximum int) int {
	if total <= 0 {
		return 0
	}
	size := int(math.Ceil(float64(total) * percentage))
	if size < minimum {
		size = minimum
	}
	if size > maximum {
		size = maximum
	}
	if size > total {
		size = total
	}
	return size
}
