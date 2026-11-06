---
name: rimworld-go-router
description: Route any rimworld-go task to the smallest set of skills and spec sections. Use first for ambiguous, cross-cutting or new tasks.
---

# rimworld-go Task Router

Tools without skill discovery (Cursor, Codex) open this file at task start.

Use this skill before implementation when the correct subsystem or context set is not already obvious.

## Goal

Select the smallest sufficient context and skill set. Never load every document or skill by default.

## Documents

Everything is indexed in `.agents/README.md`; load spec sections, never whole files. A normal task loads: the task, root `AGENTS.md`, the nearest package `AGENTS.md`, relevant interfaces and tests, and 1-3 skills.

## Routing table

| Task | Add skill |
|---|---|
| New feature or domain behavior | `rimworld-go-feature` |
| Package/dependency/interface design | `rimworld-go-hexagonal` |
| Splitting a god type, growing `App`/`Renderer`/`Game`, a failing `internal/arch` guard | `rimworld-go-hexagonal` + `.agents/specs/SOLID_GO.md` |
| Tick order, RNG, authoritative state, replay determinism, save/load, schema, snapshots, replay | `rimworld-go-determinism` |
| Hot loops, scale, allocations, data layout | `rimworld-go-performance` |
| Paths, regions, movement, congestion | `rimworld-go-pathing` |
| Intents, work discovery, jobs, reservations | `rimworld-go-ai-jobs` |
| Definitions, YAML/JSON content, mods | `rimworld-go-content-modding` |
| Sprites, icons, UI chrome, palettes, pivots, atlases, renderer presentation, asset pipeline | `rimworld-go-art-assets` |
| Player-facing screens, windows, overlays, HUD layout, navigation, inspectors, management tables | `rimworld-go-ui-screens` |
| A new or changed animal/creature species (roster, art, sounds, behaviour) | `rimworld-go-creatures` |
| Tests, fuzz, benchmarks, long simulations | `rimworld-go-testing` |
| Regression/bug/replay investigation | `rimworld-go-debugging` |
| PR review, merge wave, architecture compliance | `rimworld-go-review-integration` |
| ADR/system/package docs | `rimworld-go-documentation` |
| Task spans 2+ areas (domain, sim, ports, renderer, adapters, cmd, art, content, docs) or the user asks for parallel agents | `parallel-work` |
| Planning a multi-area task (plan mode, a plan file, a phase plan) | `parallel-work` (§0: the plan includes its Parallel workers section) |

A normal task uses 1-3 skills.

## Classification questions

Determine:

- What behavior changes?
- Which package owns that behavior?
- Is authoritative state affected?
- Can order/RNG/save state change?
- Is this a hot path?
- Does a public interface change?
- Does persistence/mod compatibility change?
- Does the task span more than one architectural layer? (If so, plan it with `parallel-work`.)
- Does this produce or change player-visible art, sprites, icons, UI chrome, renderer presentation, or the asset pipeline?
- Does this implement or change a player-facing screen, window, overlay, HUD layout, or navigation flow?

## Output before coding

Fill `.agents/templates/TASK_PACKET.md` internally; keep it concise.

## Stop conditions

Do not implement yet if:

- two packages both appear to own the same concept;
- a task requires violating dependency direction;
- a canonical primitive appears to need duplication;
- an existing ADR explicitly conflicts with the requested implementation;
- bulk art production is requested before ART Wave 0 contracts or the §34 prototype gate exist;
- a later UI screen wave is requested before Wave UI-0 framework (primitives, snapshot boundary, command gateway) exists.

In those cases, invoke the architecture/documentation workflow and resolve the contract first.
