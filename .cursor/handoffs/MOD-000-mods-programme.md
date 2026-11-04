# MOD-000: RimHUD, Neat Storage and Common Sense programme

Asked 2026-11-03: gap-analyse the three mods and add everything they add, RimHUD as complete as possible; meat art as steaks; then commit, tag, push; no questions.

Gap documents, each with the rows and work per row: `.cursor/RIMHUD_GAP_ANALYSIS.md` (RHUD, packages DOS-n), `.cursor/NEAT_STORAGE_GAP_ANALYSIS.md` (NST, packages WP-A…), `.cursor/COMMON_SENSE_GAP_ANALYSIS.md` (CSE, packages WP0…). A row's Work column is the job description. Meat art is done (`ART-MEAT-steaks.md`).

## Lead decisions on the docs' open questions

| Question | Decision |
|---|---|
| NST Architect placement | Plan A: a thirteenth category `storage`; `hudlayout.ArchCatsH` grows to fit. |
| NST sculptures admirable (S1) | Yes, they ship with the first change. |
| NST joy kind for Admire | Reuse `JoySolitary`: no saved array grows. |
| `SaveVersion` | One bump, 12 to 13, for all three. No migration step: new fields are additive and zero-safe. `testdata/v12.sav` stays and must keep loading. CSE's "no bump" is overridden because NST adds kinds an older binary can't read. |
| CSE-002 | On by default, as the mod has it. Stated as a balance change in the release notes. |
| CSE-010 | The opt-in `drug.MoodLift` preset ships. |
| Pack capacity | 40 kg times Manipulation, original tuning. |
| RHUD default | `HUD.Enabled` stays false until the last wave, then flips on for new and existing settings, since the user asked for the feature in the game. The switch stays in Options. |
| Numbers | NST sizes and per-cell capacities follow the mod's table (new exception next to ENG-003). All other NST numbers and the CSE thresholds are tuned by us and said to be. |

## Budget rule

`game` is at its ceiling (176 files, 53224 lines; `Game` 1837 methods, 64 fields). W0-P frees room first. `solid_budget.txt` is lowered only at the end, by the lead, to what is measured.

## Parallel workers

Areas: domain, simulation, ports, renderer and UI, outbound adapters, art, docs. The lead writes the contract first (W0-C) and is the only one to edit: `go.mod`, `Makefile`, `cmd/**`, `assets/palettes/*.yaml` unless named, `artlib/{contracts.py,stage.py}`, `assets/source/blender/build.py`, `command.go`, `job.go`, `save.go`, `game/testdata/*`, goldens, `solid_budget.txt`, `.cursor` checklists and specs, README, handoffs other than the agent's own.

RAM: `make agent-budget` before each wave (last: light 15, art 15, heavy 11). At most 15 agents. Agents run targeted tests with `GOFLAGS=-p=2` and `-race`, never a whole art-loading package, and never commit. The lead commits after each wave's gates. Each agent is stopped once its work is committed.

### Wave 0: room and contract

| id | model and reason | job | owns | must not touch | tests |
|---|---|---|---|---|---|
| W0-P | opus: hash-identical extraction | Free at least 1000 lines of package `game` | `game/clean.go` (renamed `sense_host.go`), `game/idle.go`, `game/drugs.go`, new sub-packages, call sites | Behaviour, `save.go` schema, `solid_budget.txt`, goldens | `bin/benchmark -hash` identical, moved code's tests |
| W0-C | lead | The three contracts: kinds, profile, settings record, commands, job kinds, read models, roles and inert hooks, `SaveVersion` 13, schema list | as listed in the docs' contract sections | | `make arch`, hash identical |
| W0-A | opus: art always | `ui.dossier_*` tokens and icons (DOS-0A) | `assets/palettes/ui.yaml`, `uikit/widgets.go`, icon sources | Other Go | `make art-check`, uikit tests |

### Wave 1: foundations, parallel

| id | model and reason | job | owns | must not touch |
|---|---|---|---|---|
| NST-A | sonnet: tables | WP-A: real rows, beauty, HP, research keys | `construction/{neat.go,neat_test.go,furniture.go,ax_hp.go}`, the 27 rows, `research` | other packages |
| NST-B | opus: invariants | WP-B: profile table, sets, `UniverseOf`, `Link`, `ZoneKind`, `onShelf`, item slots, benchmark | `zone/*`, `game/store/*`, line-neutral `game/{zones,placement}.go` | `tick.go`, `save.go` |
| NST-G | sonnet: small mode | WP-G: Admire | `game/recreate/*` except `seek.go` hunks owned by CSE-W7 later | `game` |
| NST-D | opus: hot path | WP-D: slots in the renderer | `render/world/*`, `art/catalog.go` | architect, inspector, root |
| NST-E1, E2, E3 | opus: art | Three families | `families/neat{display,racks,bulk}.py` | contracts, palettes, `build.py` |
| DOS-1 | opus: sim internals | Fill `Card`, `SetName`, remove `LearnedThisWeek`, settle UNCLASSIFIED rows | `game/dossierview/`, `game/{skills,bio_view,body}.go`, repointing | other ebiten files |
| DOS-2 | sonnet: pure logic | `sheet` | `dossier/sheet/` except `types.go` | everything else |
| CSE-W1 | sonnet: screen | Habits tab | `options/habits_tab.go`, journey file | `game/` |
| CSE-W2 | opus: saved state | Pack | `inventory/pack*.go`, `game/sense/pack*.go`, `inspector/gear_rows.go` | bills, eating |

