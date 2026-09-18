package config

import (
	"fmt"
	"time"
)

const (
	maxSampleLimit = 1_000_000
	maxCMSCells    = 10_000_000
)

// Config controls cache capacity, tiering, frequency estimation, and maintenance.
type Config struct {
	DefaultTTL        time.Duration
	HotShardCount     int
	ColdShardCount    int
	HotReadPercentage float64
	HotThresholdTTL   time.Duration
	HotMinHits        uint

	CMSDepth int
	CMSWidth int

	MaxEntries         int
	EvictionSampleSize int

	ColdCleanupInterval  time.Duration
	ColdCleanupPercent   float64
	ColdCleanupMinSample int
	ColdCleanupMaxSample int

	HotDemotionInterval  time.Duration
	HotDemotionPercent   float64
	HotDemotionMinSample int
	HotDemotionMaxSample int
}

// DefaultConfig returns conservative defaults suitable for local use.
func DefaultConfig() Config {
	return Config{
		DefaultTTL:           0,
		HotShardCount:        4,
		ColdShardCount:       4,
		HotReadPercentage:    0.20,
		HotThresholdTTL:      2 * time.Second,
		HotMinHits:           2,
		CMSDepth:             4,
		CMSWidth:             50_000,
		MaxEntries:           10_000,
		EvictionSampleSize:   32,
		ColdCleanupInterval:  100 * time.Millisecond,
		ColdCleanupPercent:   0.25,
		ColdCleanupMinSample: 5,
		ColdCleanupMaxSample: 1_000,
		HotDemotionInterval:  500 * time.Millisecond,
		HotDemotionPercent:   0.05,
		HotDemotionMinSample: 5,
		HotDemotionMaxSample: 1_000,
	}
}

// Validate rejects settings that would panic, allocate unreasonable samples,
// or produce undefined cache behavior.
func (c Config) Validate() error {
	if c.DefaultTTL < 0 {
		return fmt.Errorf("default TTL must be non-negative")
	}
	if c.HotShardCount <= 0 || c.ColdShardCount <= 0 {
		return fmt.Errorf("hot and cold shard counts must be positive")
	}
	if c.HotReadPercentage <= 0 || c.HotReadPercentage > 1 {
		return fmt.Errorf("hot read percentage must be in (0, 1]")
	}
	if c.HotThresholdTTL < 0 {
		return fmt.Errorf("hot threshold TTL must be non-negative")
	}
	if c.HotMinHits == 0 {
		return fmt.Errorf("hot minimum hits must be positive")
	}
	if c.CMSDepth <= 0 || c.CMSWidth <= 0 {
		return fmt.Errorf("Count-Min Sketch dimensions must be positive")
	}
	if c.CMSDepth > maxCMSCells/c.CMSWidth {
		return fmt.Errorf("Count-Min Sketch must not exceed %d counters", maxCMSCells)
	}
	if c.MaxEntries <= 0 {
		return fmt.Errorf("maximum entries must be positive")
	}
	if c.EvictionSampleSize <= 0 || c.EvictionSampleSize > maxSampleLimit {
		return fmt.Errorf("eviction sample size must be between 1 and %d", maxSampleLimit)
	}
	if err := validateMaintenance("cold cleanup", c.ColdCleanupInterval, c.ColdCleanupPercent, c.ColdCleanupMinSample, c.ColdCleanupMaxSample); err != nil {
		return err
	}
	if err := validateMaintenance("hot demotion", c.HotDemotionInterval, c.HotDemotionPercent, c.HotDemotionMinSample, c.HotDemotionMaxSample); err != nil {
		return err
	}
	return nil
}

func validateMaintenance(name string, interval time.Duration, percentage float64, minSample, maxSample int) error {
	if interval <= 0 {
		return fmt.Errorf("%s interval must be positive", name)
	}
	if percentage <= 0 || percentage > 1 {
		return fmt.Errorf("%s percentage must be in (0, 1]", name)
	}
	if minSample <= 0 || maxSample <= 0 || minSample > maxSample || maxSample > maxSampleLimit {
		return fmt.Errorf("%s samples must satisfy 0 < min <= max <= %d", name, maxSampleLimit)
	}
	return nil
}
