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
- relevant system doc;
- the matching item(s) in `.agent/RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md` or `.agent/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md` when the task maps to a checklist entry;
- `.agent/ART_DIRECTION_AND_ASSET_PIPELINE.md` via skill `rimworld-go-art-assets` when the feature includes sprites, icons, UI chrome, renderer presentation, or asset-pipeline tooling.

Load broader architecture or master spec only when needed.

## Workflow

1. Restate the behavior as observable acceptance criteria.
2. Identify the owning domain package.
3. Search for existing canonical types before adding any.
4. Write or update the failing behavioral test first when practical.
5. Implement the smallest complete domain change.
6. Add application/adapter wiring only when the feature crosses a port.
7. If the behavior is player-observable, add or extend a HUD/UI representation (read-only, via `Snapshot()`/read models) in the same change; if none makes sense yet, note why in the handoff.
8. If the change includes player-visible art, follow `rimworld-go-art-assets` (original assets, canonical scale/pivots/palette, in-game review).
9. Verify determinism impact.
10. Verify save/mod impact.
11. Run targeted tests.
12. Run broader tests appropriate to the changed dependency surface.
13. Benchmark if any hot path changed.
14. Update the matching checklist item's checkbox (`[x]`/`[-]`/`[!]`) in `.agent/RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md`, `.agent/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md`, and/or `.agent/ART_DIRECTION_AND_ASSET_PIPELINE.md` when the task maps to one.
15. Produce a concise handoff.

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
Art / presentation:
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
- generic frameworks introduced for one feature;
- independent camera, light, palette, or pivot conventions when art/visual work is in scope.

## Completion gate

A feature is complete only when:

- behavior is tested;
- architecture boundaries are preserved;
- deterministic execution remains defined;
- failure paths are explicit;
- relevant docs/interfaces are updated;
- save/mod compatibility is addressed;
- benchmarks exist or remain valid for hot changes;
- player-visible art, if any, meets `rimworld-go-art-assets` acceptance;
- handoff is written.
