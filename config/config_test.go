package config

import (
	"testing"
	"time"
)

func TestDefaultConfigIsValid(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config is invalid: %v", err)
	}
	if cfg.HotShardCount != 16 || cfg.ColdShardCount != 16 {
		t.Fatalf("default shard counts = (%d, %d), want (16, 16)", cfg.HotShardCount, cfg.ColdShardCount)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Config)
	}{
		{"negative default TTL", func(c *Config) { c.DefaultTTL = -time.Second }},
		{"zero hot shards", func(c *Config) { c.HotShardCount = 0 }},
		{"zero cold shards", func(c *Config) { c.ColdShardCount = 0 }},
		{"invalid hot fraction", func(c *Config) { c.HotReadPercentage = 1.1 }},
		{"zero hot hits", func(c *Config) { c.HotMinHits = 0 }},
		{"zero sketch depth", func(c *Config) { c.CMSDepth = 0 }},
		{"oversized sketch", func(c *Config) { c.CMSDepth, c.CMSWidth = 100_000, 100_000 }},
		{"zero capacity", func(c *Config) { c.MaxEntries = 0 }},
		{"zero eviction sample", func(c *Config) { c.EvictionSampleSize = 0 }},
		{"zero cleanup interval", func(c *Config) { c.ColdCleanupInterval = 0 }},
		{"invalid cleanup percentage", func(c *Config) { c.ColdCleanupPercent = 0 }},
		{"inverted cleanup sample", func(c *Config) { c.ColdCleanupMinSample, c.ColdCleanupMaxSample = 10, 5 }},
		{"zero demotion interval", func(c *Config) { c.HotDemotionInterval = 0 }},
		{"invalid demotion percentage", func(c *Config) { c.HotDemotionPercent = -1 }},
		{"inverted demotion sample", func(c *Config) { c.HotDemotionMinSample, c.HotDemotionMaxSample = 10, 5 }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := DefaultConfig()
			test.change(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
