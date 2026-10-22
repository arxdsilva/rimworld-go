# ebiten inbound adapter

Interactive Ebitengine client: screen routing, programmatic UI chrome,
and the colony map viewport. Presentation-only. It never mutates
authoritative simulation stores.

## Purpose

Host the player-facing session: boot, menus, new-game stubs, pause, and
the colony viewport with HUD chrome. Screens consume read models
(`Snapshot()`, `World()`) and emit session intents plus typed gameplay
commands via `Enqueue`. Domain code does not import this package.

## Owned concepts

- `App` — `ebiten.Game` shell, screen stack, overlay (pause), HUD
- widgets: panel, label, button (hover/disabled)
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
  its draw path (`art.go`, `draw_art.go`); vendor sprite atlas as fallback
- dev screenshot capture (`capture.go`, `game -capture`)
- presentation animation / tile lerp clocks

## Allowed dependencies

- `internal/application/game` via the consumer-owned `Simulation`
  interface (`Step`, `Snapshot`, `World`, `Enqueue`, `Width`, `Height`,
  `Tick`, `Date`, `Activity`)
- `internal/domain` types that appear on that read model (`clock`,
  `entity`, `schedule`, `command`)
- Ebitengine, image/png, OS path lookup for generated art, the vendor pack and menu splash
- `internal/platform/config` (the settings type)

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

- Simulation is created only by `SimFactory` when the player presses
  Start. Menus must run with `renderer == nil`.
- `cmd/game` constructs `game.NewColony` in the factory (Ada/Bo/Cora kit).
- Return to main menu drops the renderer so the menu stays headless.
- Pause overlay skips `Renderer.advance` and map input.
- HUD, inspector, Architect, and Work consume pointer hits before map selection.
- Escape on colony: Architect, then Work, then inspector selection, then pause.
- Disabled buttons do not navigate; they expose a reason string.
- `Layout` follows the window size; camera/zoom stay presentation-only.
- Architect (`architect.go`, `archtools.go`): categories Orders/Zones/
  Structure/Furniture; cards read costs from `game.Buildables()`; build
  tools emit `DesignateBuild` (kind id + facing) only for cells where
  `Simulation.CanPlace` is ok; area tools emit one rectangle command
  (`DesignateStockpile/Harvest/Deconstruct/Cancel`). Q/E rotate,
  right-click or Escape leaves the tool.
- Work Grow/Bld/Haul cells emit `SetWorkEnabled`.
- Pawn orders (`orders.go`): Draft button / R emits `DraftPawn`. Right-click
  with a pawn selected: drafted -> `MovePawn`; undrafted -> menu built from
  `Simulation.OrdersAt` (each entry carries its command or a disabled
  reason). Escape closes the menu first.
- The `Simulation` interface also exposes read-only `CanPlace` and
  `OrdersAt`; the adapter never re-implements placement or order rules.

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

Colony `Draw` (terrain, world overlays, pawns, daylight). Keep off-screen culling.
Menu `Draw` is cheap vector fills plus debug text.

## Determinism

This adapter is not in the authoritative tick. `Step(n)` still runs on
the simulation the factory created; speed tiers only change how often
`App` calls `advance`.

## Tests

`nav_test.go` and `app_test.go` cover routing, disabled clicks, Start
creating a sim, and pause blocking `Step`. `hud_test.go` covers colonist
bar select, Work overlay, Architect palette, Escape order, Work cell
toggles, inspector names, and HUD hit-testing. `splash_test.go`
covers the right-rail menu and PNG decode. `widgets_test.go`
checks UI palette hexes against `assets/palettes/ui.yaml`. They must not
open a window. Existing renderer unit tests stay headless.

## Benchmarks

None yet. Add them when colony draw or UI layout shows up in profiles.

## Original art (ART-006)

- When `assets/metadata` exists the renderer draws generated art only
  (`drawArt`); the vendor pack is used only if art loading fails.
- Placement: sprite top-left = footprint top-left - `origin_px` * (tile/128).
- Draw order: terrain -> grass decals -> floors -> zones/blueprint -> objects
  and pawns sorted by row then column -> designations -> selection -> daylight.
- Walls pick `wall_wood_<mask>` (N=1 E=2 S=4 W=8). Beds draw `bed_cot`
  (domain beds are single-cell). Frames draw scaffold once wood arrives.
- Pawn look (skin/hair/style/clothes) is a pure function of EntityID;
  facing comes from the last tile step. Both are presentation-only.
- Tinted textures are baked once per (sprite, tint) on the CPU and cached.
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
- Frames: pawn walk `walk01..walk12` and animal trot `walk01..walk08` advance
  with stride (one cycle per 1.6 tiles); work `work1..work6` runs on `animT`.
  Hair and hats follow the body frame's `head` attachment; tools its `hand`.
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

