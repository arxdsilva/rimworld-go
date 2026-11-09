# ADR 0004: a render-command list

Status: accepted (2026-11-08); narrowed by the 2026-11-08 revision below (PERF-007). Gap rows: GAR-140 to GAR-169.

## Context

Each render layer draws straight to its target. A lit sprite is one `DrawTrianglesShader32` call with its own sprite-plus-normal-map "pair" image (`render/light/light.go`). That keeps uniforms out of the draw, but every draw has a different source image, so Ebitengine cannot merge them. The draw pass costs the most CPU of any frame segment: a 5 s probe on the grown 500×500 colony at zoom 0.8 measured an average draw of 11.6 ms out of a 16.7 ms frame.

Ebitengine v2.10.6 merges a draw only into the command queued immediately before it, and only when the shader pointer, destination, all four source images, blend and uniform values are equal. Any other command in between breaks the chain: a fill, a pixel write, or a new image. On Metal every change of render target loads and stores the whole target, about 59 MB each way at 5K.

## Options

| | Batch inside each layer | **Render-command list (chosen)** |
|---|---|---|
| pros | A small change | The fewest calls; batching across layers; one place for culling, stats and order; easier to thread later |
| cons | No batching across layers; each layer reinvents it | Every layer's draw path is rewritten; today's order must hold exactly; needs an allocation-free list |

## Decision

1. **Layers emit and never draw.** `render/cmdlist` keeps one per-frame arena:
   - a grow-only `[]ebiten.Vertex`;
   - `[]cmd{key uint64; vtx, nvtx uint32; state uint16; dst uint8}`;
   - an interned state table `{shader, images [4], blend, uniformsID}`;
   - a static quad index pattern.

   A frame resets everything to length 0. Ebitengine copies vertices during the call, so the arena is free once `Submit` returns.
2. **The 64-bit key is `pass:4 | layer:8 | sub:32 | seq:20`.**
   - `pass` orders render targets, so all draws to one target are contiguous, and a pass samples only lower passes.
   - `layer` is today's layer.
   - `sub` depends on the layer's mode. In the default *strict* mode it is 0. In *y-sorted* mode it is the screen y, for layers that sort that way today. In *binned* mode it is the state ID, only for layers proven order-independent, such as opaque terrain.
   - `seq` is the emit counter, so keys are unique and the order is deterministic.
   - Clears and target switches are explicit command entries, not fake sprites.
3. **Sort only when needed.** An O(n) "already sorted?" check comes first, which is the common case. Otherwise an LSD radix sort over (key, index), skipping single-bucket bytes.
4. **Merge exactly when Ebitengine would.** Adjacent commands merge when they share the target, shader, four source sheets, blend and interned uniforms. A batch is capped at about 16k quads and never crosses a command entry. Each batch is one draw with a reused options struct and uniforms map.
5. **Light rides in the vertices.**
   - Per-sprite data goes in `Custom0..3` and `Color*`; uniforms hold only frame constants.
   - The per-sprite pair images give way to sheet-level normal-map pages, so lit sprites from one sheet merge.
   - A shader lint bans region-dependent helpers (`imageSrc0At`, `imageSrcN…` for N ≥ 1, `Origin` and `Size`) in list shaders, generalising `blood_draw_test.go`. That means fixing `water.kage` and `tint.kage`.
6. **Sources are sheets we own.**
   - Every sheet is loaded behind the loading screen.
   - Render targets are `Unmanaged`.
   - A debug-log test bounds the distinct source pages per world frame.
7. **Cheaper full-screen work.**
   - Shadows render at ½ linear size (a quarter of the fill) and composite with linear filtering.
   - The weather tint folds into the world composite or the final-screen pass.
   - The screen is not cleared when the terrain covers every pixel.
   - No AA fills or `FillRule` over large areas.
8. **Text and UI emit into the list too.**
   - A width cache keyed by (face, string ID), and per-face advance tables for numbers.
   - Glyphs are realized before any draw is submitted.
   - Widgets are plain functions keyed by stable IDs, with no closures and no strings per frame.
   - An `AllocsPerRun` gate of 0 per UI frame.
