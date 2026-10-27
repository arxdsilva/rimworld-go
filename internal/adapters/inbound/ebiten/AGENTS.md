# ebiten inbound adapter

Interactive Ebitengine client: screen routing, programmatic UI chrome,
and the colony map viewport. Presentation-only. It never mutates
authoritative simulation stores.

## Purpose

Host the player-facing session: boot, menus, the new-game wizard (scenario,
scenario editor, storyteller and difficulty, world, landing site on a globe,
characters, loading), pause, and the colony viewport with HUD chrome. Screens consume read models
(`Snapshot()`, `World()`) and emit session intents plus typed gameplay
commands via `Enqueue`. Domain code does not import this package.

## Owned concepts

- `App` — `ebiten.Game` shell, screen stack, overlay (pause), HUD
- widgets: panel, label, button (hover/disabled), and the immediate-mode widget kit (see below)
- the real font (`font.go`, Noto Sans embedded from `fontdata/`)
- the new-game wizard screens (`screen_*.go`, draft in `newgame_state.go`), the setup backdrop and the globe (`globe.go`, `globemath/`)
- big-map rendering: chunk read models, zoom tiers (`lod.go`, `chunk_cache.go`) and the minimap (`minimap.go`)
- UI chrome tokens (`assets/palettes/ui.yaml`, mirrored in Go)
- colony HUD: Food/Wood/time strip, colonist bar, Architect, Work, inspector
- main-menu colonist splash (original PNG, left of the action rail)
- main-menu colony diorama (`menu_diorama.go`: drift, glow, flame, scrim)
- Options screen (`screen_options.go`: tabbed rows from one table) and the
  player's settings (`settings.go`: load, apply, save through `SettingsHooks`)
- sound director (`sound_director.go`) and the renderer's sound cues
  (`sound_signals.go`) behind the consumer-owned `Sound` port (`sound.go`)
- `Renderer` — colony camera, speed tiers, selection, world draw, `Enqueue`
- original art catalog (`assets/metadata` + generated PNGs, tint masks) and
  its draw path (`art.go`, `draw_art.go`); plain coloured fallbacks without art
- dev screenshot capture (`capture.go`, `game -capture`)
- presentation animation / tile lerp clocks

## Allowed dependencies

- `internal/application/game` via the consumer-owned `Simulation`
  interface (`Step`, `Snapshot`, `World`, `Enqueue`, `Width`, `Height`,
  `Tick`, `Date`, `Activity`)
- `internal/domain` types that appear on that read model (`clock`,
  `entity`, `schedule`, `command`)
- NG-1 setup types the wizard draft and read models carry: `domain/world/planet` and `site` (a finished, read-only planet), `scenario`, `difficulty`, `storyteller`, `faction`, `pawn/pawngen`, `pawn/look`; the wizard builds a `game.Setup` from them and never reaches into game internals
- Ebitengine, image/png, OS path lookup for generated art and the menu splash
- `internal/platform/config` (the settings type)
- `internal/platform/input` (the key-binding registry and resolver)

## Forbidden dependencies

- concrete `*game.Game` pointers except at `cmd/game` (composition root)
  and the compile-time `var _ Simulation = (*game.Game)(nil)` assertion
- mutating domain stores
- wall-clock or global RNG in anything that could leak into sim
- proprietary RimWorld art, text, or layouts
- `ebiten/v2/audio` or any audio library: sound goes through `Sound`
  (cmd/game wires `internal/adapters/outbound/ebitenaudio`), so tests and
  captures never open an audio device

## Contracts

- Simulation is created only when the player presses Start (or a capture
  starts one). Menus must run with `renderer == nil`.
- The wizard turns its draft into a `game.Setup` and builds the colony step
  by step behind a progress screen through the consumer-owned `SimBuilder`;
  `cmd/game` wraps `game.NewColonyBuilder` as the `NewSimBuilder` hook. The
  one-shot `SimFactory` (`game.NewColony`, the legacy start) serves
  `-scenario` set pieces, captures and tests. No stub rosters or fixed kits
  live in this package.
- Return to main menu drops the renderer so the menu stays headless.
- Pause overlay skips `Renderer.advance` and map input.
- HUD, inspector, Architect, and Work consume pointer hits before map selection.
- Escape on colony: hidden UI, order/debug menu, formation drag, Move mode,
  Architect tool, Architect, Work, building/frame/group/pawn selection, then pause.
