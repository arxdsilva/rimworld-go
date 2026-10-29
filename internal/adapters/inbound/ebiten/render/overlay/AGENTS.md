# render/overlay package

## Package Purpose
Draws the map overlays over the ground and under or over the objects: blood and dirt stains, fog, roof, home area, plan marks, paths, zones and their labels, bridges, turret ranges, arrivals, fire, the power grid and its badges, the snow-clearing and allowed areas. Each overlay is a toggle the HUD flips and a tool may force. A `Set` reads the frame's `*render.View`, its painter and its light layer, never the renderer. ARCH-004 wave 4.3 (agent R6).

## Owned Concepts
- `Set` (24 methods, 23 exported) over one `*base`, with the sub-sets `Zones` (12 methods), `Fires` (9), `Wiring` (20: power and turrets).
- `Toggle{On, Forced}` with `Shown()` and `Toggle()`: roof, home, plan, power and snow-clear use it.
- Link kinds: `LinkKinds`, `BuildingLink`, `PowerLink`, used as `sprite.LinkGrid[overlay.LinkKinds]`.
- Stain and dirt pools (`sync`-guarded), the fog and label caches.

## Allowed Dependencies
- Leaves: `art`, `camera`, `gfx`, `uikit`.
- Layers: `render` (the view), `render/light`, `render/motion`, `render/sprite`.
- `application/game` read models; domain `building/power`, `sim/clock`, `sim/entity`, `world/roof`, `world/terrain`.
- Ebitengine (`ebiten`, `vector`); stdlib (`fmt`, `image`, `image/color`, `math`, `sort`, `strconv`, `strings`, `sync`).

## Forbidden Dependencies
- The ebiten root package and `cmd/`.
- Feature packages (`architect` and `hud` import overlay; overlay imports neither).
- `render/world`, `render/creature`, `render/env`, `render/cue`: overlay is a sibling layer that `world` composes.

## Public Contracts
- `Set` methods are called by `render/world` in the world's draw order.
- `Toggle` is written by the HUD (`hud` through `ports.Overlays`) and forced by the Architect's tools.

## Invariants
- Stain alpha equals the stain amount (`TestStainAlphaIsAmount`); stains dry darker (`TestStainDriesDarker`); the ichor colour is fixed (`TestIchorColour`).
- Pool access is guarded by `sync` (stains and dirt are read by the draw path and written by the sim sync).
- Guard limits: `Set` 24 methods, `Wiring` 20, `Zones` 12 (all under 40).

## Hot Paths
- Each overlay draws per frame over the visible tiles; the stain and dirt pools are read per tile. The package has no benchmark; the root's `DrawArt500` measures the frame.
- The `sync` locks sit on the pool access path; keep the critical section to the lookup.

## Determinism Rules
- Presentation only. Overlays read the view and their own pools; the package reads no simulation RNG and no wall clock.

## Save/Mod Impact
- None.

## Tests
- 19 source files, 16 test files, 53 `Test*` functions.
- Coverage 59.2 % (ARCH-004, 4,818 lines).

## Benchmarks
- None in the package.

## Known Design Constraints
- The App halves (`*_ui.go`) and 63 `Renderer` wrappers stay in the root; the sweep deletes the wrappers.
