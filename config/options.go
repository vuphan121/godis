package config

import "time"

type Option func(*Config)

func WithDefaultTTL(ttl time.Duration) Option {
	return func(cfg *Config) {
		cfg.DefaultTTL = ttl
	}
}

func WithShardCounts(hot, cold int) Option {
	return func(cfg *Config) {
		cfg.HotShardCount = hot
		cfg.ColdShardCount = cold
	}
}

func WithHotReadPercentage(p float64) Option {
	return func(cfg *Config) {
		cfg.HotReadPercentage = p
	}
}

func WithHotThresholdTTL(ttl time.Duration) Option {
	return func(cfg *Config) {
		cfg.HotThresholdTTL = ttl
	}
}

func WithCMS(depth, width int) Option {
	return func(cfg *Config) {
		cfg.CMSDepth = depth
		cfg.CMSWidth = width
	}
}

func WithColdCleanup(interval time.Duration, percent float64, minSample int) Option {
	return func(cfg *Config) {
		cfg.ColdCleanupInterval = interval
		cfg.ColdCleanupPercent = percent
		cfg.ColdCleanupMinSample = minSample
	}
}

func WithHotDemotion(interval time.Duration, percent float64, minSample, maxSample int) Option {
	return func(cfg *Config) {
		cfg.HotDemotionInterval = interval
		cfg.HotDemotionPercent = percent
		cfg.HotDemotionMinSample = minSample
		cfg.HotDemotionMaxSample = maxSample
	}
}
