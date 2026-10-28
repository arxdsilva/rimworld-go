# ARCH-003: the shared leaves leave the ebiten root

## Why
Every later package (renderer layers, App features) needs the drawing primitives, the camera, the art catalog, the input frame and the HUD layout as packages below it, imported one way (SOLID_GO.md §7; `.cursor/HEXAGONAL_ARCHITECTURE.md` §6). This phase moves those leaves out of `internal/adapters/inbound/ebiten` with the root's call sites unchanged (aliases and one-line wrappers, deleted as each feature moves), so the tree is green after every step.

## Wave 3.0 — `gfx` (lead)
- `gfx/`: `canvas.go` (the UI canvas: `FillRect`, `StrokeRect`, `Line`, `FillCircle`, `StrokeCircle`, `FillPoly`, `Image`, `DrawImage`, `Triangles`, `Triangles32`, `PhysRect`, `WorldText`, `WorldIcon`, the trace event), `text.go` (`Text`, `Measure`, `LineH`, `WrapPx`, `Ellipsize`, `TextW`, the styles `Small`…`Huge`, `TextColor`, the embedded Noto fonts under `gfx/fontdata/`), `rect.go` (`Rect` and its methods, `WinRect`, `ClipTo`), `scale.go` (`Scale`, `UnitScale`, `UIScale`, `HUDScale`, `MapUIScale`), `worldpass.go` (`WorldPass`, the deferred canvas draws, `CompositeWorld`), `batch.go` (`TriBatch`, `FitLayer`, `Clamp01`, `Abs32`), `state.go` (`cur`: the one process-wide canvas state — scale, world pass, test traces — with `SetScale/CurrentScale`, `BeginWorldPass/EndWorldPass/WorldPassK/CurrentPass/FogDrawn`, `SetPass/ForcePass/Pass`, `SetVia/Via`, `Traced`, `SetCanvasTrace/SetDrawTrace`, `WorldPass.Flush/Begin/Len`, `Icon`). 1,094 lines.
- Root: `gfx_compat.go` — type aliases (`rect`, `textStyle`, `align`, `triBatch`, `uiScaleState`, `traceEvent`, `worldDeferQueue`), the style and scale vars, and one-line wrappers for every moved function; `asButton`, `fieldInner` became free functions (an alias cannot carry methods). The members root code reaches were exported with 28 `gopls rename`s first (`Rect.Right/Bottom/…`, `TriBatch.Reset/Vert/Tri/Rect/Line/Flush/Verts/Idx/K`, `Scale.Dev/UI/S/World/WorldK`, `WorldPass.FogDrawn`), and the 464 unkeyed `rect{…}` literals were keyed (`go vet`'s composites check flags unkeyed literals of another package's type).
- Globals: `curUIScale`, `tracePass`, `traceVia`, `canvasTrace`, `drawTrace`, `worldPassK`, `worldDefer` are gone; tests set the canvas state through the `gfx` setters (`withUIScale`, `traceCanvas`, `wlTrace`, …). `gfx.cur` is the documented exception (SOLID_GO.md §9), one budget line.
- Guards: the four source-scanning tests (`TestUIDrawsThroughCanvas`, `TestEveryInputActionHasAJourney`'s emitter scan, `TestLightsComeOnlyFromLightSources`, `TestMapPixelsComeFromMapCursor`) walk the package tree through `packageFiles` (keys like `gfx/canvas.go:FillRect`), so a moved file stays covered; the canvas guard reads `gfx.X` as the canvas.

## Wave 3.1 — `camera`, `art`, `inputframe`, `hudlayout`, `globe` (agents)
_Filled when the wave lands._

## Tools (scratchpad, reusable by every later wave)
`needexport` (go/types: which unexported identifiers of a file set are used outside it), `movedecls` (moves named declarations into another package's file, pruning imports), `splitrecv` (moves every method of a receiver out of a file), `keylits` (keys unkeyed literals of a type). The order that works: `gopls rename` the members root code reaches (type-aware, serial — concurrent gopls runs clobber each other), then move, then export the rest inside the new package with a word-bounded `perl -pi`, then aliases and wrappers in a root `<pkg>_compat.go`, then `keylits`, then build / vet / tests.

## Findings
- `go vet` composites: unkeyed literals of an imported struct type fail vet; keyed literals are the fix, not a vet flag.
- A word-bounded rename inside the new package also hits string literals (`"worldText"` via tags); check `"[A-Z]…"` strings after the perl pass.
- `make art-check` cannot run on this machine (`validate_assets.py` needs PIL in the Python env); the Go halves (`TestNoOrphanSprites`, `TestPlantRecipesHaveNoOutline`) run inside the package tests and pass.
- The ARCH-002 benchmark baseline was recorded while `-race` suites ran; re-record on a quiet machine before ARCH-004.
