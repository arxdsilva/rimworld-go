# render/world package

## Package Purpose
The renderer's world pipeline: the draw order of one frame (ground, floors, zones, objects by row, marks and labels), the zoom tiers with the chunk and whole-map bakes, the frame's chunk lists, the terrain, plant, item and building art keys, the stack counts, chat bubbles and damage marks. `Pipeline` (the order) composes the layer packages (`sprite`, `light`, `env`, `creature`, `overlay`, `cue`) through its parts `Bakes`, `Objects` and `Labels`. It reads one `*render.View` per frame and asks the renderer only through `Source`. ARCH-004 wave 4.4 (agent R9).

## Owned Concepts
- `Pipeline` (9 methods): the draw order, copied line for line from the root.
- `Bakes` (11 methods): zoom tiers, chunk and whole-map bakes, the frame's chunk lists.
- `Objects` (9), `Labels` (9), `LOD` (3): the tier selection.
- `base` (33 methods) shared by the parts; `Shared` points at the renderer's camera, cast, blood, `lod`, `links` and `stoneArt`.
- `Source` (6 methods, values only): `World`, `Snapshot`, `Date`, `ChunkLayer`, `PlantsIn`, `DrawOrderRoute`. The simulation is reached only through it.
- `NewPlantKey(p, h)`: plant art key.

## Allowed Dependencies
- Leaves: `art`, `camera`, `gfx`, `uikit`.
- Layers: `render` (the view), `render/creature`, `render/cue`, `render/env`, `render/light`, `render/motion`, `render/overlay`, `render/sprite`.
- `application/game` read models; domain `sim/clock`, `sim/entity`.
- Ebitengine (`ebiten`, `vector`); stdlib (`cmp`, `fmt`, `image`, `image/color`, `math`).

## Forbidden Dependencies
- The ebiten root package (`Renderer` is not imported; the root calls `r.pipeline().Draw(screen)`) and `cmd/`.
- Feature packages.
- Simulation packages beyond the read values in `Source` and the view.

## Public Contracts
- `New(Layers, Hosts, Shared, *render.View, Source)` builds the pipeline once; `Pipeline.Draw(screen)` runs one frame.
- `Source` is consumer-owned and values only; the root's renderer satisfies it.

## Invariants
- The draw order is the order the root had before the move; the layer order tests (through `render.Trace.Layer`) pin it.
- `LOD` selects the zoom tier used by `Bakes` and `Objects`.

## Hot Paths
- `Pipeline.Draw` runs once per frame and calls every layer. The chunk lists are built once per frame; ARCH-004 passed the list slices by value into the layers for this reason.
- ARCH-004 gate: no `moved to heap` in this package (checked with `-gcflags=-m=2`); the `ChunkLayer500` benchmark in the root is the gate.
- Rule: never pass a pointer to a frame local through `Source` or any other interface (see `render/AGENTS.md`).

## Determinism Rules
- Presentation only. The pipeline reads the simulation through `Source` and never writes back; no RNG and no wall clock.

## Save/Mod Impact
- None.

## Tests
- 17 source files, 3 test files, 20 `Test*` functions; `fakeHosts` (13 methods) drive them.
- Coverage 77.6 % (ARCH-004, 3,023 lines).

## Benchmarks
- None in the package. `ChunkLayer500` in the root is the gate.

## Known Design Constraints
- `lod`, `links` and `stoneArt` stay `Renderer` fields in the root (about 20 root files read `r.lod`); the pipeline reaches them through `Shared`.
- The root's trimmed `draw_art.go` keeps the placement ghost (it takes the Architect's `ghostCell`); `stone_art.go` keeps the titles.
- `architect/sprites.go` duplicates these build keys until the sweep points it here.
