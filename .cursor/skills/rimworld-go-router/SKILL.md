---
name: rimworld-go-router
description: Route any rimworld-go engineering task to the smallest relevant set of project skills and canonical documents. Use first for ambiguous, cross-cutting, or newly assigned tasks.
---

# rimworld-go Task Router

Use this skill before implementation when the correct subsystem or context set is not already obvious.

## Goal

Select the smallest sufficient context and skill set for the task.

Never load every project document or every skill by default.

## Canonical project documents

Read in this order only as needed:

1. `/AGENTS.md`
2. nearest package `AGENTS.md`
3. relevant source interfaces/types
4. relevant tests
5. relevant `docs/systems/*.md`
6. relevant `docs/adr/*.md`
7. `.cursor/HEXAGONAL_ARCHITECTURE.md`
8. `.cursor/GO_BEST_PRACTICES.md`
9. `.cursor/MASTER_SPEC.md` only when broader product intent is required
10. `.cursor/RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md` when scoping/ordering a gameplay feature
11. `.cursor/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md` when scoping/ordering a UI feature
12. `.cursor/MAIN_UI_SCREENS_SPECIFICATION.md` when implementing or changing a player-facing screen, window, overlay, or HUD layout (via skill `rimworld-go-ui-screens`; load named sections only)
13. `.cursor/ART_DIRECTION_AND_ASSET_PIPELINE.md` when the task produces or changes player-visible art, renderer presentation, UI chrome, or asset-pipeline tooling (via skill `rimworld-go-art-assets`; load named sections only)

Token/context policy is defined by `.cursor/AI_TOKEN_OPTIMIZATION.md`.

## Routing table

| Task | Add skill |
|---|---|
| New feature or domain behavior | `rimworld-go-feature` |
| Package/dependency/interface design | `rimworld-go-hexagonal` |
| Tick order, RNG, authoritative state, replay determinism | `rimworld-go-determinism` |
| Hot loops, scale, allocations, data layout | `rimworld-go-performance` |
| Paths, regions, movement, congestion | `rimworld-go-pathing` |
| Intents, work discovery, jobs, reservations | `rimworld-go-ai-jobs` |
| Save/load, schema, snapshots, replay | `rimworld-go-persistence-replay` |
| Definitions, YAML/JSON content, mods | `rimworld-go-content-modding` |
| Sprites, icons, UI chrome, palettes, pivots, atlases, renderer presentation, asset pipeline | `rimworld-go-art-assets` |
| Player-facing screens, windows, overlays, HUD layout, navigation, inspectors, management tables | `rimworld-go-ui-screens` |
| Tests, fuzz, benchmarks, long simulations | `rimworld-go-testing` |
| Regression/bug/replay investigation | `rimworld-go-debugging` |
| PR review, merge wave, architecture compliance | `rimworld-go-review-integration` |
| ADR/system/package docs | `rimworld-go-documentation` |

A normal task should use **1–3 skills**, not the full catalog.

## Classification questions

Determine:

- What behavior changes?
- Which package owns that behavior?
- Is authoritative state affected?
- Can order/RNG/save state change?
- Is this a hot path?
- Does a public interface change?
- Does persistence/mod compatibility change?
- Does the task span more than one architectural layer?
- Does this produce or change player-visible art, sprites, icons, UI chrome, renderer presentation, or the asset pipeline?
- Does this implement or change a player-facing screen, window, overlay, HUD layout, or navigation flow?

## Output before coding

Internally establish a task packet:

```text
Task ID:
Objective:
Owner package:
Allowed packages:
Forbidden packages:
Skills:
Canonical docs:
Tests:
Benchmarks:
Determinism impact:
Save/mod impact:
Art/visual impact:
Screen/UI impact:
```

Keep it concise.

## Stop conditions

Do not implement yet if:

- two packages both appear to own the same concept;
- a task requires violating dependency direction;
- a canonical primitive appears to need duplication;
- an existing ADR explicitly conflicts with the requested implementation;
- bulk art production is requested before ART Wave 0 contracts or the §34 prototype gate exist;
- a later UI screen wave is requested before Wave UI-0 framework (primitives, snapshot boundary, command gateway) exists.

In those cases, invoke the architecture/documentation workflow and resolve the contract first.
