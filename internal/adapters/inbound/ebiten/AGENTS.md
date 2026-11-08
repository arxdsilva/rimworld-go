# internal/adapters/inbound/ebiten

## Purpose
Interactive Ebitengine client: screen routing, programmatic UI chrome and the colony map viewport. Hosts the player-facing session: boot, menus, the new-game wizard, pause, and the colony viewport with its HUD chrome. Screens consume read models (`Snapshot()`, `World()`) and emit session intents plus typed gameplay commands via `Enqueue`.

Since ARCH-001…006 (`.agents/specs/SOLID_GO.md`, `.agents/handoffs/ARCH.md`) this root package is the **composition root** only. Every feature and every draw layer is its own package under this directory, each behind a small consumer-owned interface; the root constructs them (`NewApp`, `New`) and satisfies their interfaces. The guard `go test ./internal/arch/` enforces the size limits and the import direction: a sub-package never imports this root, a feature package never imports another feature package, a `render/*` layer never imports a feature.

## Owns

| package | owns | its AGENTS.md |
|---|---|---|
| `ports/` | the roles a feature sees of the shell (`Hinter`, `Sizer`, `Commands`, `World`, `View`, `PawnSel`, `Overlays`, `Speed`, `Settings`, `Panels`, `Widgets`, `Saves`, `Dispatcher`), the `Intent`s and `Screen` | `ports/AGENTS.md` |
| `ids/` | widget and click ids | `ids/AGENTS.md` |
| `gfx/`, `camera/`, `art/`, `globe/`, `globemath/`, `inputframe/`, `hudlayout/`, `uikit/` | the shared leaves: canvas and text, the camera, the art catalog, the globe, the input frame, the pure HUD layout, the immediate-mode widget kit | each dir |
| `render/` | `View`: one frame's context (camera, art, this frame's world and pawns, clocks, options) filled once per frame by the root `Renderer` | `render/AGENTS.md` |
| `render/{motion,light,sprite,env,creature,overlay,cue,minimap,world}` | the draw layers: movement tracks, lighting and shadows, sprite painting and linked art, ground cover and weather, creatures, map overlays, sound cues, the minimap, the world draw order with its zoom tiers | each dir |
| `selection/` | what is selected (`Model`) | `selection/AGENTS.md` |
| `dossier/`, `dossier/sheet/` | the colonist card and its setup modal; `sheet` is its pure logic | `dossier/AGENTS.md` |
| `mainmenu/`, `wizard/`, `options/` | the main menu, the New Colony flow, the Options screen with the settings store and save slots | each dir |
| `windows/`, `architect/`, `orders/`, `dialogs/`, `learn/`, `devtools/`, `perf/`, `inspector/`, `hud/` | the colony's features: the main-tab windows, the Architect, the order flow, trade/comms/caravan, the learning helper, developer mode, the frame-timing instruments, the selection's inspector, the HUD regions | each dir |

In this package:
- `App` (`app.go`) — the `ebiten.Game`: boot, the screen stack (`nav.go`), the pause overlay, the Update time budget (`update_budget.go`), the session (`startSession`/`dropSession`), one pointer per feature.
- `shell` (`shell.go`) — one small type per `ports` role, composed into `App.shell`; the screen table and the click routes `NewApp` fills (never `init()`); `shellDispatch.Do` is the one place that knows how the panels exclude each other and how selection and camera meet.
- `<pkg>_compat.go` — each feature's root side: its `Host` adapter, its `Shell` composition and its constructor. Nothing else belongs there; a wrapper that only forwards is deleted in the change that repoints its last caller (SOLID_GO §11).
- `colony_compose.go` — the colony screen's composition: the widget declaration order, the draw order over the features, the chrome hit union, the `hud()` layout memo over `hudlayout.Compute`.
- `hotkeys.go`, `input_frame.go` — the context stack and action routing (UI-016); `ui_scale.go`, `world_scale.go` — logical, physical and map pixels (PLAY-012, PERF-002); `background.go`, `sound_director.go`, `capture*.go`, `screen_boot.go`, `screen_loading.go`, `screen_pause.go`.
- `Renderer` (`renderer.go`) — the colony camera, speed tiers, the selected pawn, the command queue, `advanceTicks` and `Draw`: fills `render.View` and calls the layers. The host adapters the layers need (`envHost`, `creatureHost`, `cueHost`, …) live beside each layer's compat file. `Simulation` is the union of eight consumer groups (`sim_ports.go`), stored only here and in `internal/compose/game`.

