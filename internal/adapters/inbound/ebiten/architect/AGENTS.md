# architect package

## Package Purpose
The Architect panel: its categories and tools, material and crop pickers, the drag box and draw styles, zone and area tools, building Move mode, home-area automation, locks on tools the colony cannot use yet, and the forced overlays a held tool shows. Turns the player's drags into typed commands through `Shell`. Feature package (ARCH-005 wave 5.3, agent A6).

## Owned Concepts
- `Tool` (35 methods) with `Zones` (30), `Areas` (19), `Marks` (13), `Mover` (8), each `struct{*Tool}`.
- `Sim` (7 methods, the simulation reads a tool needs), optional capabilities `ZoneRefuser`, `AreaReader`, `BuildingMover`.
- `Host` (embeds `ports.Dispatcher`, 8 methods) and `Map` (8 methods: the renderer's ghost, ring, link and zone draws, zone labels, plants, overlays).
- Exported data: `Categories`, `Tools`, `FacingNames`, `StyleNames`, `ZoneSpecies`; pure helpers `PlacementCells`, `ShapeCells`, `BuildSprite`, `BuildSpriteStuff`, `MovableKind`, `StockLine`, `PowerLine`, `RotateFacing`.
- The two former package globals (`areaPaint`, `archLocks`) are fields on `Tool`.

## Allowed Dependencies
- Leaves: `gfx`, `hudlayout`, `ids`, `inputframe`, `ports`, `uikit`, `art`.
- Layers: `render/env`, `render/overlay`, `render/sprite` (allowed for architect).
- Read model: `selection` (the selection kinds), `application/game` views.
- Domain value types: `building/construction`, `building/power`, `sim/clock`, `sim/command`, `sim/entity`, `world/plant`.
- `platform/input`; Ebitengine (`ebiten`, `vector`); stdlib.

## Forbidden Dependencies
- The ebiten root package and `cmd/`.
- Other feature packages (`hud`, `inspector`, `windows`, `orders`, `dialogs`, `learn`, `devtools`, `perf`, `options`, `wizard`, `mainmenu`).
- `render/*` must not import architect (the reverse direction is the rule).

## Public Contracts
- `New(sh Shell)` builds the tool panel; `Shell` = `ports.Hinter + Sizer + Commands + World + View + Widgets + Host + Map`.
- Output is `command.Command` values through `ports.Commands`; no direct domain writes.
- `Sim` is a seven-method read interface (`CanPlaceAt`, `ReplacesAt`, `AutoFacingAt`, `CanZoneAt`, `Areas`, `TemperatureAt`, `Buildables`).

## Invariants
- Guard limits: `Tool` 35 and `Zones` 30 methods (≤ 40), no type over 25 fields without a note; every interface ≤ 8 methods.
- Package size: 16 source files, 4,002 lines (`go test ./internal/arch/`).
- Build keys: `sprites.go` duplicates `render/world`'s build keys until the sweep points it there.

## Hot Paths
- Declares widgets once per `Update`; draws per frame. No per-frame allocation claim is made: the package has no benchmark.
- Placement checks through `Sim` run while a tool is held and the pointer moves.

## Determinism Rules
- Presentation only; never reaches the simulation's RNG or wall clock.
- Commands go through `ports.Commands`; placement checks are read-only calls on `Sim`.

## Save/Mod Impact
- None. The package reads and writes no files.

## Tests
- 5 test files, 23 `Test*` functions, including `TestArchCategoriesHaveTools`, `TestToolCatalogIsConsistent`, `TestSnowClearToolsInZones`, `TestZoneToolsRenamedAndRoofMoved`.
- Tests use a `fakeShell` (14 methods) instead of the simulation.
- Coverage 42.7 % (ARCH-005 row).

## Benchmarks
- None in the package.

## Known Design Constraints
- Root keeps converting shims (`archTool`, `ghostCell`, `rowRun`, `archCategories`, `archTools`) that about ten root files still read; removed by the sweep (gopls rename).
- The inspector's Zone producer (`cmds_zone.go`) stays root until it joins `Zones`.
- Build sprites and key helpers duplicate `render/world`; delete the duplicate when repointed.
