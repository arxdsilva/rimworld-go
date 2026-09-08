# rimworld-go Agent Skills

This package contains reusable `SKILL.md` playbooks for AI coding agents working on rimworld-go.

## Location

These playbooks live in `.cursor/skills/` so Cursor auto-discovers them. Do not load all skills into every task. Start with `rimworld-go-router`.

## Skills

| Skill | Purpose |
|---|---|
| rimworld-go-router | Select minimal task context and skills |
| rimworld-go-bootstrap | Repository foundations and CI |
| rimworld-go-feature | Normal feature implementation |
| rimworld-go-hexagonal | Ports/adapters/package boundaries |
| rimworld-go-determinism | Ticks, RNG, ordering, workers, hashes |
| rimworld-go-performance | CPU/memory/scale optimization |
| rimworld-go-pathing | Navigation, regions, movement |
| rimworld-go-ai-jobs | Pawn work, jobs, reservations |
| rimworld-go-persistence-replay | Saves, migrations, replay |
| rimworld-go-content-modding | Definitions and mods |
| rimworld-go-art-assets | Sprites, icons, UI chrome, palettes, pivots, atlases, renderer presentation, asset pipeline |
| rimworld-go-ui-screens | Player-facing screens, windows, overlays, HUD layout, navigation, inspectors |
| rimworld-go-testing | Tests, fuzz, benchmarks, long runs |
| rimworld-go-debugging | Reproduce and fix simulation bugs |
| rimworld-go-review-integration | PR review and multi-agent merge |
| rimworld-go-documentation | ADRs, system docs, AGENTS, handoffs |

## Suggested repo layout

```text
.cursor/
├── rules/          # always-on Cursor recognition
├── skills/         # task playbooks (this folder)
├── handoffs/
├── DOCUMENT_INDEX.md
├── MASTER_SPEC.md
├── HEXAGONAL_ARCHITECTURE.md
├── GO_BEST_PRACTICES.md
├── AI_TOKEN_OPTIMIZATION.md
├── ART_DIRECTION_AND_ASSET_PIPELINE.md
├── MAIN_UI_SCREENS_SPECIFICATION.md
├── RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md
└── RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md

docs/
├── adr/
└── systems/
```

## Global context rule

A normal implementation task should load only:

```text
task
root AGENTS.md
nearest package AGENTS.md
relevant skill(s)
relevant interface/types
tests
one system doc/ADR when necessary
named ART_DIRECTION sections when visual/asset work is in scope
named MAIN_UI_SCREENS sections when a player-facing screen is in scope
```

The full master spec should not be injected into every coding task.

## Source of truth

These skills operationalize the project's existing architecture, token optimization, Go best practices, art direction, screen specifications, and master specification; they do not replace those documents.
