---
name: rimworld-go-review-integration
description: Review and integrate rimworld-go changes from multiple agents, checking architecture, determinism, Go quality, performance, tests, conflicts, public contract drift, and art-direction compliance when visual assets changed.
---

# Review & Integration

## Reviewer context

Read only:

- task acceptance criteria;
- diff;
- affected package `AGENTS.md`;
- relevant contract/ADR;
- tests/benchmarks reported;
- named `ART_DIRECTION_AND_ASSET_PIPELINE.md` sections when visual files changed.

Open more source only when needed.

## Review order

1. correctness;
2. architecture ownership;
3. determinism;
4. invariants;
5. data complexity/performance;
6. concurrency;
7. persistence/mod compatibility;
8. tests;
9. Go API/readability;
10. documentation/handoff.
11. matching checklist checkbox updated in `.agent/RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md`, `.agent/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md`, or `.agent/ART_DIRECTION_AND_ASSET_PIPELINE.md`, if the change satisfies (fully or partially) a listed item.
12. player-observable behavior has a corresponding read-only HUD/UI representation, or the PR states why none applies yet.
13. player-visible art follows `.agent/ART_DIRECTION_AND_ASSET_PIPELINE.md` (scale/pivot/palette, clean-room, in-game review) when sprites, icons, UI chrome, or asset-pipeline files changed.

## Reject patterns

- unrelated broad refactor;
- weakened/deleted test to make CI pass;
- duplicate canonical type;
- domain -> adapter dependency;
- global mutable state;
- goroutine-per-entity;
- wall-clock gameplay;
- map-order decisions;
- silent save format break;
- new hot global scan;
- performance claim without benchmark;
- unbounded external input;
- generated code hand edits;
- proprietary or reconstructed RimWorld art;
- bulk art that skips Wave 0 contracts or the §34 prototype gate;
- visual RNG feeding authoritative simulation.

## Multi-agent integration

Before merging a wave:

```text
collect handoffs
compare interface changes
identify overlapping files
resolve canonical contract once
merge in dependency order
run full tests
run determinism suite
run architecture lint
run affected benchmarks
```

Do not merge two competing abstractions and “clean up later.”

## Handoff validation

Each completed task should state:

- changed files;
- public contracts;
- invariants;
- tests;
- benchmark delta;
- determinism impact;
- save/mod impact;
- art/visual impact when presentation assets changed;
- limitations/follow-ups.

Do not request chain-of-thought.
