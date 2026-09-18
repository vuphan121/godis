package config

import "time"

type Option func(*Config)

func WithDefaultTTL(ttl time.Duration) Option {
	return func(cfg *Config) { cfg.DefaultTTL = ttl }
}

func WithShardCounts(hot, cold int) Option {
	return func(cfg *Config) {
		cfg.HotShardCount = hot
		cfg.ColdShardCount = cold
	}
}

func WithHotReadPercentage(percentage float64) Option {
	return func(cfg *Config) { cfg.HotReadPercentage = percentage }
}

func WithHotThresholdTTL(ttl time.Duration) Option {
	return func(cfg *Config) { cfg.HotThresholdTTL = ttl }
}

func WithHotMinHits(hits uint) Option {
	return func(cfg *Config) { cfg.HotMinHits = hits }
}

func WithCMS(depth, width int) Option {
	return func(cfg *Config) {
		cfg.CMSDepth = depth
		cfg.CMSWidth = width
	}
}

func WithCapacity(maxEntries, evictionSampleSize int) Option {
	return func(cfg *Config) {
		cfg.MaxEntries = maxEntries
		cfg.EvictionSampleSize = evictionSampleSize
	}
}

func WithColdCleanup(interval time.Duration, percentage float64, minSample int) Option {
	return func(cfg *Config) {
		cfg.ColdCleanupInterval = interval
		cfg.ColdCleanupPercent = percentage
		cfg.ColdCleanupMinSample = minSample
	}
}

func WithColdCleanupMaxSample(maxSample int) Option {
	return func(cfg *Config) { cfg.ColdCleanupMaxSample = maxSample }
}

func WithHotDemotion(interval time.Duration, percentage float64, minSample, maxSample int) Option {
	return func(cfg *Config) {
		cfg.HotDemotionInterval = interval
		cfg.HotDemotionPercent = percentage
		cfg.HotDemotionMinSample = minSample
		cfg.HotDemotionMaxSample = maxSample
	}
}
