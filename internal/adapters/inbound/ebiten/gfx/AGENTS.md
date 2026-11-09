# internal/adapters/inbound/ebiten/gfx

## Purpose
The client's drawing primitives, shared by the HUD and the renderer: the UI canvas (logical pixels in, physical out, scaled by the UI scale), text, rectangles, the triangle batch, and the world pass that queues canvas draws aimed at the render-scaled map. It knows nothing of the game. Moved out of the root in ARCH-003 wave 3.0 (1,094 lines).

## Owns
- Canvas primitives: `FillRect`, `StrokeRect`, `Line`, `FillCircle`, `StrokeCircle`, `FillPoly`, `Image`, `DrawImage`, `Icon`, `ImageOp`, `ScaledVerts`, `ClipTo`.
- Text: `Text`, `Measure`, `LineH`, `LineStep`, `TextW`, `MaxTextW`, `Ellipsize`, `FaceSize`; styles `Small` through `Huge`, `TextColor`; embedded Noto fonts under `gfx/fontdata/` (no runtime file access).
- `Runs` (`textrun.go`, GAR-160): a caller-owned cache (cap `RunCap`, least recently drawn evicted) of rendered integer labels, one `DrawImage` per stack count instead of a shaping pass and a draw per glyph; keyed by number, face, scales and style size, tinted at draw time; a world pass or a draw trace goes through `Text`.
- `Rect` (16 methods) and `WinRect`.
- `Scale`, `UnitScale`, `UIScale`, `HUDScale`, `MapUIScale`, `SetScale`.
- `WorldPass`, `BeginWorldPass`, `EndWorldPass`, `CompositeWorld`, `FitLayer`; `TriBatch` (`Vert`, `Tri`, `Rect`, `Line`, `Flush`).
- `aashape/` (PERF-008): anti-aliased discs, rings, lines, rects, frames and convex polygons as triangles with a 1-px alpha feather, drawn in place with one `DrawTriangles32` from a white texel, so they keep call order and merge. Every `aa` branch of the canvas primitives and `PxDisc`, `PxRing`, `PxLine`, `PxFrame` (physical pixels) go through its `Painter`, held on `cur.aa`.
- `state.go`: `cur`, the one process-wide canvas state (UI scale, world pass, test traces). `SetPass`, `ForcePass`, `Pass`, `SetVia`, `Traced`, `SetCanvasTrace`, `SetDrawTrace`, `FogDrawn`.

## Depends on
- Ebitengine: `ebiten`, `ebiten/text/v2`, `ebiten/vector`.

## Must not depend on
- Every other package in the ebiten tree (gfx is the bottom of the UI stack); it cannot depend on the root's `Renderer` or `App`.

## Contracts
- Draw helpers take `*ebiten.Image` and logical coordinates; they convert to physical pixels using the current `Scale`.
- Tests set canvas state through the setters (`withUIScale`, `traceCanvas`, `wlTrace`); tests must not write `cur` directly.
- The root and every package call `gfx` directly (the transition wrappers are gone, ARCH-006).

## Invariants
- `cur` is the documented exception to the no-package-state rule (`.agents/specs/SOLID_GO.md` §9); it is main-goroutine only and carries one budget line in `internal/arch/solid_budget.txt` (`gfx.cur`). A second global here is a rule violation; tests reset `cur` through setters.
- No anti-aliased `ebiten/vector` call (`…, true)`, `AntiAlias: true`) outside `aashape/guard_test.go`'s allowlist: the colonist route polylines and the concave-polygon fallback. Ebitengine draws AA vector paths through a stencil atlas per target, which cost half the Draw CPU before PERF-008.
- The canvas guard in the root (`TestUIDrawsThroughCanvas`) requires UI draws to go through `gfx.X`.

## Hot paths
- Every primitive is called per frame by the HUD, the renderer and the features.
- New helpers must not allocate per call beyond the image operation itself (SOLID_GO §7). Not benchmarked here.
- `WorldPass.Flush` replays the deferred draws queued for the world pass.

## Tests
- `aashape` tests coverage against an exact supersampled reference (|Δ| ≤ 0.1 per pixel, area ≤ 0.5 %), 0 allocations, and the guard; `BenchmarkAAShapes`.
- `textrun_test.go` covers `Runs` (metrics, count change, scale and font invalidation, the cap, 0 allocations on a hit).
- No other test files in the package. The root's source-scanning guards (`TestUIDrawsThroughCanvas`, `TestMapPixelsComeFromMapCursor`, `TestLightsComeOnlyFromLightSources`) walk `gfx/*.go` through `packageFiles`, and the feature packages' tests exercise it.
- Coverage: not measured.
