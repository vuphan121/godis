# godis

<p align="center">
  <strong>Experimental Go Cache </strong><br/>
  Exploring cache strategies inspired by Redis and modern in-memory systems
</p>

<p align="center">
  <img src="https://img.shields.io/badge/language-Go-00ADD8?logo=go" />
  <img src="https://img.shields.io/badge/status-Experimental-yellow" />
</p>

---

<p align="center">
  <strong>Table of Contents</strong><br/>
  <a href="#about-the-project">About The Project</a> ·
  <a href="#high-level-design">High-level Design</a> ·
  <a href="#deep-dives--design-docs">Design Docs</a> ·
  <a href="#configuration">Configuration</a> ·
  <a href="#running-the-project">Running the Project</a>
</p>

---

## About The Project

`godis` is an experimental in-memory cache library written in Go, built for learning and exploration.  
- Focuses on understanding how modern cache systems manage frequently accessed data, balance performance trade-offs, and maintain efficiency under load.  
- Prioritizes clarity and experimentation over production readiness, serving as a hands-on way to study cache internals.

---

## High-level Design

At a high level, the cache is split into **two logical tiers**:

- **Hot cache** – stores frequently accessed keys for fast retrieval
- **Cold cache** – holds newly written or less frequently accessed keys


Core ideas:

- All keys start in the **cold cache**
- Access patterns determine whether a key is promoted to the **hot cache** or demoted back to cold over time

This separation allows the cache to prioritize memory and CPU resources for frequently used data while keeping overall maintenance lightweight.

---

## Deep Dives & Design Docs

Detailed design discussions live in Notion documents to keep this repository focused and readable.

- **Low-level cache design & data structures**  
  → _Notion doc link_

- **Hot / cold tiering strategy**  
  → _Notion doc link_

- **Count-Min Sketch & frequency estimation**  
  → _Notion doc link_

- **Sampling-based eviction & maintenance**  
  → _Notion doc link_

- **Comparison with other cache systems (Redis, LRU, LFU, etc.)**  
  → _Notion doc link_

---

## Configuration

The cache is configured using the Go **options pattern**, allowing behavior to be tuned incrementally without a large constructor.

Example usage:

    cache := cache.NewCache(
        config.WithDefaultTTL(5*time.Minute),
        config.WithShardCounts(4, 4),
        config.WithCMS(4, 50000),
        config.WithHotReadPercentage(0.5),
    )

If no options are provided, the cache falls back to a default configuration.

### Configuration overview

Configuration is grouped into several logical areas:

#### Cache layout
- HotShardCount / ColdShardCount  
  Number of shards per tier. Sharding reduces lock contention under concurrent access.

- DefaultTTL  
  Default time-to-live applied to entries if no TTL is specified at write time.

#### Hot / cold behavior
- HotReadPercentage  
  Controls how aggressively read access influences promotion into the hot cache.

- HotThresholdTTL  
  Minimum effective lifetime required for a key to be considered “hot”.

#### Frequency estimation
- CMSDepth / CMSWidth  
  Parameters for the Count-Min Sketch used to approximate access frequency without maintaining per-key counters.

#### Background maintenance
- ColdCleanupInterval / HotDemotionInterval  
  How often background goroutines sample entries for cleanup or demotion.

- Cleanup and demotion percentages and sample sizes  
  Bound the amount of work performed per maintenance cycle to keep overhead predictable.


## Running the Project

### Requirements

- Go 1.20 or newer (recommended)

### Using the library

Install the module:

```bash
go get github.com/vuphan121/godis