- Disabled buttons do not navigate; they expose a reason string.
- `Layout` follows the window size; camera/zoom stay presentation-only.
- Architect (`architect.go`, `archtools.go`): categories Orders, Zones,
  Structure, Furniture, Production, Power (PLAY-021), Security and Misc
  (nine would overflow the panel: heater and cooler sit in Misc); cards read costs from `game.Buildables()`; build
  tools emit `DesignateBuild` (kind id + facing) only for cells where
  `Simulation.CanPlace` is ok; area tools emit one rectangle command
  (`DesignateStockpile/Harvest/Deconstruct/Cancel`). Q/E rotate single
  placements and change the draw style of drag tools (`drawstyle.go`:
  line, filled/empty rectangle, filled/empty oval; Ctrl squares; oval
  area tools send one rectangle per row run); right-click or Escape leaves
  the tool and the right-click is consumed.
- Work cells emit `SetWorkPriority` (off, 1..4, cycling per click; `SetWorkEnabled` is the legacy toggle).
- Pawn orders (`orders.go`): Draft button / R emits `DraftPawn`. Right-click
  with a pawn selected: drafted -> `MovePawn`; undrafted -> menu built from
  `Simulation.OrdersAt` (each entry carries its command or a disabled
  reason). Escape closes the menu first.
- The `Simulation` interface also exposes read-only `CanPlace` and
  `OrdersAt`; the adapter never re-implements placement or order rules.

## Input (UI-016)

- Only `input_source.go` reads Ebitengine's keyboard and mouse. `App.Update`
  polls once (`pollInput`) into `inputFrame` and resolves actions with
  `input.Resolve` against the context stack (`contextStack`: text > modal >
  tool > panel > map > global > dev). A pressed key fires at most one
  action; a used mouse press is `take()`n so later handlers skip it.
- Handlers read `a.in.fired(action)`, `held`, `clicked`, `down`,
  `released`, the raw `key(name)` (text owners only) and the modifiers.
  The renderer receives intents (`togglePause`, `setSpeed`, `stepSpeed`,
  `applyCamera`), never keys.
- Bindings live in `config.Settings.Controls` (overrides only) and are
  edited on Options › Controls (`controls_ui.go`). Labels and tooltips
  use `keyHint`, so they always show the live key.
- Developer mode (Options › Gameplay) adds the Dev context; its debug
  actions use the consumer-owned `devTools` interface (`devmode.go`).
- Tests drive `App.Update` with `fakeInput` (`input_ui_test.go`).

## UI scale (PLAY-012, UI-024)

- The UI lays out in logical pixels (`a.sw/a.sh`, `currentCursor()`) and
  draws through `ui_canvas.go` (`uiFillRect`, `uiStrokeRect`, `uiLine`,
  `uiImage`, `uiTriangles`, …) and `drawText`; the scale is `uiScale()`.
- Anything placed by the camera stays raw in physical pixels: use
  `mapCursor()`, `a.mapViewport()`, `r.cam.tile()`. Convert a world anchor
  for a UI element with `toLogical`.
- `TestUIDrawsThroughCanvas` rejects raw drawing outside its exemption list
  (each entry has a reason).
- World detail (PERF-002, `world_scale.go`): the camera's pixels are map
  pixels (physical × render scale). Get them from `mapCursor()`,
  `a.mapViewport()`, `a.mapScale()` (logical → map) or `mapUIScale()` (map →
  logical, also in Update), never `a.pw`/`uiScale()` outside the world pass.
  Map overlays drawn by tools and selection go in `drawMapOverlays`; their
  canvas draws are queued and drawn native (`TestMapPixelsComeFromMapCursor`).

## Widget kit and text (NG-1)

- Immediate mode (`ui_kit.go`, `ui_state.go`). A screen declares its widgets once
  per `Update` from a hook it registers in an `init()` with
  `registerScreenUI`; the hook runs right after input is polled, before the
  click path, and only while the screen is on top with no overlay. Widgets
  read the input, mutate the value they are bound to, register a hit
  rectangle and queue their drawing; `App.Draw` calls `uiPaint` for the queue,
  the modal stack and the tooltip. State is `a.ui` (`uiSt()`).