## Depends on
- `internal/application/game` through the consumer-owned groups in `sim_ports.go` (`SimStepper`, `SimCommander`, `SimFrame`, `SimWorld`, `SimPlacement`, `SimColony`, `SimTabs`, `SimChunks`); a feature or layer declares its own smaller interface over the same methods.
- `internal/domain` value types that appear on the read models (`clock`, `entity`, `schedule`, `command`, `work`, `skill`, …) and the NG-1 setup types the wizard carries (`planet`, `site`, `scenario`, `difficulty`, `storyteller`, `faction`, `pawn/pawngen`, `pawn/look`).
- Ebitengine, image/png, OS path lookup for generated art and the menu splash; `internal/platform/config` (settings), `internal/platform/input` (the key-binding registry and resolver).
- `pkg/mathx` (`Abs`).

## Must not depend on
- concrete `*game.Game` pointers except at `internal/compose/game` (composition root) and the compile-time `var _ Simulation = (*game.Game)(nil)` assertion
- `ebiten/v2/audio` or any audio library: sound goes through the `Sound` port (`sound.go`; `internal/compose/game` wires `internal/adapters/outbound/ebitenaudio`)
- a sub-package importing this root; a feature importing another feature (cross-feature requests are `ports.Intent`s the shell dispatches)

## Contracts
- The simulation is created only when the player presses Start (or a capture starts one). Menus run with `renderer == nil`; every shell role is safe then (reads return the zero value, writes do nothing).
- The wizard turns its draft into a `game.Setup` and builds the colony step by step behind the loading screen (`wizard.Loader`); `internal/compose/game` wraps `game.NewColonyBuilder`. Return to main menu drops the renderer so the menu stays headless. The pause overlay skips `Renderer.advance` and map input.
- Escape on colony (`handleColonyEscape`): hidden UI, order/debug menu, formation drag, Move mode, Architect tool, Architect, windows, building/frame/zone/group/animal/item/plant/pawn selection, then pause — each feature exposes its `Escape`-style call; the root keeps the order.
- HUD, inspector, Architect and windows consume pointer hits before map selection (`colonyChromeHit`). Disabled buttons do not navigate; they expose a reason string.
- Input (UI-016): only `inputframe` reads Ebitengine's keyboard and mouse; `App.Update` polls once into the frame and resolves actions against the context stack (text > modal > tool > panel > map > global > dev). A pressed key fires at most one action; a used mouse press is taken so later handlers skip it. Bindings live in `config.Settings.Controls`; labels use `ports.Settings.KeyHint`, so they always show the live key.
- UI scale (PLAY-012, UI-024): the UI lays out in logical pixels and draws through `gfx`; anything the camera places stays in map pixels (`mapCursor()`, `mapViewport()`, `mapScale()`), never `pw`/`uiScale()` outside the world pass. `TestUIDrawsThroughCanvas` rejects raw drawing outside its exemption list (each entry has a reason) and `TestMapPixelsComeFromMapCursor` checks every `TileAt`/`screenToWorld` caller, in the sub-packages too.
- Settings are presentation-only and never reach the simulation, except Max colonies, sent as `command.SetColonyCap` (a rule the player sets); `-capture` runs get no `SettingsHooks` and no `Sound`, so captures are deterministic.
- Commitment colonies: one save slot written by the game itself; pause shows Save & Quit, Load and developer mode are disabled (`options`).

## Invariants
- Screen ids and widget bounds never enter domain or save state.
- Sprite animation uses the presentation frame counter (`View.Present`), never sim ticks, to pick frames; tile lerp uses sim ticks vs the step tick.
- Menu chrome colours match `assets/palettes/ui.yaml` (`TestUIPaletteMatchesYAML` in `uikit`).
- The main-menu splash and the diorama are optional; without them the gradient backdrop is drawn. Their motion runs on `menuTick`, never the sim.
- Size limits (`.agents/specs/SOLID_GO.md` §8) hold for every type here; `App` (57 fields, 244 methods) and `Renderer` (48, 153) are the remaining budgeted debt in `internal/arch/solid_budget.txt` — only lower it.

## Hot paths
Colony `Draw`: `render/world.Pipeline` over the layers, culled to the visible tiles with the chunk tiers; no full-map loop per frame; no new allocation per frame; no interface value in a per-tile or per-object loop; no pointer to a frame local through an interface method (SOLID_GO §7). `Renderer.advanceTicks` runs once per Update on the same read models: only creatures near the view keep a motion track (`render/motion`); memos keyed on read-model identity apply only to a simulation with the view cache.

