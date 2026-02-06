package config

import "time"

type Config struct {
	DefaultTTL        time.Duration
	HotShardCount     int
	ColdShardCount    int
	HotReadPercentage float64
	HotThresholdTTL   time.Duration

	CMSDepth int
	CMSWidth int

	ColdCleanupInterval  time.Duration
	ColdCleanupPercent   float64
	ColdCleanupMinSample int

	HotDemotionInterval  time.Duration
	HotDemotionPercent   float64
	HotDemotionMinSample int
	HotDemotionMaxSample int
}

func DefaultConfig() Config {
	return Config{
		DefaultTTL:        0,
		HotShardCount:     4,
		ColdShardCount:    4,
		HotReadPercentage: 0.5,
		HotThresholdTTL:   2000 * time.Millisecond,

		CMSDepth: 4,
		CMSWidth: 50000,

		ColdCleanupInterval:  100 * time.Millisecond,
		ColdCleanupPercent:   0.25,
		ColdCleanupMinSample: 5,

		HotDemotionInterval:  500 * time.Millisecond,
		HotDemotionPercent:   0.05,
		HotDemotionMinSample: 5,
		HotDemotionMaxSample: 1000,
	}
}