- Widgets: `ui_check.go` (checkbox, radio), `ui_slider.go`, `ui_textfield.go`
  (single and multi-line; `CtxText` stops global shortcuts, Esc restores),
  `ui_scroll.go` (virtualised, wheel routed away from the camera),
  `ui_tooltip.go`, `ui_modal.go` (stack, `CtxModal`), `ui_table.go`,
  `ui_tabs.go`, `ui_progress.go`, `ui_spark.go`, `ui_layout.go` (`rect`, the
  setup dialog frame, the 1280x720 minimum). Ids carry a per-screen prefix
  (`scen.`, `sced.`, `story.`, `diff.`, `world.`, `land.`, `pawns.`, `fac.`,
  `modal.`). Esc order: text edit, top modal, Back.
- Text goes through `font.go`: `textStyle` (`tsSmall` 11, `tsBody` 13,
  `tsLabel` 13 bold, `tsLarge` 16 bold, `tsTitle` 22 bold, `tsHuge` 32 bold),
  `drawText`, `measure`, `lineH`, `wrapPx`, `ellipsize`, `setUIScale`.
  `drawLabel` and `debugPrintAt` forward to it. Never assume a glyph width or
  line height: measure, wrap or ellipsize.
- Wizard draft state lives in `newgame_state.go` and survives Back/Next; it
  becomes a `game.Setup` only on Start. Map sizes come from `game.MapSizes()`
  (gated by `game.MaxEnabledSide`).
- The globe (`globe.go`) is one triangle batch over a finished
  `*planet.Planet`; it never changes the planet. The setup backdrop and the
  globe freeze under Reduced Motion.
- Chunk LOD (`lod.go`): tile size 48 px or more draws live, 16 to 48 px draws
  baked 16 px chunk images (`chunk_cache.go`, LRU, rebaked when
  `game.ChunkView.Ver` moves), under 16 px draws one 4 px whole-map bake.
  Bakes are unlit and lit per tile at draw time. The renderer reads
  `ChunkLayer`, `PlantsIn` and `MinimapLayer` from the `Simulation` interface
  and never loops over the whole map per frame.
- Minimap (`minimap.go`): maps over 150 tiles, Shift+M, click recentres.
- Commitment colonies (`saves.go`): one save slot written by the game itself;
  pause shows Save & Quit, Load and developer mode are disabled.
- Captures: `-capture-ui` states for the wizard live in `capture.go`
  (`menuCaptures`, scripted clicks) and for colony UI in `capture_colony.go`.

## Invariants

- Screen IDs and widget bounds never enter domain or save state.
- Sprite animation uses a presentation frame counter, not sim ticks, to
  pick frames. Tile lerp may use sim ticks vs `moveTick`.
- UI hit-testing consumes the pointer before the map.
- Menu chrome colors must match `assets/palettes/ui.yaml`.
- Main-menu splash is presentation-only; a missing PNG still shows the
  backdrop and right-rail buttons.
- The menu diorama is optional (`AppConfig.LoadMenuScene`); without it the
  gradient backdrop is drawn. Its motion runs on `menuTick`, never the sim,
  and `-capture-ui menu` shoots it without starting a game.

## Hot paths

Colony `Draw` (terrain, world overlays, pawns, daylight). Keep off-screen culling
and the chunk tiers; no full-map loops per frame. Menu `Draw` is cheap vector
fills plus real-font text; the globe is 0.3-0.5 ms per frame.

`Renderer.advance` runs once per update on the same read models (ENG-F):
- Only animals within `motionViewPad` tiles of the last frame's view keep a
  motion track (`syncAnimals`); one further away has none (`motionOf` is not
  ok, it is drawn on its cell) and starts afresh when it nears the view.
  Plant contacts, step dust, shadow casters, stack counts and trace ends cull
  by cell before sampling motion. Arc lengths are memoised per track step.
- Memos keyed on read-model identity (`Renderer.split`, `App.alerts`) apply
  only to a simulation with the view cache (`viewCacher`); a test double
  without it is re-read every call.
- Loop over `[]AnimalView` / `[]BuildingView` by index: the structs are large.

## Determinism

This adapter is not in the authoritative tick. `Step(n)` still runs on
the simulation the factory created; speed tiers only change how often
`App` calls `advance`. At 3x and faster the Update time budget
(`update_budget.go`) may carry ticks to the next Update: it moves ticks
between Updates, never changes their order, and captures are exempt.

## Tests

`nav_test.go` and `app_test.go` cover routing, disabled clicks, Start
creating a sim, and pause blocking `Step`. `ui_kit_test.go` covers the widget kit and each `screen_*_test.go` its screen. `hud_test.go` covers colonist
bar select, Work overlay, Architect palette, Escape order, Work cell
toggles, inspector names, and HUD hit-testing. `splash_test.go`
covers the right-rail menu and PNG decode. `widgets_test.go`
checks UI palette hexes against `assets/palettes/ui.yaml`. They must not
open a window. Existing renderer unit tests stay headless.

