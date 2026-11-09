# internal/adapters/inbound/ebiten/render/light

## Purpose
The renderer's sun, moon and lamp lighting: the light grid (sky exposure and lamp light per tile), the lit-sprite shader (`lit.kage`) and its pair images, the light sources, the colonists' drop shadows, daylight and the no-art sky overlay. It reads the frame's `*render.View` and its own state, never the renderer. ARCH-004 wave 4.1 (agent R2). Only colonists on unroofed tiles cast a shadow (user decision, 2026-11-01, STK-SH): the box and blob casters of buildings, rock, plants and items, their layer and blur shader, and ART-012's creature shadows are gone.

## Owns
- `Lit` (10 exported methods): `Draw`, `DrawGround`, `LightTile`, `LightFoot`, `Sync`, `SourceOf`, `GroundLamp`, `LitShader`, `SyncUniforms`, `BatchGround`. `Blaze` is the fire-light hook.
- The ground batch (`ground_batch.go`, GAR-159): while the pipeline's live ground pass has it on, lit ground draws append their quads to one reused buffer, sampling copies of their images on a ground sheet (2048 px pages, a 1 px transparent gutter, texel copies), and go out as one `DrawTrianglesShader32`.
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
- The ground batch flushes when the destination, sheet page, shader or light changes, before any `Lit.Draw` that does not join (a bake, lighting off, no shader, a non-ground sprite), and on `BatchGround`. A caller that draws past the light layer while the batch is on (water through `render/env`) turns it off around that draw. Byte-equal to the per-tile calls under the same atlas placement (`TestGroundBatch*`, the pixel-diff harness).
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
