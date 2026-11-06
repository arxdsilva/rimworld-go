---
name: rimworld-go-review-integration
description: Review and integrate rimworld-go changes from one or many agents: architecture, determinism, performance, tests, contract drift, screen and art compliance.
---

# Review and integration

## Load

Read only the acceptance criteria, the diff, the affected package `AGENTS.md`, the reported tests and benchmarks, and (when relevant) the MAIN_UI sections of the changed screen or the ART sections of the changed assets. Definition of done: `.agents/specs/MASTER_SPEC.md` §70. Open more source only when needed.

## Review order

1. correctness
2. architecture ownership
3. determinism
4. invariants
5. data complexity and performance
6. concurrency
7. persistence and mod compatibility
8. tests
9. Go API and readability
10. ledger section (`.agents/templates/HANDOFF.md`)
11. matching checklist boxes ticked (root `AGENTS.md` checkbox rule)
12. player-observable behaviour has a read-only HUD/UI representation, or the change says why none applies yet
13. screens follow MAIN_UI §0, §58, §59 plus the screen section
14. art follows ART (scale, pivot, palette, clean room, in-game review) when sprites, icons, chrome or asset tooling changed

## Reject

An unrelated broad refactor; a weakened or deleted test; a duplicate canonical type; domain -> adapter dependency; global mutable state; goroutine-per-entity; wall-clock gameplay; map-order decisions; a silent save format break; a new hot global scan; a performance claim without a benchmark; unbounded external input; hand-edited generated code; proprietary or reconstructed RimWorld art; bulk art that skips Wave 0 contracts or the ART §34 prototype gate; visual RNG feeding the simulation; a later-wave screen before Wave UI-0; a screen that mutates domain state or scans the world every frame.

## Multi-agent integration

Before merging a wave: collect ledger sections; compare interface changes; find overlapping files; resolve each canonical contract once; merge in dependency order; run the full tests, the determinism suite, the architecture lint and the affected benchmarks (gates: `.agents/specs/PARALLEL_WORK.md` §5). Never merge two competing abstractions to "clean up later".

## Ledger section check

Each task states: changed contracts, invariants, tests, benchmark delta, determinism impact, save and mod impact, art impact, screen impact, limits and follow-ups. Do not request chain-of-thought.
