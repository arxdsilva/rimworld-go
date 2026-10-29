# ARCH-006: the sweep — the root keeps only the composition

## Why
After ARCH-003/004/005 every feature and draw layer lives in its own package, but the root still carried the transition: 29 `<pkg>_compat.go` files of one-line wrappers, aliases and converting shims that kept the old call sites compiling, the 32-method `Simulation`, dead fields and the names the guards knew. The sweep deletes what nothing needs, repoints what the root and its tests still call, and leaves the root as the composition (`app.go`, `shell.go`, `nav.go`, `hotkeys.go`, `colony_compose.go`, the boot/pause/loading screens, capture, `renderer.go`'s orchestration) plus the host adapters each feature's `Shell` needs.

## Part 1 — mechanical (lead, one commit)
Tools in the session scratchpad, all `go/types` over the root package with its tests: `unusedroot` (declarations with no reference / tests-only references), `wrapmap` (pass-through wrappers and their targets), `aliasuses` (every use position of a `type/const/var X = pkg.Y` alias or a pass-through function), `deldecls` (delete declarations by name), `movetest` (move declarations into a test file).
1. 392 pass-through wrappers (`func (a *App) x(p...) { return a.win.X(p...) }` and the Renderer's): every root caller repointed onto the target by one word-bounded substitution per wrapper (2,017 sites in 199 files; only lowercase names unique across the root's receiver types), then the wrappers deleted.
2. 1,154 aliases (`type foo = pkg.Foo`, `const`/`var` aliases, pass-through functions): 5,562 use positions rewritten to `pkg.Foo` by exact file:line:col from the type-checker (an import alias such as `artpkg` where a file had a same-named local identifier — 29 files), imports fixed by goimports, then the aliases deleted. Two embedded-alias fields changed name with their type (`colFacRow.FacRow`, `heardCue.Cue`).
3. 667 + 1,147 unreferenced unexported declarations deleted in rounds (a type goes with its methods only when none is referenced); 178 declarations only tests referenced moved into `<file>_test.go` helper files (the exported ones — `NewApp`, `Update`, `Draw`, `Layout` — moved back: cmd/game is their caller). Comment-only files deleted (`camera_smooth.go`, `font.go`, `input_source.go`, `ui_canvas.go`, `ui_text.go`, `gfx/globe/hudlayout/inputframe_compat.go`).
4. `Simulation` is now the union of eight consumer groups in `sim_ports.go` (`SimStepper`, `SimCommander`, `SimFrame`, `SimWorld`, `SimPlacement`, `SimColony`, `SimTabs`, `SimChunks`), kept only for the composition root; its 32-method budget line is deleted.
5. The canvas guard's `logicalDraws` lost the deleted root wrappers (the real code calls `gfx.X`, which the guard already classifies by import); its self-tests' sample sources import `gfx`.

| | before ARCH-006 | after part 1 |
|---|---|---|
| root non-test files / lines | 70 / 14,617 | 62 / 10,931 |
| `App` methods / fields | 607 / 57 | 244 / 57 |
| `Renderer` methods / fields | 334 / 48 | 153 / 48 |
| `Simulation` | 32 methods | 8 embedded groups |
| `DrawArt500/z1` | 1.00 ms, 418 draw calls, 3,826 allocs | the same |

## Part 2 — the renderer side, the shims, the docs (two commits)
- R10 (opus): `Renderer.lod`/`links`/`stoneArt` moved into `world.Pipeline` (which allocates the zoom tiers, the link grid and the stone cache and wires the env grass and crop batches; `Shared` keeps the camera, the cast and the blood); `architect/sprites.go` deleted — architect reads `world.BuildSprite` and co; the ten renderer-side compat files became `layers.go` and three host adapters (`env_host.go`, `creature_host.go`, `cue_host.go`); every remaining Renderer wrapper was inlined at its caller or became a layer method. Renderer 153 → 71 methods, 48 → 40 fields; what is left is orchestration (`advance`, `syncPawns`, the camera, speed, selection, `Draw`), the layer construction (`pipe`, `overlays`, `cueBus`, `liveView`) and the methods the App side still calls (the architect previews, the sound director's loops, the shell's overlay toggles, the minimap, the selection helpers).
- Lead: the sixteen converting shims became aliases of their package types (`architect.ToolDef`/`GhostCell`/`RowRun`, `inspector.HealthOption`/`ItemVerb`, `hud.Alert`/`AlertRow`/`RightCard`/`LetterRow`/`MessageRow`/`StatusChip`/`GlobalPlan`/`Toast`/`ResRow`/`BarStatus`, `dialogs.TradeLine`); the root tests' lowercase field reads were renamed by the compiler's own "but does have field X" hints in a loop (`fixhints.py`), the converters became identities and were inlined, then the aliases were repointed and deleted like the others. `archCategories`/`archTools` are `architect.Categories`/`Tools`.
- S1 (haiku): an `AGENTS.md` per sub-package (33) from the §33 template and each package's `doc.go`/`deps.go`/imports/tests; the root ebiten `AGENTS.md` rewritten as the composition root's contract with a package map and a "where the presentation knowledge went" index; `HEXAGONAL_ARCHITECTURE.md` §4 shows the ebiten tree, §21 the layer and feature rules.

| | before ARCH-006 | after |
|---|---|---|
| root non-test files / lines | 70 / 14,617 | 56 / 10,128 |
| `App` methods / fields | 607 / 57 | 244 / 57 |
| `Renderer` methods / fields | 334 / 48 | 71 / 40 |
| `<pkg>_compat.go` | 29 files | 14 (each a feature's host adapter, shell composition and constructor, plus the few non-pass-through wrappers the composition still calls) |
| `Simulation` | 32 methods | 8 embedded groups |
| sub-packages / their AGENTS.md | 33 / 0 | 33 / 33 |
| `DrawArt500/z1` | 1.00 ms, 418 draw calls, 3,826 allocs | the same |

## Still budgeted (not hidden)
- `App` 57 fields / 244 methods: the composition (`app.go` 28, `hotkeys.go` 15, `capture.go` 13, `input_frame.go` 11, `colony_compose.go` 9, `ui_scale.go` 9, `sound_director.go` 8), the zone commands (`cmds_zone.go` 16 — they belong with `architect.Zones`), the map-side selection draws (`multiselect_render.go` 13) and the wizard's screen-table wrappers (`wizard_compat.go` 20). The screen/scale group, the session group and the twelve feature pointers are the sub-struct candidates for the fields.
- `Renderer` 40 fields / 71 methods, `uikit.Kit` 73/30, `wizard.Draft` 42 fields, `globe.View` 51, `render/creature.Set` 93 methods, `game.Game` 2105/152: every line in `internal/arch/solid_budget.txt` says so and only shrinks.
- The `*_compat_test.go` files hold the helpers that let the root's integration tests reach into a feature (178 declarations moved there); a new test calls the feature directly.

## Findings
- A pass-through wrapper is the only shape a textual repoint can retire safely: the mapper flags anything else (`nonpass`, `variadic`, a name shared by another receiver type — `Kit` was, and deleting it broke `uikit.Owner`).
- Replacing an alias by exact type-checker positions is safe where a word-bounded regex is not (`.id`, `.open`, `rows[1].facRow`); the two failures were embedded alias fields, whose field name is the alias's.
- "Tests-only" is not "dead": `NewApp`, `Update`, `Draw` are called by cmd/game only. Exported names stay in production files.
