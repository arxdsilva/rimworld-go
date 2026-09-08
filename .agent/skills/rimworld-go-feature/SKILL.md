---
name: rimworld-go-feature
description: Implement a scoped rimworld-go gameplay or simulation feature while preserving domain ownership, determinism, testability, and performance.
---

# Feature Implementation

## Required context

Read:

- root `AGENTS.md`;
- nearest package `AGENTS.md`;
- current public contracts;
- existing tests for the behavior;
- relevant system doc.

Load broader architecture or master spec only when needed.

## Workflow

1. Restate the behavior as observable acceptance criteria.
2. Identify the owning domain package.
3. Search for existing canonical types before adding any.
4. Write or update the failing behavioral test first when practical.
5. Implement the smallest complete domain change.
6. Add application/adapter wiring only when the feature crosses a port.
7. Verify determinism impact.
8. Verify save/mod impact.
9. Run targeted tests.
10. Run broader tests appropriate to the changed dependency surface.
11. Benchmark if any hot path changed.
12. Produce a concise handoff.

## Domain placement

Game rules belong in `internal/domain`.

Use cases/session orchestration belong in `internal/application`.

Technology belongs in `internal/adapters`.

Do not put game rules in Ebitengine callbacks, CLI handlers, protobuf adapters, or filesystem loaders.

## Feature invariants

Every feature must answer:

```text
Owner:
State read:
State written:
Simulation phase:
Update frequency:
Ordering:
Index/query used:
Allocation behavior:
Persistence:
Mod/content surface:
Debug visibility:
```

## Avoid

- new global registries;
- UI-driven domain mutation;
- broad refactors unrelated to acceptance criteria;
- one-off duplicate IDs/types;
- global scans when an index exists or should exist;
- goroutine-per-entity behavior;
- wall-clock-dependent gameplay;
- order-sensitive map iteration;
- generic frameworks introduced for one feature.

## Completion gate

A feature is complete only when:

- behavior is tested;
- architecture boundaries are preserved;
- deterministic execution remains defined;
- failure paths are explicit;
- relevant docs/interfaces are updated;
- save/mod compatibility is addressed;
- benchmarks exist or remain valid for hot changes;
- handoff is written.