9. **Warm-up** (started in GAR E9, `light.SharedLit`): one tiny draw per (shader, blend, sheet) behind the loading screen. Release builds can later ship precompiled `.metallib` files via `exp/shaderprecomp`, regenerated on each Ebitengine upgrade.
10. **Auto resolution with hysteresis**, for the world and shadow passes only; the UI stays native:
    - the signal is an EWMA of the frame interval, and the resolution never drops while CPU-bound;
    - drop one step at 1.05× the budget for 30 frames, raise only below 0.80× for 120 frames, at least 60 frames between changes;
    - steps {1, 0.875, 0.75, 0.625, 0.5}, each a viewport into one max-size target;
    - a no-flap test.
11. **Proof.** The old path stays behind a flag until every layer has moved. Fixed scenes render through both paths in one process and must match byte for byte with zero tolerance. Draw calls are counted from an `ebitenginedebug` run. Reduced-resolution paths get their own golden images.
12. **Observable.** A per-frame stats snapshot feeds the debug HUD: commands, batches, sheets, targets, target switches, whether the sort ran, and the resolution step.

## Revision (2026-11-08, PERF-007): measured before building

An `ebitenginedebug` probe of the grown 500×500 colony at zoom 1 (`scripts/ebdebug.sh`) executes about **152 draw commands a frame** after Ebitengine's own merging, with 17 target switches and almost no pixel writes. The lit world is about 7 of those. Ebitengine already moves source-only images onto shared atlas pages, so lit draws from different pair images do merge, and lit.kage's uniforms are frame constants (§5's Custom slots are already used). The UI makes most commands: canvas images, text, and fills.

The CPU went elsewhere. In `BenchmarkDrawArt500/z1`, 0.82 s of 1.62 s of Draw CPU is Ebitengine's anti-aliased `vector` path. Each AA fill or stroke is deferred per destination and drawn through a stencil-buffer atlas when the destination is next used.

So the order changes:
1. AA shapes become feathered triangles drawn in place (`gfx/aashape`). This is the largest Draw win.
2. Byte-exact fixes inside layers: crops flush only on overlap; water needs no per-tile region uniforms; a prepare pass for bakes and pixel writes.
3. §1–§4 (the list, sort keys, radix sort) are **not built**. The go/no-go gate (≥ 1 µs of Ebitengine CPU per call × ≥ 1,500 calls a frame, or more than 350 commands) is not met. A strict-order coalescing emitter is the most that would follow, and only if a later in-game profile asks for it.
4. §11's proof is the pixel-diff capture (`-capture-diff`): old path, new path and old path again in one process, byte-equal for moves and batching, and a stated tolerance for the AA swap.

## Adoption

The list and lit-in-vertices land first (E7a). Then layers move one at a time in draw order, each pixel-equal on the capture scenes (E7b). Then half-resolution shadows and the folded weather (E7c), then Auto detail (E7d). The gate: `DrawArt500` calls drop by 70% or more at zoom 0.25.

## Consequences

- Layers depend on a small emitter port, not on `*ebiten.Image`.
- Sheet-level normal pages change the art pipeline: `atlasgen` writes normal pages beside colour pages, through the art skill, with no orphaned assets left behind.
- Draw order becomes data, so the debug HUD can show it.

## Sources

Research fragment R-render, with every URL:
- Ericson, "Order your graphics draw calls around!", on sort keys.
- Bevy render phases (sorted and binned).
- Terdiman on radix sort.
- The Ebitengine v2.10.6 source: `commandqueue.go`, `command.go`, `metal/graphics_darwin.go`.
- Dear ImGui (draw-list splitting, advance tables); Muratori, immediate-mode GUI, 2005.
- Unreal's dynamic resolution; Intel's dynamic resolution rendering.
- GPU Gems 3, on low-resolution passes.
