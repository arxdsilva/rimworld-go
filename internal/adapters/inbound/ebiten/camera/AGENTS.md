# internal/adapters/inbound/ebiten/camera

## Purpose
The presentation camera: offset, zoom, device density, eased pan and zoom, viewports and clamping. Moved out of the ebiten root in ARCH-003 wave 3.1 (`camera/`, 326 lines).

## Owns
- `Camera` (`Tile`, `SyncDev`, `SyncUnit`, `MinZoomFor`, `Clamp`, `LimitsFor`, …) with exported state `OffsetX`, `OffsetY`, `Zoom`, `Unit`, `Placed`, `Dev`.
- `Smooth` (8 methods): eased pan and zoom toward a target.
- `Limits`: map bounds and slack for clamping.
- `Viewport` (`X0`, `Y0`, `X1`, `Y1`, `W`, `H`) with `ColonyViewport` and `ColonyViewportPhysical`.
- `ScreenDev()` (device density) and `ClampAxis`.
- `UnitFor(h)`: the cell for a window h map pixels tall (h / `CellsTall`, UIS-080).
- Constants: `TileSize` (the art's 64-px reference cell), `CellsTall` (48), `MinZoom` 0.40 and `MaxZoom` 2.18 of the default (UIS-081), `EdgeSlack`, `PanMaxSpeed`, `PanPixels`.

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
- The cell is `Unit` x `Zoom`; `Unit` is the window's height / 48 in map pixels, synced by `Clamp` and `Smooth.Step` (`SyncUnit` rescales the offsets, velocity, anchor and fx so the top-left world point stays). An unsynced camera (`Unit` 0: bakes, tests) uses `TileSize`. The UI scale never enters it (UIS-084).
- The zoom-out floor is `MinZoom`, unless the map is too small to cover the window there (`MinZoomFor`).
- Pure value types: `Camera` and `Viewport` are copied by value into the render view each frame.

## Hot paths
- `Smooth` and `Clamp` run once per `Update`; `WorldToScreen` lives in `render` and reads the camera by value.
- No allocation: all types are values; no interfaces.

## Tests
- 1 source file, 2 test files, 6 `Test*` functions: `TestClampAxis`, `TestPanAcceleratesAndCoasts`, `TestPanDoesNotOvershootBounds`, `TestZoomEasesToTargetAnchoredAtCursor`.
- `smooth_test.go` moved with the code. Coverage: not recorded in the handoff.

## Notes
- The root imports this package as `camerapkg` through the root `Renderer` (`renderer.go`, `layers.go`).
- `SyncDev` copies the device density into the camera (pan speeds); `Dev` is exported for the root's readers. The density reaches the cell through the physical window height only.
- Saved views (`View`) keep the zoom in the default-relative unit; the sidecar's version (`filesave.ViewVersion`) maps older saves to the default zoom, centre kept (UIS-087).