Player journeys (QA-001, `journey_*_test.go`): every player-facing change
ships a `TestJourney<Area><Name>` that drives the real loop through
`newJourney` (fake keys and mouse into `App.Update`, frames through
`App.Draw`, a pinned clock) and asserts the command that left the client or
the read model after it. `applyClick` and direct handler or renderer calls
do not count (`TestJourneysUseTheRealInputPath` rejects them in journey
files). `click(t, a, id)` (app_test.go) presses the element with the real
mouse through three Updates — it is the right helper for any other test
that clicks a HUD button or widget — but it is not a journey either;
`clickDirect` runs the dispatcher alone and is reserved for a test that
must not run an Update (say why at the call). `TestEveryInputActionHasAJourney` fails on an input
action no journey presses or a command kind no journey asserts, unless it is
owed in `journeyActionTODO` / `journeyKindTODO` with the file that will
cover it; those lists only shrink. `make test-journeys` runs the suite;
CI (`.github/workflows/test.yml`) runs everything on every push.

## Benchmarks

`BenchmarkDrawArt500` / `BenchmarkDrawArt250` (`draw_bench_test.go`, with a draw-call count), `BenchmarkGlobeFrame`, `BenchmarkChunkLayer500` and `BenchmarkMinimapLayer500`. `-fpsprobe N` on `cmd/game` logs real frame stats.

## Original art (ART-006)

- When `assets/metadata` exists the renderer draws generated art only
  (`drawArt`); if art loading fails it draws plain coloured terrain and
  pawn squares (the vendor pack was removed in PLAY-010).
- Placement: sprite top-left = footprint top-left - `origin_px` * (tile/128).
- Draw order: terrain -> grass decals -> floors -> zones/blueprint -> objects
  and pawns sorted by row then column -> designations -> selection -> daylight.
- Linked rendering (ART-015, `linkgrid.go`): walls, doors, fences and rock
  link through flags/accepts (walls and rock join; fences join walls one
  way; rock links off the map edge). The grid is cached on
  `WorldView.StructVer` (one per Renderer, so per session); frames and
  ghosts use a per-frame planned set. Keys are `<prefix>_<NN>` (N=1 E=2
  S=4 W=8) or `<prefix>_<NN>_c<h>` with filled corners (NE=1 SE=2 SW=4
  NW=8), 47 per family; missing art falls back to `_NN`, then to a
  related family (`linkFallback`). Families: `rock/rock_wall`,
  `rock/rock_smooth`, `rock/ore_<metal>`, `structure/wall_<wood|stone|steel>`.
- Stone art is tint-neutral: the stone tint (`stone_tint` palette group)
  multiplies as a colour scale, or bakes through the ore sprites' mask
  (`stone_art.go`). Rock, stone ground and edges, stone walls, fences,
  floors, chunks and blocks take their stone's tint. Rock draws and the
  terrain pass cover only the visible tiles (padded).
- Beds draw `bed_cot` (domain beds are single-cell). Frames draw scaffold
  once wood arrives.
- Pawn look (skin/hair/style/clothes) is a pure function of EntityID;
  facing comes from the last tile step. Both are presentation-only.
- Power (PLAY-021): the sim owns every number; the UI reads
  `BuildingView` power fields and `WorldView.PowerNets` and sends
  `SetPowerWanted`/`SetThermostat`. `power_draw.go` draws the buildings
  (conduit autotile on its own pass, glow layers only while running,
  turbine phases on `Renderer.power`), `power_overlay.go` the Shift+P grid
  overlay (`Renderer.pov`, forced while a power tool is in hand),
  `cmds_power.go` the pane lines and Power toggle (V), `alerts_power.go`
  brownout and not-connected alerts. Light sources gate on `Glow` only.
  A hidden conduit is selectable only while the overlay shows.
- Light sources (PLAY-020, `light_sources.go`): every lamp and burning
  campfire the renderer lights or shadows with comes from one helper; never
  test `Kind == "lamp"` for lighting elsewhere. A lamp's light falls off
  over its metadata `light_radius`, stays in its room or, outdoors, in its
  line of sight, and its direction is packed per tile into the lamp vertex
  value (`light_lamp.go`), so batch builders pass it through unchanged.
  Never interpolate packed values between vertices: the ground bake
  (`drawBaked`) writes plain levels per corner (`cornerRow`, the mean
  of the open tiles around it) and merges a run only when all its corners
  match; live ground planes (terrain tiles and edges, floors) draw through
  `drawLitGround`, which writes the same field per vertex
  (`lightGrid.groundLevel`); a sprite passes its foot's world position to
  `drawLit`, which lights it by continuous distance to the dominant lamp
  (`lightGrid.atF`).
