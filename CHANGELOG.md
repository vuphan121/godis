# Changelog

This file records user-visible changes for each tagged release.

## Unreleased

### Added

- Reserved for changes made after `v0.1.1`.

## 0.1.1 - 2026-09-18

### Added

- Deterministic concurrent-history coverage for TTL expiration and capacity-one eviction.
- Concurrent maintenance stress coverage for promotion, demotion, expiration, mutation, and bounded-size invariants.
- A parallel frequency-update benchmark for regression tracking.

### Changed

- Count-Min Sketch updates use 256 lock stripes per row instead of one sketch-wide write lock.
- Internal time reads are injectable so expiration histories can be verified without wall-clock timing.
- The README now presents the generic facade, current defaults, architecture, verification workflow, and agent-guide handoff accurately.

## 0.1.0 - 2026-09-18

### Added

- Bounded sharded hot/cold cache with TTL expiration.
- Sampled expired-first, cold-first approximate LFU eviction.
- Concurrency-safe Count-Min Sketch with lazy generational aging.
- Untyped `Cache` API and optional string-keyed `TypedCache[V]` facade.
- Idempotent worker shutdown, cache statistics, configuration validation, and lifecycle errors.
- Unit, race, model, fuzz, benchmark, invariant, and comment-policy tests.
- Linux and Windows continuous integration with immutable action revisions.

### Changed

- `NewCache` returns `(*Cache, error)`.
- `Set` returns an error.
- `Delete` reports whether a key existed.
- Count-Min Sketch decay advances a generation in constant time instead of scanning every counter.
- Existing-key writes coordinate at the shard level and the default tier width increases from 4 to 16 shards.
- Capacity eviction uses one cross-shard reservoir, avoiding allocation growth as shard count increases.

### Fixed

- Nested-lock deadlock during hot-key demotion.
- Data races in frequency and threshold state.
- Incorrect shard size bookkeeping during expiration.
- Non-atomic promotion and demotion behavior.
- Zero-threshold promotion of every cold key.

## Release policy

- Use semantic versions while the module is pre-v1.
- Keep breaking API changes within minor releases and document them here before tagging.
- Keep fixes and compatible additions within patch releases.
- Move entries from `Unreleased` into a dated version section in the release commit.
- Run formatting, vet, unit, race, model, fuzz-seed, and benchmark checks before creating an annotated tag.
- Tag only a clean commit that has passed Linux and Windows CI.
