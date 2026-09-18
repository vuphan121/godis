# godis

[![CI](https://github.com/vuphan121/godis/actions/workflows/ci.yml/badge.svg)](https://github.com/vuphan121/godis/actions/workflows/ci.yml)

`godis` is an experimental, bounded in-memory cache library for Go. It combines sharded hot/cold tiers, approximate frequency tracking, TTL expiration, sampled eviction, and background maintenance behind concurrent untyped and generic APIs.

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

For compile-time value types, use the generic facade:

```go
c, err := cache.NewTypedCache[string](config.WithCapacity(10_000, 32))
if err != nil {
	log.Fatal(err)
}
defer c.Close()

if err := c.Set("user:42", "Ada"); err != nil {
	log.Fatal(err)
}
user, ok := c.Get("user:42")
```

## API

```go
c, err := cache.NewCache(options...)
err = c.Set(key, value)
err = c.Set(key, value, 30*time.Second)
err = c.Set(key, value, 0)
value, ok := c.Get(key)
deleted := c.Delete(key)
size := c.Size()
stats := c.Stats()
c.Close()
```

A negative TTL or more than one TTL argument returns `cache.ErrInvalidTTL`. Writes after shutdown return `cache.ErrClosed`.

## How it works

All new keys enter a cold tier. Successful reads update a concurrency-safe Count-Min Sketch. A cold key becomes eligible for promotion after reaching the current frequency threshold and, when it has an expiration, retaining enough useful lifetime.

The threshold is calculated from the estimated counts of resident keys instead of random sketch cells. A minimum-hit setting prevents the zero-threshold behavior that would otherwise promote every key. Sketch counters use lazy generational decay, so aging is constant-time and old traffic does not dominate forever.

Hot and cold maps are sharded. Tier transitions use a cache-level coordination lock so a completed operation cannot leave a key in both tiers or overwrite a newer concurrent mutation. Background workers sample cold entries for expiration and hot entries for demotion.

## Capacity and eviction

The cache is bounded by `MaxEntries`, which defaults to 10,000. Inserting a new key at capacity samples candidates and evicts an expired entry first. Otherwise, it prefers a low-frequency cold entry, falling back to the hot tier only when the cold tier is empty.

This is an approximate sampled LFU policy, not a strict global LFU or LRU implementation. Increasing the eviction sample size improves candidate quality at the cost of more work during insertion.

## Configuration

Configuration uses functional options. Invalid combinations are rejected before memory is allocated or workers are started.

| Option | Purpose | Default |
| --- | --- | --- |
| `WithDefaultTTL` | TTL used when `Set` omits a TTL | No expiration |
| `WithShardCounts` | Hot and cold shard counts | 16, 16 |
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

Public operations are safe for concurrent use. Existing-key writes use shard-level coordination, while insertions, eviction, deletion, and tier transfers use cache-level coordination to preserve size and ownership invariants. Reads remain concurrent. Frequency updates use row-local lock striping instead of one sketch-wide write lock.

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

Detailed engineering decisions, test evidence, benchmark history, and the next-work queue live in the [godis agent guide](https://github.com/vuphan121/agent-files/blob/main/godis/AGENTS.md).

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
- [Release history](https://github.com/vuphan121/godis/releases)
