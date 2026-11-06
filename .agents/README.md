# .agents index

The only index of agent guidance. Paths are relative to `.agents/` unless they start with `/` or a package path. Load by section (grep the heading), never a whole big file.

## 1. Rules

| File | What |
|---|---|
| `/AGENTS.md` | Always loaded (Claude, Cursor, Codex): the non-negotiables. |
| `<pkg>/AGENTS.md` | Loads when working in that package. Format: `templates/PACKAGE_AGENTS.md`. |

## 2. Specs (`specs/`)

| File | What | Load when | Key sections |
|---|---|---|---|
| `MASTER_SPEC.md` | Product and system scope | Product intent or a system's contract is needed; grep `# <n>.` | §6-8 clock, determinism, phases; §17-27 pawns, jobs, reservations; §45-47 saves, replay; §48-50 modding; §53-55 budgets, concurrency; §62-70 coding rules, handoff, PR done; §77-78 guardrails, anti-patterns |
| `HEXAGONAL_ARCHITECTURE.md` (HEX) | Layers, ports, adapters, repository ownership | Package, dependency or interface design | §3 dependency rule; §6 consumer-owned ports; §10-12 commands, read models, simulation owner; §19-23 persistence, content, renderer, path, time boundaries; §26-27 DI, init flow |
| `GO_BEST_PRACTICES.md` (GO) | Mandatory Go rules | Writing or reviewing Go | §5-7 interfaces, constructors; §9-11 errors, context; §12-14 concurrency, determinism; §25-28 fixed-point, time, RNG; §29-30 allocation, pools; §38 function size; §86 search before creating |
| `SOLID_GO.md` | SOLID in Go, size limits, arch guard | A change adds methods or fields to a large type, declares an interface, adds package state or `init()`, or splits a package | §8 measurable limits; §9 package-level state; §10 splitting a god type; §11 what not to do |
| `AI_TOKEN_OPTIMIZATION.md` | Context and token discipline | Planning context; hook questions | §3 progressive disclosure; §5 task packet; §8 handoff compression; §11 search before read; §32a hooks and measurement |
| `PARALLEL_WORK.md` | Fan-out to parallel subagents | A plan or task spans several areas | §0 plan includes its workers; §1 lead first; §2 disjoint ownership; §5 coordination; §5a lifecycle; §6 model choice |
| `ART_DIRECTION_AND_ASSET_PIPELINE.md` (ART) | Visual production, sprites, pipeline | Player-visible art or asset tooling, via `rimworld-go-art-assets` (sections only) | §1 definition of done; §3-6 scale, pivots, camera, lighting; §7 palette; §10-11 pawns, animals; §19 layout, naming; §24-25 metadata, pipeline; §34 prototype gate; §35 acceptance |
| `MAIN_UI_SCREENS_SPECIFICATION.md` (MAIN_UI) | Per-screen layout, navigation, commands, states | A player-facing screen, via `rimworld-go-ui-screens` (that screen's section only) | One `##` block per screen; closing `Wave UI-0`..`UI-10` sections give the order |
| `SOCIAL_SYSTEM_CONTRACT.md` | ENG-003 social/host-system contract | Social, host or prison work | C0 global rules; C1 enums; C2 domain packages; C3 application; C4 UI; C5 ownership |
| `CLEAN_ROOM.md` | Licensing, clean-room policy and the ENG-003 exceptions | Using external reference data, wiki numbers, mods | Read whole (short) |
| `RELEASES.md` | Tag-driven release workflow, packaging, notes | Tags, CI, packaging, new runtime asset directories | Read whole (short) |

## 3. Backlog (`backlog/`)

| File | Order of |
|---|---|
| `RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md` | Vanilla gameplay features |
| `RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md` | Player-facing UI/UX |

Grep the section by feature name or ID; never read a checklist whole. Update the item's checkbox in the same change: `[x]` full acceptance met, `[-]` partial (short note on what is missing), `[!]` blocked (why). Art and screen checklists live inside the ART and MAIN_UI specs.

## 4. Gaps (`gaps/`)

Each file holds the decisions, wiki constants and phase rows for one area. Load only for tasks of that family.

| ID family | File | Load only for |
|---|---|---|
| AIL-* | `AILMENTS_GAP_ANALYSIS.md` | Conditions that build up and fade (exposure, disease, infection, toxins, malnutrition); no pawn held down forever |
| STK-AL | `ALLOW_GAP_ANALYSIS.md` | Forbid/allow for things and buildings, allowed areas |
| STK-AG, AX-* | `ARCHITECT_GAP_ANALYSIS.md` | Architect categories and items vs the vanilla catalog |
| BED-* | `BEDS_GAP_ANALYSIS.md` | Beds, rest, room quality, comfort, beauty, outdoors |
| CBT-* | `COMBAT_GAP_ANALYSIS.md` | Fighting: damage model, armor, weapons, turrets, traps, explosions, AI, combat UI |
| CSE-* | `COMMON_SENSE_GAP_ANALYSIS.md` | Common Sense habit rules (tidying, bills, packs, joy, wandering) |
| DEF-* | `DEFENCE_GAP_ANALYSIS.md` | Ranged/melee, armor, cover, traps, turrets, mortars, doors, raids, mechanoids |
| FOOD-* | `FOOD_GAP_ANALYSIS.md` | Growing, animals and butchering, cooking, preservation, eating, hunger |
| GEAR-*, OUT-* | `GEAR_GAP_ANALYSIS.md` | The Gear tab and apparel policies (outfits) |
| HRV-* | `HARVEST_GAP_ANALYSIS.md` | Who harvests, batching, yield and speed |
| MED-* | `MEDICAL_GAP_ANALYSIS.md` | Wounds, bleeding, tending, infection, diseases, anatomy, bionics, surgery, aging |
| MIN-* | `MINING_GAP_ANALYSIS.md` | Mining time and yield, ore, cave-ins, veins, deep drill, infestation |
| CTL-* | `MOUSE_CONTROLS_GAP_ANALYSIS.md` | What the mouse does on the map, bar, windows, dialogs |
| MAP-* | `MULTIMAP_GAP_ANALYSIS.md` | Several maps ticking at once; caravan and pod rules; map switching |
| NST-* | `NEAT_STORAGE_GAP_ANALYSIS.md` | The 27 Neat Storage furniture pieces |
| OPT-* | `OPTIONS_GAP_ANALYSIS.md` | The Options screen and the translation layer |
| SIM-061..066 | `PAWN_INTERACTION_GAP_ANALYSIS.md` | Orders on a pawn, rescue, feeding patients, visiting, mental breaks involving others |
| PWR-* | `POWER_GAP_ANALYSIS.md` | Grid joining, visible wire, Reconnect |
| PRD-* | `PRODUCTION_GAP_ANALYSIS.md` | Benches, bills, recipes, quality, materials, deterioration, art, stockpile filters |
| REF-* | `REFUEL_GAP_ANALYSIS.md` | Feeding fuel to campfires, lamps, generators, launchers, smithies |
| RSC-* | `RESCUE_GAP_ANALYSIS.md` | Who is carried to which bed, carrier requirements, aftermath |
| RSCH-* | `RESEARCH_GAP_ANALYSIS.md` | Research speed, tech levels, project table, benches, Research tab |
| RHUD-* | `RIMHUD_GAP_ANALYSIS.md` | The RimHUD colonist card, layout editor, presets, widgets |
| SHT-* | `SHOOTING_GAP_ANALYSIS.md` | Accuracy, aim time, interception, shot information |
| STK-ST | `STORAGE_GAP_ANALYSIS.md` | Stack counts per tile, the dumping stockpile |
| TEMP-* | `TEMPERATURE_GAP_ANALYSIS.md` | Climate, room thermodynamics, comfort, clothing, temperature UI |
| WRK-* | `WORK_PRIORITIES_GAP_ANALYSIS.md` | The Work tab, tie-break between work types, task order |
| WG-* | `WORLDGEN_GAP_ANALYSIS.md` | Create world and starting-site screens, globe look |

## 5. Handoffs (`handoffs/`)

`INDEX.md` lists each ledger with its ID families, then every task ID with title, date and `ledger#anchor`; look an ID up there and open that section. A finished task appends one section to its family's ledger (`templates/HANDOFF.md`) and rebuilds the index with `python3 scripts/handoff_index.py`. Originals: `git show 76d605d04:.agents/handoffs/<stem>.md`.

## 6. Skills (`skills/<name>/SKILL.md`)

Claude discovers them by description; Cursor and Codex open the router first. A task uses 1-3.

| Skill | Use when |
|---|---|
| `rimworld-go-router` | Task type or context is unclear; picks skills and spec sections |
| `rimworld-go-feature` | A normal feature or domain behaviour |
| `rimworld-go-hexagonal` | Package, dependency or interface design; splitting a god type |
| `rimworld-go-determinism` | Ticks, RNG, ordering, workers, hashes, saves, migrations, replay |
| `rimworld-go-performance` | Hot loops, scale, allocations, data layout |
| `rimworld-go-pathing` | Paths, regions, movement, congestion |
| `rimworld-go-ai-jobs` | Pawn work discovery, jobs, reservations |
| `rimworld-go-content-modding` | Definitions, YAML/JSON content, mods |
| `rimworld-go-art-assets` | Sprites, icons, UI chrome, palettes, pivots, atlases, renderer presentation, asset pipeline |
| `rimworld-go-ui-screens` | Screens, windows, overlays, HUD layout, navigation, inspectors |
| `rimworld-go-creatures` | A new or changed animal: roster, rig art, four generated sounds |
| `rimworld-go-testing` | Tests, fuzz, benchmarks, long runs |
| `rimworld-go-debugging` | Reproducing and fixing simulation bugs |
| `rimworld-go-review-integration` | PR review, merge waves, architecture compliance |
| `rimworld-go-documentation` | ADRs, system docs, package AGENTS, handoffs |
| `parallel-work` | A task spans 2+ areas, a multi-area plan, or the user asks for parallel agents |

## 7. Templates (`templates/`)

| File | For |
|---|---|
| `HANDOFF.md` | One ledger section per finished task |
| `TASK_PACKET.md` | The task packet established before coding |
| `PACKAGE_AGENTS.md` | A nested `<pkg>/AGENTS.md` |

## 8. Hooks (`hooks/`)

`read_guard.py` (denies large shell views and whole-file Reads) and `agent_model_guard.py` (requires a named model on `general-purpose` agents), both run from `.claude/settings.json`. Tests: `python3 .agents/hooks/test_hooks.py`. Details: `AI_TOKEN_OPTIMIZATION` §32a.

Guard: `internal/arch/agents_test.go` (run by `make arch` and CI) fails on paths into the old tool folders, broken links or `.agents/` paths, ledger anchors that name no section, `.claude/skills` links that do not match `.agents/skills`, ledger sections missing from `INDEX.md`, and a root `AGENTS.md` over its byte budget (`internal/arch/agents_budget.txt`, only shrinks).

## 9. Tool adapters

| Path | Holds | Why it must sit there |
|---|---|---|
| `.cursor/mcp.json` | Cursor MCP servers | Cursor reads MCP config only from `.cursor/` |
| `.claude/settings.json` | Permissions, hooks, attribution settings | Claude Code reads settings only from `.claude/` |
| `.claude/agents/Explore.md` | The Explore subagent definition | Claude Code loads subagents from `.claude/agents/` |
| `.claude/skills/*` | Symlinks to `.agents/skills/*` | Claude Code discovers skills only from `.claude/skills/` |