- Weather dims daylight by `weather.SunFactor` (`weatherSun`), the factor
  solar output uses; don't keep a renderer-only table.
- Zones (PLAY-020): a zone is the last selection candidate on its cell, and
  its label selects it (`zone_select.go`); its pane and commands are in
  `cmds_zone.go`, the Expand and Shrink tools in `zone_tools.go` (state on
  `App.zone`). A growing zone's fill skips the cells holding its plants.
- Art memory (PLAY-019, `art_memory.go`): the game indexes the art and
  decodes a sprite on first use; a background prewarm keeps deflated copies
  of every file family except animals. CPU pixels are freed after the GPU
  upload, except for crop units, tilled overlays and grass tufts, whose
  bakes read them back: a new CPU reader of sprite pixels must add its keys
  to `cpuReadKeys`. Tinted textures are baked on the GPU (`tint.kage`) per
  (sprite, tint) in a 128 MiB LRU.
- `TestGeneratedArtCoversRenderer` fails if the renderer asks for a sprite
  that `make art` does not produce.
- `game -capture out.png` boots, starts a colony, enqueues a fixed set of
  ordinary commands, steps the sim and writes one frame (`make art-capture`).

## Motion and body animation (ART-007, ART §27.1)

- `motion.go` owns on-screen movement: `pawnMotion`/`animalMotion` return a
  `moveState` (position, velocity, speed, stride, walk blend, facing) built
  from the snapshot's `StepTick`, `StepTicks`, `NextX/NextY`, `HasNext` and
  `BlockedBy`. Paths curve through tile centres; walks ease in and out;
  facing uses hysteresis. `interpWorld`/`interpAnimal` delegate to it.
- Clocks: `animT()` (sim ticks plus the elapsed fraction of the frame; freezes
  when paused, scales with speed) drives movement, walk cycles, work swings
  and combat reactions. `present` drives breathing, wind, fire, particles and
  UI pulses. Captures use no sub-frame interpolation, so they stay deterministic.
- Pawns have no frames (ART-025): walk bob, sway and lean follow stride (one
  cycle per 1.6 tiles); per-action lunges, recoil, lean and dip run on
  `animT`; a fall is a buckle then a rotation. Strikes land at trace age 0.
  Animal trot `walk01..walk08` advances with stride. Hair and hats follow the
  body's `head` attachment. Held items (`held.go`, ART-027, user decision:
  RimWorld-faithful) are placed by their centre from the body centre (head
  x, halfway head to feet): a weapon only on duty (`onDuty`), carried low
  across the body at rest, aimed out towards the target when aiming or
  striking (flipped upright when aiming left, behind the body only aiming
  north); a hauled item on the chest (behind facing north). No tool is
  ever drawn: work is the lean/dip toward the job, particles at the work
  spot (`workSpot`) and the in-world progress bar (`work_progress.go`,
  overlay pass, tile >= 24 px, one per work tile). `hold` remains only as
  the punch trail's point; `back` carries rescued patients. West is east
  mirrored (`facingKey`, also carried items, face scar, shadows, animals);
  the lit shader inverts the normal X for mirrored draws.
  `figureLayers` serves the colony, portraits and the colonist bar.
  `projShearK` (`light.go`) must match the metadata shear (0.35).
- `camera_smooth.go`: eased pan and cursor-anchored zoom; capture writes the
  camera directly.
- The colonist inspector shows "waiting for <name>" while `BlockedBy` is set.
- Settings are presentation-only (window, particles, camera, autosave,
  sound) and never reach the simulation. `-capture` runs get no
  `SettingsHooks` and no `Sound`, so captures are deterministic.
- Sound cues come from presentation signals only: work strikes
  (`workStrikeAt`, silent when paused), world diffs (frames, plants,
  mines, door occupancy), traces, lightning, threats and clicks. The
  director pans by screen position, culls beyond half a view off screen,
  fades effects with camera distance to silence fully zoomed out (`zoomEar`),
  limits repeats, and picks music (menu, calm, tension) and ambience
  (day/night, rain, campfires) from `WorldView`.

