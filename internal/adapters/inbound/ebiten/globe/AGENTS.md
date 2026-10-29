# globe package

## Package Purpose
The New Colony wizard's planet view: the baked sphere, starfield, view modes and hit testing. It is built on `globemath`, `gfx` and `art`, and never changes the planet it reads. Moved out of the root in ARCH-003 wave 3.1 (`globe/`, 1,200 lines). Used by the `wizard` and `windows` (World tab) packages.

## Owned Concepts
- `View` (38 methods, 20 exported) built by `New(p *planet.Planet, cat *art.Catalog) *View`; holds the baked sphere, starfield and the selected site (`Selected()`, `SetHovered`).
- `Input` (the frame's pointer and drag input) and `Mode` (view modes; `ModeNone` is -1).
- Palettes and ramps: `WorldPalette`, `TempRamp` (`ColourStop`), `HillMix`.
- Constants `DoubleFrames` and the mode set.

## Allowed Dependencies
- Leaves: `art`, `gfx`, `globemath`, `inputframe`.
- Domain value types: `faction`, `world/biome`, `world/planet`, `world/site`. Read-only.
- Ebitengine (`ebiten`, `vector`); stdlib (`encoding/hex`, `fmt`, `image`, `image/color`, `math`, `strings`, `sync`).

## Forbidden Dependencies
- Simulation and application packages (`application/game`, `sim/*`). The view takes the planet as a value it reads.
- The ebiten root package and `cmd/`.
- Feature packages (`wizard` and `windows` import globe; globe imports neither).

## Public Contracts
- `New(p, cat)`; `SetHovered`, `Selected()`. The `selected` field is private behind its accessor.
- `Draw` and `DrawBody` paint the sphere from the baked tiles; `drawMarks` paints the site marks.

## Invariants
- The view never mutates `*planet.Planet` (package rule, in the doc comment).
- Colours come from `WorldPalette` and `TempRamp`; `TestGlobePaletteMatchesYAML` checks the palette against the YAML source.
- `View` has 51 fields (budget `globe.View 51` in `solid_budget.txt`): the same debt as the root's `globeView`, moved with the type.

## Hot Paths
- `Draw` runs per frame over the baked sphere. `BenchmarkGlobeFrame` measures the frame (ARCH-004 gate reads `GlobeFrame` ns/op and allocs/op).
- The bake and tile recolouring run on mode change or new planet, not per frame.

## Determinism Rules
- Presentation only. Nothing here feeds the planet, the site choice or the simulation's RNG.
- Hit testing is pure geometry (`globemath`).

## Save/Mod Impact
- None. The package reads no files at runtime (palette data comes from the embedded or art catalog).

## Tests
- 1 source file, 1 test file, 28 `Test*` functions (`TestGlobePaletteMatchesYAML`, `TestGlobeCornersAreSharedAcrossTiles`, `TestGlobeColoursByWaterThenBiome`, `TestGlobeModesRecolourOnly`).
- Coverage 95.6 % (ARCH-003), the highest in the tree.

## Benchmarks
- `BenchmarkGlobeFrame`.

## Known Design Constraints
- `View` carries 51 fields and is budgeted; the next split is by responsibility (sphere bake, marks, input).
- The root's `App.ng` stays on App because `world_ui.go` reads the planet (ARCH-005 wave 5.2 note).
