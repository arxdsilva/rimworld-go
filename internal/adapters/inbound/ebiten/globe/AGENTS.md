# globe package

## Package Purpose
The New Colony wizard's planet view: the baked sphere, starfield, view modes and hit testing. It is built on `globemath`, `gfx` and `art`, and never changes the planet it reads. Moved out of the root in ARCH-003 wave 3.1 (`globe/`, 1,200 lines). Used by the `wizard` and `windows` (World tab) packages.

## Owned Concepts
- `View` (38 methods) built by `New(p *planet.Planet, cat *art.Catalog) *View`; holds the baked sphere, starfield and the selected site (`Selected()`, `SetHovered`), the grid toggle (`SetGrid`/`Grid`), `Bare()` (tiles only, for the backdrop disc) and `Surface(tile)` (a marker point on the relief).
- `planetGeom` (`relief.go`, WG plan §1.2): the once-per-planet relief. Deduplicated corners and their sharing tiles, corner and centre heights (corners: mean land elevation plus 0.8 of the mean of the two largest hill lifts, so mountains join into ridges; ocean corners at sea level), relief positions (`ReliefMax`), shading normals (hill lift exaggerated ×3 for the light only, facet weight by hilliness), the river/road/coast/grid lines and the per-vertex colours of the mode (soft borders within a water class, rock and snow by height above the sea-level snowline).
- `labels.go` (WG plan §1.2 part 2): rivers by size (`riverWidth`, tapered at a source, ending at the shore), roads by class (`roadStyles` over `planet.RoadClassOf`), feature and settlement names (`Label`, `View.Labels()`; laid out each Build: settlements then features largest first, zoom band, facing > 0.55, rect collision, at most 24; drawn in `drawMarks`), and the legend swatches `RoadLegend`, `RiverLegend`, `HillLegend`.
- `xform`, `cornerCache`, `ovMesh` (`frame.go`): the frame's camera and projection (embedded: `WorldPt`, `Zh`, `CamDir`), the per-corner screen cache with the far-to-near bucket sort, the overlay batch (embedded: `Ov`, `Oi`).
- `Input` (the frame's pointer and drag input) and `Mode` (view modes; `ModeNone` is -1).
- Palettes and ramps: `WorldPalette`, `TempRamp` (`ColourStop`), `HillKey` (Hilliness-mode colours).
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
- `View` has 34 fields (budget `globe.View 34` in `solid_budget.txt`; was 51 before WG-1 moved the planet data into `planetGeom` and the frame into `xform`/`cornerCache`/`ovMesh`). Grow per-planet data in `planetGeom`, never on `View`.
- Shared corners are indexed, so every tile places a corner at the bit-identical point (`TestGlobeSharedCornersIdentical`); water stays at radius 1 and nothing exceeds `ReliefMax` (`TestGlobeReliefPeaksAboveCorners`, the silhouette test).
- No depth buffer: tiles draw far to near and, inside a tile, back faces first; the index list is the identity, so vertex order is draw order.

## Hot Paths
- `Draw` runs per frame over the baked sphere. `BenchmarkGlobeFrame` measures the frame (ARCH-004 gate reads `GlobeFrame` ns/op and allocs/op; WG gate ≤ 2 ms at Dist 1.5, 0 allocs; about 0.8 ms after WG-1 part 2 on an M2 Max). Faceted: 3 vertices per fan face, up to 18 per tile; the sun turns into planet space once per frame.
- The bake and tile recolouring run on mode change or new planet, not per frame.

## Determinism Rules
- Presentation only. Nothing here feeds the planet, the site choice or the simulation's RNG.
- Hit testing is pure geometry (`globemath`).

## Save/Mod Impact
- None. The package reads no files at runtime (palette data comes from the embedded or art catalog).

## Tests
- 4 source files, 1 test file, 36 `Test*` functions (`TestGlobePaletteMatchesYAML`, `TestGlobeCornersAreSharedAcrossTiles`, `TestGlobeColoursByWaterThenBiome`, `TestGlobeModesRecolourOnly`, the relief tests `TestGlobeReliefPeaksAboveCorners`, `TestGlobeSunFacingSlopeBrighter`, `TestGlobeSharedCornersIdentical`, `TestGlobeSnowOnHighColdPeaks`, `TestGlobeGridAndBare`, `TestGlobeRoadsByClass`, `TestGlobeRiversTaperAndEndAtShore`, `TestGlobeLabelsByZoomFacingAndCollision`).
- Visual review: `GLOBE_CAPTURE_DIR=<dir> go test -run TestGlobeCapture ./internal/adapters/inbound/ebiten/` (CPU raster of `Verts`/`Ov`, including the mountain close-ups and the Hilliness mode).
- Coverage 95.6 % (ARCH-003), the highest in the tree.

## Benchmarks
- `BenchmarkGlobeFrame`.

## Known Design Constraints
- `View` carries 34 fields and is budgeted; the next split is by responsibility (sphere bake, marks, input).
- The root's `App.ng` stays on App because `world_ui.go` reads the planet (ARCH-005 wave 5.2 note).
