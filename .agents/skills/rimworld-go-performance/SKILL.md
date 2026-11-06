---
name: rimworld-go-performance
description: Optimize rimworld-go simulation scale, CPU, memory, allocations, data layout, indexes, update frequency, and concurrency based on measurement.
---

# Performance & Scale

## Optimization order

Always prefer:

```text
1. eliminate bad complexity/global scans
2. reduce update frequency
3. add indexes
4. use dirty invalidation
5. sleep inactive state
6. use analytical advancement
7. batch
8. reduce allocation
9. improve data layout/cache locality
10. parallelize pure work
11. micro-optimize
```

Do not jump to concurrency or `unsafe` first.

## Scale target mindset

Design hot systems for:

- 1,000+ active settlers;
- large tile maps;
- 100k+ plants/items;
- long-running worlds.

Every hot subsystem should answer:

```text
N:
frequency:
complexity:
index:
dirty trigger:
sleep behavior:
allocs:
parallelizable pure work:
```

## Data rules

Prefer:

- dense slices;
- stable compact IDs;
- bitsets/flags;
- sparse sets where appropriate;
- caller/scratch buffers when proven useful;
- region/chunk-local queries.

Avoid:

- pointer-heavy object graphs;
- repeated whole-world scans;
- temporary maps per tick;
- string IDs in hot loops;
- per-entity goroutines;
- allocating iterator abstractions without evidence.

## Measurement

Before/after performance work capture:

```text
ns/op or ms/op
B/op
allocs/op
scenario size
tick frequency
```

Use Go benchmarks, `pprof`, trace, and long-run heap checks.

## Concurrency

Parallelize only pure or snapshot-based work.

Authoritative mutation remains single-owner and ordered.

## Performance regression rule

A feature that meaningfully worsens a core benchmark must either:

- be fixed;
- have a documented tradeoff approved by architecture/performance ownership.

## `unsafe`

Forbidden by default.

Requires ADR + benchmark evidence + tests + portability justification.
