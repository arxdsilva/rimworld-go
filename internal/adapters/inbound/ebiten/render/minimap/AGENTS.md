# render/minimap package

## Package Purpose
The colony minimap: the baked map layer the simulation hands out by version, its box on screen, the viewport frame and the clicks that pan the camera. It reads the frame's `*render.View`, never the renderer (SOLID_GO.md §7). ARCH-004 wave 4.3 (agent R8).

## Owned Concepts
- `Map` (8 methods, 6 exported) built by `New() *Map`: `On`, `Toggle`, `Box`, `Click` (returns the camera offsets), `View`, `Draw`.
- `Source` (5 methods): `Width`, `Height`, `World`, `Snapshot`, `MinimapLayer(ver uint64) (game.MinimapView, bool)`.

## Allowed Dependencies
- Leaves: `camera`, `gfx`, `hudlayout`, `uikit`.
- Layer: `render` (the view).
- `application/game` read models (`MinimapView`, `WorldView`, `PawnView`).
- Ebitengine (`ebiten`); stdlib `image/color`.

## Forbidden Dependencies
- The ebiten root package and `cmd/`.
- Feature packages (the minimap is drawn by the root's HUD wiring; this package imports none of them).
- Other `render/*` layers: minimap is a leaf layer.

## Public Contracts
- `Click` returns camera offsets; the caller applies them to the camera. The package never moves the camera itself.
- `Draw` takes the destination image; the baked layer is requested by version through `Source.MinimapLayer`.

## Invariants
- The baked map is rebuilt only when `MinimapLayer(ver)` returns a new version.
- Clicks centre the camera on the clicked tile (`TestClickReturnsCentringOffsets`); the minimap toggles and colours as specified (`TestOnToggleAndColors`).

## Hot Paths
- `Draw` runs per frame; it blits the cached layer and the viewport frame. The bake is version-keyed, not per frame. Not benchmarked here; the root's `MinimapLayer500` is the gate.

## Determinism Rules
- Presentation only. The layer is the simulation's output for a version; the minimap adds no randomness.

## Save/Mod Impact
- None.

## Tests
- 1 source file, 1 test file, 2 `Test*` functions.
- Coverage 29.3 % (ARCH-004).

## Benchmarks
- None in the package. `MinimapLayer500` in the root is the gate.

## Known Design Constraints
- Smallest render layer (271 lines).
