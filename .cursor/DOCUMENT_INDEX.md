# Documentation Index

All paths below are relative to this `.cursor/` folder unless noted.

| Need | Read |
|---|---|
| Product/game scope | `MASTER_SPEC.md` |
| Hexagonal architecture and repo boundaries | `HEXAGONAL_ARCHITECTURE.md` |
| Go standards/performance/concurrency | `GO_BEST_PRACTICES.md` |
| Efficient AI context usage | `AI_TOKEN_OPTIMIZATION.md` |
| Visual production, sprites, icons, UI chrome, asset pipeline | `ART_DIRECTION_AND_ASSET_PIPELINE.md` (playbook: `skills/rimworld-go-art-assets/SKILL.md`) |
| Per-screen layout, navigation, commands, and states | `MAIN_UI_SCREENS_SPECIFICATION.md` (playbook: `skills/rimworld-go-ui-screens/SKILL.md`) |
| Ordered vanilla gameplay feature backlog | `RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md` |
| Ordered player-facing UI/UX backlog | `RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md` |
| Per-task AI handoffs | `handoffs/` |
| Task-specific playbooks (start here for any new task) | `skills/rimworld-go-router/SKILL.md` |
| Multi-area work across parallel agents | `PARALLEL_WORK.md` (playbook: `skills/parallel-work/SKILL.md`) |
| Cursor always-on recognition rules | `rules/` |
| Global contributor rules | `../AGENTS.md` |

Recommended agent loading order:

1. task;
2. `AGENTS.md`;
3. package `AGENTS.md`;
4. interfaces/tests;
5. relevant system doc/ADR;
6. named `ART_DIRECTION_AND_ASSET_PIPELINE.md` sections when visual/asset work is in scope;
7. named `MAIN_UI_SCREENS_SPECIFICATION.md` sections when a player-facing screen is in scope;
8. only then broader documents.
