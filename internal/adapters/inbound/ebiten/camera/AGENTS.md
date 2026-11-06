# internal/adapters/inbound/ebiten/camera

## Purpose
The presentation camera: offset, zoom, device density, eased pan and zoom, viewports and clamping. Moved out of the ebiten root in ARCH-003 wave 3.1 (`camera/`, 326 lines).

## Owns
- `Camera` (5 methods: `Tile`, `SyncDev`, `MinZoomFor`, `Clamp`, `LimitsFor`) with exported state `OffsetX`, `OffsetY`, `Zoom`, `Placed`, `Dev`.
- `Smooth` (8 methods): eased pan and zoom toward a target.
- `Limits`: map bounds and slack for clamping.
- `Viewport` (`X0`, `Y0`, `X1`, `Y1`, `W`, `H`) with `ColonyViewport` and `ColonyViewportPhysical`.
- `ScreenDev()` (device density) and `ClampAxis`.
- Constants: `TileSize`, `EdgeSlack`, `PanMaxSpeed`; the root's `panPixels`, `minZoom`, `maxZoom` alias them.

## Depends on
- `gfx` (the device scale and logical pixel helpers); `math`.

## Must not depend on
- Domain, application and simulation packages: it takes screen size and device scale as arguments.
- Feature packages and `render/*` (camera is below them).

## Contracts
- `Smooth` eases toward its target; `Camera.Clamp(v, mapW, mapH, landing)` keeps the view on the map.
- `ColonyViewport(sw, sh)` and `ColonyViewportPhysical(sw, sh, s)` give the map rectangle for a screen size.
- `Viewport` edges are exported because the root's map-pixel guard reads them.

## Invariants
- Zoom and pan limits come from `Limits`; `ClampAxis` never returns a value outside the bounds plus `EdgeSlack`.
- Pure value types: `Camera` and `Viewport` are copied by value into the render view each frame.

## Hot paths
- `Smooth` and `Clamp` run once per `Update`; `WorldToScreen` lives in `render` and reads the camera by value.
- No allocation: all types are values; no interfaces.

## Tests
- 1 source file, 2 test files, 6 `Test*` functions: `TestClampAxis`, `TestPanAcceleratesAndCoasts`, `TestPanDoesNotOvershootBounds`, `TestZoomEasesToTargetAnchoredAtCursor`.
- `smooth_test.go` moved with the code. Coverage: not recorded in the handoff.

## Notes
- The root imports this package as `camerapkg` through the root `Renderer` (`renderer.go`, `layers.go`).
- `SyncDev` copies the device density into the camera; `Dev` is exported for the root's readers.
