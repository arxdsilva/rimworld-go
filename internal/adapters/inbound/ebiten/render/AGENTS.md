# render package

## Package Purpose
The frame context the renderer's draw layers read: `View`, filled once per frame by the root `Renderer` from one read of the simulation, and `Trace`, the test hooks (nil in production). Layers get a `*View` and never reach back into the `Renderer` (SOLID_GO.md §7). The shared base of every `render/*` package, so layers can depend on it without depending on the renderer.

## Owned Concepts
- `View`: this frame's camera, art catalog, world, pawns, clocks (`AnimT`, `Alpha`, `Present`) and options. Filled once at the top of `Renderer.Draw`.
- Methods: `WorldToScreenF`, `WorldToScreen` (float and tile coordinates to screen), `LayerDrawn`, `MarkDrawn`, `Tracing`.
- `Trace`: `Layer func(name string)` (layer order tests) and `AnimalMark func(kind string, a, b, c, d float32)` (mark tests). Both nil in production.

## Allowed Dependencies
- `art` (catalog), `camera` (the camera value).
- `application/game` read models; domain `sim/clock`, `sim/entity`.
- Standard library only otherwise.

## Forbidden Dependencies
- The ebiten root package (`Renderer`, `App`) and `cmd/`.
- Feature packages (`hud`, `architect`, `inspector`, `windows`, and the rest) and `ports`.
- Any `render/*` child package: they import this one, not the reverse.

## Public Contracts
- `View` is a struct passed by pointer to every layer. Layers read its fields; they do not store the simulation.
- `WorldToScreenF` is the coordinate mapping layers use; the ARCH-004 gate checks it with `-gcflags=-m=2`.

## Invariants
- `View` is written once per frame, before any layer runs.
- `Trace` hooks are nil in production; the layers check `Tracing()` before calling them.

## Hot Paths
- `WorldToScreenF` runs per drawn object. It is a pure method on a value; it must stay inlinable and allocation-free.
- `LayerDrawn` and `MarkDrawn` are no-ops unless a test sets `Trace`.

## Determinism Rules
- Presentation only. `View` carries the simulation's read values; it never writes back and never reads the RNG or the clock.

## Save/Mod Impact
- None.

## Tests
- None in the package. The layer packages' tests build a `View` directly.

## Benchmarks
- None in the package. The frame benchmarks are the root's `DrawArt500`, `ChunkLayer500`, `MinimapLayer500` (ARCH-004 gate).

## Known Design Constraints
- Lesson from ARCH-004 wave 4.3: never pass a pointer to a frame local through an interface method. Escape analysis hides the callee, so `w := r.game.World()` moved to the heap until the list slices were passed by value. Pass the slices or values a layer reads.
- The root's `Renderer` has 53 fields after wave 4.3; the sweep (ARCH-006) removes the one-line wrappers.