Benchmarks: `BenchmarkDrawArt500` / `BenchmarkDrawArt250` (with a draw-call count), `BenchmarkChunkLayer500`, `BenchmarkMinimapLayer500` in this package; `BenchmarkGlobeFrame` in `globe`, `BenchmarkLightGridLamps` in `render/light`. A renderer change is gated by a back-to-back A/B of `DrawArt500/z1` against a worktree of the previous commit (ns/op within noise, draw-calls/op identical, allocs/op unchanged) — five-run averages are noise (ARCH-004). `make fps` is the real frame-rate probe (PERF-002).

## Tests
- Root tests are the integration tests: `nav_test.go`, `app_test.go` (routing, Start creating a sim, pause blocking `Step`), `hud_test.go`, the `screen_*_test.go`, and the player journeys (QA-001, `journey_*_test.go`): every player-facing change ships a `TestJourney<Area><Name>` that drives the real loop through `newJourney` (fake keys and mouse into `App.Update`, frames through `App.Draw`, a pinned clock) and asserts the command that left the client or the read model after it. `click(t, a, id)` presses an element with the real mouse through three Updates.
- `TestEveryInputActionHasAJourney` fails on an input action no journey presses or a command kind no journey asserts (it walks the sub-packages too), unless owed in `journeyActionTODO` / `journeyKindTODO`; those lists only shrink.
- Unit tests on hand-built values live in each sub-package with its `fakeShell`. The `*_compat_test.go` files hold the helpers that let root tests reach into a feature; a new test calls the feature directly instead of adding one.
- Tests must not open a window; `make test-journeys` runs the journeys.
- Raid pause in journeys: after `j.g.RaidArmed(...)`, call `j.update(1)` before pressing a speed key, because pause-on-threat (`threat_ui.go`) pauses the game on that update and undoes an earlier speed key. If a combat journey shows no shots or bursts, check the tick is advancing before debugging the sim (wave C, 2026-10-29, lost a cycle to this).

## Notes
- Time budget: the Update time budget may carry ticks to the next Update at 3x and faster. It moves ticks between Updates, never changes their order, and captures are exempt. Speed tiers only change how often `App` calls `advance`.
- Decomposition recipe (ARCH-001…006, `.agents/handoffs/ARCH.md`): the lead lays the stubs (`deps.go` with `Shell`/`Host`, root `<pkg>_compat.go` with the host adapter and constructor, the App pointer) and pre-migrates fields other files read; agents move files with disjoint ownership.
- Agents never edit lead-owned files; they leave `lead_edits_<pkg>.pl` scripts the lead applies when they report (an exported-field rename breaks every other agent's build until then; apply perl only to files that match, never `*.go`).
- Mechanical sweeps use `go/types` tools, not regexes: pass-through wrappers map by AST shape, alias uses are rewritten by exact `file:line:col`; "tests-only" is not dead (`NewApp`, `Update`, `Draw` are called by `internal/compose/game`); interface-implementing methods show 0 direct uses.
- Never pass a pointer to a frame local through an interface method in a draw path: it escapes and costs +2 allocs per frame (found by `-benchmem` and `-gcflags=-m=2`). The gate is a back-to-back `DrawArt500/z1` A/B against a worktree of HEAD, not averages.
- Where the presentation knowledge went (these used to be sections of this file; each now lives with its owner's AGENTS.md and doc.go):
  - Original art, placement, the draw order, linked walls/fences/rock, stone tints, the art catalog's memory and prewarm, `TestGeneratedArtCoversRenderer` → `art/`, `render/sprite`, `render/world`.
  - Lighting: lamps and campfires from one helper, the lamp vertex packing, the ground bake levels, weather dimming daylight → `render/light`, `render/env`.
  - Motion and body animation (ART-007/025/027): tracks, clocks (`AnimT` vs `Present`), stride-driven bob, animal gaits, held items, work particles and progress → `render/motion`, `render/creature`.
  - Overlays, power and zones drawing, fire → `render/overlay`; the Architect's tools, draw styles, zones and areas → `architect`; zone commands stay in `cmds_zone.go` until they join `architect.Zones`.
  - Sound cues from presentation signals → `render/cue`; the director that pans, culls and picks music stays here (`sound_director.go`).
  - Chunk LOD and the minimap → `render/world`, `render/minimap`.
  - The widget kit and text → `uikit`, `gfx`; the wizard draft and the globe → `wizard`, `globe`.
