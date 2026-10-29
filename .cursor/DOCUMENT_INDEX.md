# Documentation Index

All paths below are relative to this `.cursor/` folder unless noted.

| Need | Read |
|---|---|
| Product/game scope | `MASTER_SPEC.md` |
| Hexagonal architecture and repo boundaries | `HEXAGONAL_ARCHITECTURE.md` |
| Go standards/performance/concurrency | `GO_BEST_PRACTICES.md` |
| SOLID in Go: type/interface/package size limits, splitting god types, the arch guard | `SOLID_GO.md` (guard: `internal/arch/solid_test.go`, `make arch`) |
| Efficient AI context usage | `AI_TOKEN_OPTIMIZATION.md` |
| Visual production, sprites, icons, UI chrome, asset pipeline | `ART_DIRECTION_AND_ASSET_PIPELINE.md` (playbook: `skills/rimworld-go-art-assets/SKILL.md`) |
| Per-screen layout, navigation, commands, and states | `MAIN_UI_SCREENS_SPECIFICATION.md` (playbook: `skills/rimworld-go-ui-screens/SKILL.md`) |
| Adding or changing an animal/creature (roster, art, four generated sounds) | `skills/rimworld-go-creatures/SKILL.md` (rule: `rules/creatures.mdc`) |
| Ordered vanilla gameplay feature backlog | `RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md` |
| Ordered player-facing UI/UX backlog | `RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md` |
| Medical system gap analysis vs RimWorld, wiki constants, MED phases | `MEDICAL_GAP_ANALYSIS.md` |
| Pawn-to-pawn interaction gap analysis vs RimWorld (rescue, orders on a pawn, feeding patients, visiting, mental breaks), wiki constants, SIM-061…066 | `PAWN_INTERACTION_GAP_ANALYSIS.md` |
| Per-task AI handoffs | `handoffs/` |
| Task-specific playbooks (start here for any new task) | `skills/rimworld-go-router/SKILL.md` |
| Multi-area work across parallel agents | `PARALLEL_WORK.md` (playbook: `skills/parallel-work/SKILL.md`) |
| New-game flow, globe, factions, storytellers, difficulty, colonist identity, water, big maps (plan of record) | `NG1_NEW_GAME_PLAN.md` |
| Sentry turret, trade, radio console: contract, tuning table, phases and ownership (PLAY-024) | `PLAY024_PLAN.md` |
| Cursor always-on recognition rules | `rules/` |
| Global contributor rules | `../AGENTS.md` |

Recommended agent loading order:

1. task;
2. `AGENTS.md`;
3. package `AGENTS.md`;
4. interfaces/tests;
5. relevant system doc/ADR;
6. `SOLID_GO.md` §8–§10 when a change adds methods or fields to `App`, `Renderer` or `Game`, declares an interface, adds a package-level var or `init()`, or creates or splits a package (`make arch` tells you);
7. named `ART_DIRECTION_AND_ASSET_PIPELINE.md` sections when visual/asset work is in scope;
8. named `MAIN_UI_SCREENS_SPECIFICATION.md` sections when a player-facing screen is in scope;
9. only then broader documents.
