# godis

[![CI](https://github.com/vuphan121/godis/actions/workflows/ci.yml/badge.svg)](https://github.com/vuphan121/godis/actions/workflows/ci.yml)

`godis` is an experimental, bounded in-memory cache library for Go. It explores sharding, hot/cold tiering, approximate frequency tracking, TTL expiration, sampled eviction, and background maintenance.

The project is suitable for learning and experimentation. It has automated correctness and race tests, but it has not yet been proven under production workloads.

## Requirements

- Go 1.20 or newer

## Installation

```bash
go get github.com/vuphan121/godis
```

## Quick start

```go
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/vuphan121/godis/cache"
	"github.com/vuphan121/godis/config"
)

func main() {
	c, err := cache.NewCache(
		config.WithCapacity(10_000, 32),
		config.WithDefaultTTL(5*time.Minute),
		config.WithShardCounts(8, 8),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	if err := c.Set("user:42", "Ada"); err != nil {
		log.Fatal(err)
	}

	if value, ok := c.Get("user:42"); ok {
		fmt.Println(value)
	}
}
```

`NewCache` validates the final configuration and returns an error for unsafe values. Every created cache starts maintenance goroutines, so callers must call `Close`. Closing is idempotent.

## API

```go
c, err := cache.NewCache(options...)
err = c.Set(key, value)                  // configured default TTL
err = c.Set(key, value, 30*time.Second) // explicit TTL
err = c.Set(key, value, 0)              // no expiration
value, ok := c.Get(key)
deleted := c.Delete(key)
size := c.Size()
stats := c.Stats()
c.Close()
```

A negative TTL or more than one TTL argument returns `cache.ErrInvalidTTL`. Writes after shutdown return `cache.ErrClosed`.

## How it works

All new keys enter a cold tier. Successful reads update a concurrency-safe Count-Min Sketch. A cold key becomes eligible for promotion after reaching the current frequency threshold and, when it has an expiration, retaining enough useful lifetime.

The threshold is calculated from the estimated counts of resident keys instead of random sketch cells. A minimum-hit setting prevents the zero-threshold behavior that would otherwise promote every key. Sketch counters periodically decay so old traffic does not dominate forever.

Hot and cold maps are sharded. Tier transitions use a cache-level coordination lock so a completed operation cannot leave a key in both tiers or overwrite a newer concurrent mutation. Background workers sample cold entries for expiration and hot entries for demotion.

## Capacity and eviction

The cache is bounded by `MaxEntries`, which defaults to 10,000. Inserting a new key at capacity samples candidates and evicts an expired entry first. Otherwise, it prefers a low-frequency cold entry, falling back to the hot tier only when the cold tier is empty.

This is an approximate sampled LFU policy, not a strict global LFU or LRU implementation. Increasing the eviction sample size improves candidate quality at the cost of more work during insertion.

## Configuration

Configuration uses functional options. Invalid combinations are rejected before memory is allocated or workers are started.

| Option | Purpose | Default |
| --- | --- | --- |
| `WithDefaultTTL` | TTL used when `Set` omits a TTL | No expiration |
| `WithShardCounts` | Hot and cold shard counts | 4, 4 |
| `WithCapacity` | Maximum entries and eviction sample size | 10,000, 32 |
| `WithCMS` | Count-Min Sketch depth and width | 4, 50,000 |
| `WithHotReadPercentage` | Target fraction used to calculate the hot threshold | 20% |
| `WithHotMinHits` | Minimum accesses required for promotion | 2 |
| `WithHotThresholdTTL` | Minimum remaining TTL required for promotion | 2 seconds |
| `WithColdCleanup` | Cold-tier cleanup interval, fraction, and minimum sample | 100 ms, 25%, 5 |
| `WithColdCleanupMaxSample` | Maximum cold cleanup sample per shard | 1,000 |
| `WithHotDemotion` | Hot-tier interval, fraction, and sample bounds | 500 ms, 5%, 5–1,000 |

Percentages must be in `(0, 1]`; shard counts, sketch dimensions, capacities, intervals, and sample sizes must be positive. Minimum samples cannot exceed maximum samples.

## Observability

`Stats` returns a point-in-time snapshot:

```go
type Stats struct {
	Size        int
	Hits        uint64
	Misses      uint64
	Promotions  uint64
	Demotions   uint64
	Evictions   uint64
	Expirations uint64
}
```

Counters are process-local and reset when a new cache is created. Keys are never exposed as metric labels.

## Concurrency and lifecycle

Public operations are safe for concurrent use. Cache-level coordination currently serializes mutations and tier transfers in favor of simple, testable correctness. Reads remain concurrent. Future performance work should preserve the tier-transition invariants and be justified with race tests and before-and-after benchmarks.

The Count-Min Sketch is approximate. Deleting a key does not selectively clear its counters because doing so would corrupt counts shared through hash collisions; old counts disappear through periodic decay.

## Development

Run the full verification suite:

```bash
gofmt -w .
go vet ./...
go test ./...
go test -race ./...
```

Run the sequential operation fuzzer:

```bash
go test -run '^$' -fuzz '^FuzzSequentialModel$' -fuzztime=30s ./cache
```

Run benchmarks:

```bash
go test -run '^$' -bench . -benchmem ./cache
```

CI runs formatting, vet, unit, and race checks on Linux, plus the unit suite on Windows. A repository policy test rejects comments in Go source; durable explanations belong in the README and the external agent guide.

## Compatibility note

The hardened constructor and mutation API differ from the earliest experimental version:

- `NewCache` now returns `(*Cache, error)`.
- `Set` now returns an error.
- `Delete` now reports whether a key existed.
- Callers must call `Close`.

The project has not published a stable v1 API, so further compatibility changes remain possible while the design is validated.

## Design documents

- [Low-level design](https://www.notion.so/Godis-low-level-design-30005c0ca4468048ac33fbcc5417f004?source=copy_link)
- [Analysis of common caching systems](https://www.notion.so/Simple-analysis-of-common-caching-systems-30405c0ca4468042b295f064e33f6a46?source=copy_link)
