# devtools package

## Package Purpose
Developer mode: the dev panel's spawns, cheats and toggles, the map-field overlays over the world, and the F-key debug actions. `Panel` reaches the simulation through `Shell` and its `Host`. Feature package (ARCH-005 wave 5.3, agent A12).

## Owned Concepts
- `Panel` (20 methods, 10 exported) built by `New(sh Shell) *Panel`.
- `State` (dev panel state; holds `Run func(d Tools) string` per action), `Action`, `Env`, `Selected`, `PanelRects`, `FieldsOverlay`.
- Interfaces: `Host` (6 methods: `Env`, `HUD`, `Toast`, `Selected`, `StepTick`, `Icon`); `Tools` (8 methods: `SpawnAnimal`, `KillAnimal`, `FireIncident`, `Raid`, `SpawnStack`, `SetNeeds`, `DevLosePart`, `DevSpawnImplants`); `Keys` (`Fired`); `FieldSource` (`MapFieldLayers`).
- Helpers: `FieldPixels`, `FieldLegendLines`, `Column`, `LoseParts`; constants `FieldAlpha`, `LosePickID`.
- `var _ Tools = (*game.Game)(nil)` in `state.go` is the compile check that the root's game satisfies `Tools`.

## Allowed Dependencies
- Leaves: `gfx`, `hudlayout`, `ports`, `uikit`.
- Application read models and commands: `application/game`, `sim/command`, `sim/clock`, `sim/entity`.
- Domain value types: `economy/item`, `pawn/body`; `platform/config`, `platform/input`.
- Ebitengine (`ebiten`); stdlib (`fmt`, `strconv`, `strings`).

## Forbidden Dependencies
- The ebiten root package and `cmd/`.
- Other feature packages (`hud`, `architect`, `inspector`, `windows`, `orders`, `dialogs`, `learn`, `perf`, `options`, `wizard`, `mainmenu`, `selection`).
- `render/*` must not import devtools.

## Public Contracts
- `New(sh Shell) *Panel`; `Shell` = `Sizer + Commands + World + View + Speed + Widgets + Settings + Host`.
- Dev actions are applied through `Tools`, which the root wires to `*game.Game`. This path is not `ports.Commands` only: the cheats call game methods directly. The handoff does not say whether that is intended; treat new cheats the same way until decided.

## Invariants
- Panels are off unless dev mode is on (`TestPanelsOffWithoutDevMode`).
- Map-field overlay colours and legend lines are pure functions of the layer (`TestMapFieldPixels`, `TestMapFieldLegendListsClasses`).

## Hot Paths
- `FieldPixels` is a pure function of the map-field layer.
- Panel widgets are declared once per `Update`; draws per frame. No allocation claim: not benchmarked.

## Determinism Rules
- Presentation and developer tooling only; never reaches the simulation's RNG or wall clock.
- Spawns and cheats change simulation state through `Tools`; they are developer-only and must not run in a normal session.

## Save/Mod Impact
- None.

## Tests
- 8 source files, 2 test files, 9 `Test*` functions (`TestMapFieldPixels`, `TestMapFieldLegendListsClasses`, `TestDevColumnAvoidsEveryRegion`, `TestPanelsOffWithoutDevMode`).
- Coverage 76 % (ARCH-005 row).

## Benchmarks
- None.

## Known Design Constraints
- `Panel` has 20 methods, not the 12 the ARCH-005 row lists; the row undercounts.
- The F10 screenshot and the frame-rate probe belong to `perf`, not here.
