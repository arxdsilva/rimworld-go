# internal/adapters/inbound/ebiten/render

## Purpose
The frame context the renderer's draw layers read: `View`, filled once per frame by the root `Renderer` from one read of the simulation, and `Trace`, the test hooks (nil in production). Layers get a `*View` and never reach back into the `Renderer` (SOLID_GO §7). The shared base of every `render/*` package, so layers can depend on it without depending on the renderer.

## Owns
- `View`: this frame's camera, art catalog, world, pawns, clocks (`AnimT`, `Alpha`, `Present`) and options. Filled once at the top of `Renderer.Draw`.
- Methods: `WorldToScreenF`, `WorldToScreen` (float and tile coordinates to screen), `LayerDrawn`, `MarkDrawn`, `Tracing`.
- `Trace`: `Layer func(name string)` (layer order tests) and `AnimalMark func(kind string, a, b, c, d float32)` (mark tests). Both nil in production.

## Depends on
- `art` (catalog), `camera` (the camera value).
- `application/game` read models; domain `sim/clock`, `sim/entity`.

## Must not depend on
- The ebiten root package (`Renderer`, `App`), feature packages (`hud`, `architect`, `inspector`, `windows`, and the rest) and `ports`.
- Any `render/*` child package: they import this one, not the reverse.

## Contracts
- `View` is a struct passed by pointer to every layer. Layers read its fields; they do not store the simulation or write back.
- `WorldToScreenF` is the coordinate mapping layers use; the ARCH-004 gate checks it with `-gcflags=-m=2`.

## Invariants
- `View` is written once per frame, before any layer runs.
- `Trace` hooks are nil in production; the layers check `Tracing()` before calling them.

## Hot paths
- `WorldToScreenF` runs per drawn object. It is a pure method on a value; it must stay inlinable and allocation-free.
- `LayerDrawn` and `MarkDrawn` are no-ops unless a test sets `Trace`.
- No benchmark in the package. The frame benchmarks are the root's `DrawArt500`, `ChunkLayer500`, `MinimapLayer500` (ARCH-004 gate).

## Tests
- None in the package. The layer packages' tests build a `View` directly.

## Notes
- Lesson from ARCH-004 wave 4.3: never pass a pointer to a frame local through an interface method. Escape analysis hides the callee, so `w := r.game.World()` moved to the heap until the list slices were passed by value. Pass the slices or values a layer reads.
- The root's `Renderer` has 53 fields after wave 4.3; the sweep (ARCH-006) removes the one-line wrappers.
