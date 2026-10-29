# gfx package

## Package Purpose
The client's drawing primitives, shared by the HUD and the renderer: the UI canvas (logical pixels in, physical out, scaled by the UI scale), text, rectangles, the triangle batch, and the world pass that queues canvas draws aimed at the render-scaled map. It knows nothing of the game. Moved out of the root in ARCH-003 wave 3.0 (1,094 lines).

## Owned Concepts
- Canvas primitives: `FillRect`, `StrokeRect`, `Line`, `FillCircle`, `StrokeCircle`, `FillPoly`, `Image`, `DrawImage`, `Icon`, `ImageOp`, `ScaledVerts`, `ClipTo`.
- Text: `Text`, `Measure`, `LineH`, `LineStep`, `TextW`, `MaxTextW`, `Ellipsize`, `FaceSize`; styles `Small` through `Huge`, `TextColor`; embedded Noto fonts under `gfx/fontdata/`.
- `Rect` (16 methods) and `WinRect`.
- `Scale`, `UnitScale`, `UIScale`, `HUDScale`, `MapUIScale`, `SetScale`.
- `WorldPass`, `BeginWorldPass`, `EndWorldPass`, `CompositeWorld`, `FitLayer`; `TriBatch` (`Vert`, `Tri`, `Rect`, `Line`, `Flush`).
- `state.go`: `cur`, the one process-wide canvas state (UI scale, world pass, test traces). `SetPass`, `ForcePass`, `Pass`, `SetVia`, `Traced`, `SetCanvasTrace`, `SetDrawTrace`, `FogDrawn`.

## Allowed Dependencies
- Ebitengine: `ebiten`, `ebiten/text/v2`, `ebiten/vector`.
- Standard library: `embed`, `bytes`, `image`, `image/color`, `math`, `strconv`, `strings`, `sync`, `fmt`.

## Forbidden Dependencies
- Every other package in the ebiten tree (gfx is the bottom of the UI stack).
- Domain, application, and the root package; nothing imports `cmd/`.

## Public Contracts
- Draw helpers take `*ebiten.Image` and logical coordinates; they convert to physical pixels using the current `Scale`.
- Tests set canvas state through the setters (`withUIScale`, `traceCanvas`, `wlTrace`); tests must not write `cur` directly.

## Invariants
- `cur` is the documented exception to the no-package-state rule (SOLID_GO.md §9); it is main-goroutine only and carries one budget line in `internal/arch/solid_budget.txt` (`gfx.cur`).
- The canvas guard in the root (`TestUIDrawsThroughCanvas`) requires UI draws to go through `gfx.X`.
- Font files are embedded; no runtime file access.

## Hot Paths
- Every primitive is called per frame by the HUD, the renderer and the features.
- New helpers must not allocate per call beyond the image operation itself (SOLID_GO.md §7). Not benchmarked here.
- `WorldPass.Flush` replays the deferred draws queued for the world pass.

## Determinism Rules
- Presentation only. No RNG, no clock, no simulation reads.

## Save/Mod Impact
- None. Font data is embedded in the binary.

## Tests
- No test files in the package. The root's source-scanning guards (`TestUIDrawsThroughCanvas`, `TestMapPixelsComeFromMapCursor`, `TestLightsComeOnlyFromLightSources`) walk `gfx/*.go` through `packageFiles`, and the feature packages' tests exercise it.
- Coverage: not measured.

## Benchmarks
- None.

## Known Design Constraints
- `cur` is one package-level state; tests reset it through setters. A second global here is a rule violation.
- `gfx` cannot depend on the root's `Renderer` or `App`; the root and every package call `gfx` directly (the transition wrappers are gone, ARCH-006).
