# internal/adapters/inbound/ebiten/render/light

## Purpose
The renderer's sun, moon and lamp lighting: the light grid (sky exposure and lamp light per tile), the lit-sprite shader (`lit.kage`) and its pair images, the light sources, the colonists' drop shadows, daylight and the no-art sky overlay. It reads the frame's `*render.View` and its own state, never the renderer. ARCH-004 wave 4.1 (agent R2). Only colonists on unroofed tiles cast a shadow (user decision, 2026-11-01, STK-SH): the box and blob casters of buildings, rock, plants and items, their layer and blur shader, and ART-012's creature shadows are gone.

## Owns
- `Lit` (13 exported methods): `Draw`, `DrawGround`, `LightTile`, `LightFoot`, `Sync`, `SourceOf`, `GroundLamp`, `LitShader`, `SyncUniforms`, `Batch`, `Flush`, `Stats`, `StartFrame`. `Blaze` is the fire-light hook.
- `Stats` (`stats.go`, GAR-161): one frame's lit-draw counts (the calls Lit issued, the batched quads, the batch flushes by `FlushReason`, the sheet copies and resets); the pipeline calls `StartFrame` at the top of the world pass, which zeroes them and then makes the sheet copies the last frame asked for; `BenchmarkDrawArt500` reports `batched/op` and `flush-raw/op`, and `Stats().String()` is the form for a temporary in-game log (the PERF probe does not print it).
- The lit batch (`lit_batch.go`, GAR-159 ground, GAR-161 objects): while a pass has it on (`Batch(true)`: the live ground pass, and the object pass from after the cord stubs to before the last crop flush), every shaded lit draw with no blood whose image is on the sheet appends its quad to one reused buffer, sampling the copy of the image it would have sampled (the sprite, or its normal-map pair), and the quads go out as one `DrawTrianglesShader32` per run. An image not on the sheet yet draws alone (`FlushNew`) and asks for a copy.
- `Grid` (13 methods): sky exposure and lamp light per tile; `AtF` is the float lookup.
- `Shadows` (3 methods: `Draw`, `SetOff`, `blobTexture`), `Foot` (a colonist's ground pivot and scale), `Colonist`/`Casts` (who casts: a colonist, not a prisoner, visitor, ally, guest, raider, body or carried pawn, on a tile with no built or natural roof), `ColonistQuad`, `ColonistAlpha`. The pipeline (`world.Pipeline.ShadowFeet`) fills the feet. `Cast` is an empty marker the pipeline sets during a bake (`sprite.Pass.Cast`).
- Daylight and the sky overlay; one embedded shader (the lit-sprite shader `lit.kage`).

## Depends on
- Leaves: `art`, `camera`, `gfx`; layer `render` (the view).
- `application/game` read models; domain `sim/clock`, `sim/entity`, `world/lighting`, `world/roof`, `world/room`, `world/weather`.
- `embed` (shaders); `pkg/mathx` (`Abs`), `pkg/randx` (`Mix64`).

## Must not depend on
- Sibling layers that import light (`creature`, `env`, `overlay`, `sprite`, `world`): light sits below them.
- The simulation's RNG or clock; lighting reads the clock values in the view.

## Contracts
- `Lit.Draw` draws one lit sprite; `Grid.AtF` and `LightTile` are called per sprite. ARCH-004 recorded that `Grid.AtF`, `LightTile` and `Lit.Draw` did not inline before the move either.
- The lit batch flushes when the destination, sheet page, shader or light changes, at its quad cap, before any `Lit.Draw` that does not join (a bake, lighting off, no shader, a bloodied figure: its noise reads absolute source positions), on `Batch`, and on `Flush`. Every raw draw the object pass reaches (DrawImage, vector, gfx shapes and text, a flame, a triangle batch of its own) calls its `*light.Lit`'s `Flush` first, so it lands over the quads queued before it: the root's `TestObjectPassFlushesBeforeRawDraws` walks the pass by name. A caller that draws past the light layer while the batch is on (water through `render/env`) turns it off around that draw.
- The sheet: 4096 px unmanaged pages, at most `sheetPages` (2, 128 MB), allocated as needed; texel copies (`BlendCopy`) of the whole source with a 1 px transparent gutter, packed in shelves, keyed by the sprite image (not its pair: the pair cache may evict and rebuild a pair mid-frame). Copies are made only in `StartFrame`, in the order the last frame asked for them, never between two draws of a frame: a draw to a page between two screen draws ends the screen's render pass, about 1 ms of GPU each on a 5K screen (PERF-015: copies after a mid-frame reset made 30 ms frames). An asked copy whose pair the cache has since evicted is skipped. The sheet starts again in `StartFrame` for a new catalog, and, at most once per `resetCooldown` (120) frames, when the asked copies do not fit or the last frame averaged fewer than `scatterRun` (8) quads per page change; the frame after then draws alone and asks for all its images in draw order (no-flap: `TestLitBatchFullSheetResetCooldown`, `TestLitBatchScatterRepacks`). Byte-equal to per-sprite calls from the same sheet (`TestLitBatch*`, `TestGroundBatch*`, the pixel-diff harness); against the images' own atlas places a few pixels differ by 1/255 (the source position is interpolated at another magnitude).
- `Shadows.Draw(v, light, screen, feet, dayOfYear)` draws every blob in one batched draw with a texture made once; no per-colonist allocation (`TestColonistShadowPass`).
- Only colonists outdoors cast: the root's `TestOnlyColonistsOutdoorsCastShadows` runs whole frames with walls, rock, a tree, items, a bridge, an animal, a prisoner and a raider at noon, low sun and moon; `TestJourneyColonistShadowAndToggle` checks the Options toggle.

## Invariants
- Lamps are brighter near than far in a room (`TestLampNearBrighterThanFarInRoom`); night is darker than noon (`TestSkyOverlayNightDarkerThanNoon`); day overlay is clear (`TestSkyOverlayDayIsClear`).
- Lights come only from light sources (`TestLightsComeOnlyFromLightSources`, a root guard).
- Light values are functions of the view's clock, weather, roof and room read models.

## Hot paths
- `Lit.Draw` runs once per lit sprite; `LightGridLamps` rebuilds the grid's lamp light. `BenchmarkLightGridLamps` measures it: 142–153 µs before, 139–142 µs after on a quiet run, 0 allocs (ARCH-004). The root's `BenchmarkShadows` is the colonist pass.

## Tests
- 5 source files, 6 test files.
- Coverage 84 % (ARCH-004).

## Notes
- No shadow textures or metadata exist (`make art-check` rejects a `shadow:` block).
