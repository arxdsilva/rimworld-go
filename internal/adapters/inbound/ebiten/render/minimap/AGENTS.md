# internal/adapters/inbound/ebiten/render/minimap

## Purpose
The colony minimap: the baked map layer the simulation hands out by version, its box on screen, the viewport frame and the clicks that pan the camera. It reads the frame's `*render.View`, never the renderer (SOLID_GO §7). ARCH-004 wave 4.3 (agent R8).

## Owns
- `Map` (8 methods, 6 exported) built by `New() *Map`: `On`, `Toggle`, `Box`, `Click` (returns the camera offsets), `View`, `Draw`.
- `Source` (5 methods): `Width`, `Height`, `World`, `Snapshot`, `MinimapLayer(ver uint64) (game.MinimapView, bool)`.

## Depends on
- Leaves: `camera`, `gfx`, `hudlayout`, `uikit`. Layer: `render` (the view).
- `application/game` read models (`MinimapView`, `WorldView`, `PawnView`).

## Must not depend on
- Feature packages: the minimap is drawn by the root's HUD wiring; this package imports none of them.
- Other `render/*` layers: minimap is a leaf layer.

## Contracts
- `Click` returns camera offsets; the caller applies them to the camera. The package never moves the camera itself.
- `Draw` takes the destination image; the baked layer is requested by version through `Source.MinimapLayer`.

## Invariants
- The baked map is rebuilt only when `MinimapLayer(ver)` returns a new version.
- Clicks centre the camera on the clicked tile (`TestClickReturnsCentringOffsets`); the minimap toggles and colours as specified (`TestOnToggleAndColors`).

## Hot paths
- `Draw` runs per frame; it blits the cached layer and the viewport frame. The bake is version-keyed, not per frame. The root's `MinimapLayer500` is the gate.

## Tests
- 1 source file, 1 test file, 2 `Test*` functions.
- Coverage 29.3 % (ARCH-004).

## Notes
- Smallest render layer (271 lines).
