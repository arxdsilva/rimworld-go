---
name: rimworld-go-feature
description: Implement a scoped rimworld-go gameplay or simulation feature with domain ownership, determinism, tests and performance; also adding a package or command.
---

# Feature implementation

## Load

- Root `AGENTS.md`, the nearest package `AGENTS.md`, the public contracts and existing tests of the behaviour.
- The matching backlog rows in `.agents/backlog/`.
- `rimworld-go-ui-screens` when a screen, window, overlay or HUD is involved (flow: MAIN_UI §0); `rimworld-go-art-assets` when sprites, icons, chrome or presentation are.
- Definition of done: `.agents/specs/MASTER_SPEC.md` §70. Determinism: MASTER §6–7. Concurrency and anti-patterns: MASTER §55, §78. Layering and ports: `.agents/specs/HEXAGONAL_ARCHITECTURE.md` §6.

## Steps

1. Restate the behaviour as observable acceptance criteria.
2. Identify the owning domain package; search for existing canonical types before adding any.
3. Write or update the failing behavioural test first when practical.
4. Implement the smallest complete domain change. Game rules go in `internal/domain`, orchestration in `internal/application`, technology in `internal/adapters`; none in Ebitengine callbacks, CLI handlers, protobuf adapters or filesystem loaders.
5. Add application or adapter wiring only when the feature crosses a port.
6. If the behaviour is player-observable, add a read-only HUD/UI representation in the same change (root `AGENTS.md`), or say in the ledger why none exists yet.
7. Check determinism, save and mod impact.
8. Run targeted tests, then broader tests for the changed dependency surface; benchmark if a hot path changed.
9. Tick the matching checklist boxes (root `AGENTS.md` checkbox rule).
10. Append a section to the family ledger (`.agents/templates/HANDOFF.md`).

## New package or command

Create one only when a task needs it, not to match a diagram. Give it a `AGENTS.md` from `.agents/templates/PACKAGE_AGENTS.md` once it is real, construct concrete types only at the composition root, and add it to the architecture and SOLID budget checks (`make arch`).

## Answer before coding

```text
Owner / State read / State written / Simulation phase / Update frequency / Ordering
Index or query used / Allocation behaviour / Persistence / Mod surface / Debug visibility
Art or presentation / Screen or HUD
```

## Avoid

New global registries; UI-driven domain mutation; refactors unrelated to the acceptance criteria; duplicate IDs or types; global scans where an index exists or should; goroutine-per-entity; wall-clock gameplay; map-order decisions; a framework for one feature; private camera, light, palette or pivot conventions; private navigation, window or HUD rules where a named screen exists.

## Completion gate

MASTER §70, plus: failure paths explicit; save and mod compatibility addressed; benchmarks exist or stay valid for hot changes; art meets `rimworld-go-art-assets`; screens meet MAIN_UI §59.
