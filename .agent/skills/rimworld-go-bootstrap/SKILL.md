---
name: rimworld-go-bootstrap
description: Bootstrap or extend the rimworld-go repository foundations: module, directories, canonical primitives, CI, architecture lint, ADRs, headless simulation shell, and agent documentation.
---

# Repository Bootstrap

Use for foundation/setup work, especially before parallel feature agents begin.

## Recommended order

1. `go.mod`;
2. minimal directory structure required by current tasks;
3. root `AGENTS.md`;
4. canonical `Tick`;
5. canonical `EntityID`;
6. deterministic RNG primitive/API;
7. `Tile` and `ChunkCoord`;
8. typed command envelope and ordering;
9. headless `Simulation.Step`;
10. state hashing skeleton;
11. testkit/simtest;
12. architecture lint;
13. CI;
14. initial ADRs;
15. package `AGENTS.md` as packages become real;
16. `assets/` layout, palettes, and metadata schema only when renderer/art work begins — follow `.agent/ART_DIRECTION_AND_ASSET_PIPELINE.md` Wave 0 via `rimworld-go-art-assets`.

Do not generate dozens of empty packages just to match a diagram.

## Initial CI gates

At minimum:

```bash
gofmt check
go vet ./...
staticcheck ./...
go test ./...
go test -race ./...   # targeted/CI tier as runtime allows
architecture lint
content lint when content exists
determinism smoke test
```

## Initial ADR set

Consider:

```text
0001 hexagonal architecture
0002 single-owner deterministic simulation
0003 discrete ticks
0004 deterministic RNG
0005 entity/component storage
0006 chunked world
0007 hierarchical pathfinding
0008 command-based authoritative mutations
0009 snapshot rendering
0010 persistence schema
0011 content/runtime indexing
0012 snapshot-compute-ordered-commit workers
```

## Parallel-agent gate

Do not fan out agents until:

- canonical primitives compile;
- ownership is documented;
- contracts needed by parallel tasks are stable enough;
- CI can reject obvious architecture/determinism regressions.