### Wave 2: behaviour and the card

| id | model and reason | job | owns |
|---|---|---|---|
| NST-C | sonnet | WP-C: Architect category, 27 cards, inspector | `architect/*`, `inspector/{storage_ui,cmds_defence}.go`, `ebiten/journey_neat_test.go` |
| DOS-3 | sonnet | `dossier.Panel` | `dossier/` except `deps.go`, `setup_*` |
| DOS-4 | opus: composition root | Pane hook, header, tab strip, floating rect, rename | as in the RHUD doc |
| CSE-W3 | opus: tidy | CSE-017…027 | `game/sense/tidy*.go`, `game/care/tend.go`, `inspector/cmds_build.go` |
| CSE-W4 | opus: bills | CSE-018 call, 029…035, 040 | `game/craft/{gather,haul,shop,packgather,deps}.go` |
| CSE-W5 | sonnet | CSE-005, 037…039 | `game/sense/{spare,unload,backtopack}.go`, `inspector/gear_rows.go` |
| CSE-W6 | sonnet | CSE-012…015 | `item/meal.go`, `game/diet/choose.go`, `craft/view.go`, `recipe/flavour.go` |
| CSE-W7 | opus: joy | CSE-001, 002, 007, 008, 010, 011 | `game/recreate/seek.go`, `care/{gift,feed}.go`, `game/sense/{topoff,relax}.go`, `drug/policy.go`, `windows/assign.go` |
| CSE-W8 | sonnet | CSE-028, 041…043 | `game/sense/wander.go`, `game/snow/*`, `room/*`, `game/thermo/cache.go` |
| CSE-W9 | haiku | CSE-046 | `recipe.go` (`Kinds`), `inspector/bill_*.go`; `craft/view.go` after CSE-W6 |

### Wave 3: the card's settings

DOS-5 (sonnet, setup and colours), DOS-6 (sonnet, editor, presets, `cardpresetfile`, `openfolder`), DOS-7 (sonnet, letters), DOS-8 (haiku, lessons). Owned files as in the RHUD doc.

### Wave 4: finish

DOS-9 work and everything else by the lead: timing overlay, default flip, goldens re-pinned with reasons, budgets lowered, checklists, specs, README, `DOCUMENT_INDEX.md`, handoffs, final gates, commit, push, tag.

Lead after each wave: `go build ./...`, `make fmt vet lint`, `make arch`, the full `-race` suite run alone, `go test -count=3 -run 'Determin|SameSeed|Hash|Golden' ./internal/...`, `make art-check` when art changed, the Windows no-cgo build, `cmd/benchmark` against `scratchpad/bench_mods_before.txt`, and a visual review for presentation.

## Notes carried into later packages

- DOS-1: fill `Card.Stats` (stat lines read 0 otherwise) and `Card.BreakAt` (the imminent-break rung never fires otherwise); `Card.WorkType` is -1 when there is none. There's no per-condition pain and no per-wound tone in `PartView`: add them if the rows need them. Read `DOS-2-sheet.md` for the rows settled only through `Card.ExtraLines` (228, 235, 236, 245, 251, 255).
- DOS-3: draw from the API listed in `DOS-2-sheet.md` (`Catalog()` is a function; `Move`, `Remove`, `Add` take a root pointer and a path; tips are `[]TipLine`).
- NST-C: `windows/storage.go:192` and `architect/zone_select.go:268` call `game.ProfileFilterRows(z.Profile, z.Filter)` when `z.Profile != ""`.

## Contract as written (W0-C), where it differs from the CSE doc

- `sense.Sense` lives on `worldState`, reached by `senseOf(g)` (`game/sense_host.go`). Its state is plain fields and maps, and `sense/save.go` already saves all of them. No component adds a field.
- The doc's `Packs` component is `Gear` (`sense/gear.go`): pack, spare, unload, back to pack. `Packs` is the map.
- `Tidy.After` returns true when it started the interrupted job again. `TopOff.Wants` means "wants recreation", true by default; false sends the pawn to look for work.
- Roles are per component: `TidyRoles`, `GearRoles`, `JoyRoles`, declared in the component's file, with adapters in that owner's `game/sense_*_host.go` and one line in `senseOf`.
- `craft`, `care`, `snow`, `recreate` roles are declared by the agents that use them, with their adapter, not by W0-C.
- A command kind the client does not emit yet needs no owed entry: the guard fails on those. The change that first emits one ships its journey.

## Outcome (2026-11-04)

All wave packages merged; released as v0.1.25 (v0.1.24 failed CI and published no archives, see CI-002).

| Mod | Rows | Done | Excluded |
|---|---|---|---|
| RimHUD | 504 | 429 | 75 |
| Neat Storage | 27 | 27 | 0 |
| Common Sense | 55 | 45 | 10 |

Each gap document lists under Counts where a done row differs from the mod. DOS-9 records the flip to on and the bugs it exposed.

Simulation benchmark, grown fixture, seed 1, tick mean in ms, before and after the programme: 120 pawns 0.176 and 0.188; 250 pawns 0.252 and 0.243; 500 pawns 0.292 and 0.308. Frame cost of the card, from `BenchmarkDossierDraw`: about 0.1 ms and 555 allocations per draw.

Not touched by this programme, still open: the `newMapState` hash window, `SaveInfo.Day` and `RIMWORLD_UPDATE_SAVE_FIXTURE`.
