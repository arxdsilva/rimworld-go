# render/light package

## Package Purpose
The renderer's sun, moon and lamp lighting: the light grid (sky exposure and lamp light per tile), the lit-sprite shader (`lit.kage`) and its pair images, the light sources, the box and blob shadows of buildings, rock, plants and items with their blur, daylight and the no-art sky overlay. It reads the frame's `*render.View` and its own state, never the renderer. The shadow pass asks the renderer for its casters through `Casters`. ARCH-004 wave 4.1 (agent R2). Creatures cast no shadow (user decision, 2026-10-31): the silhouettes, point-light shadows and contact pools of ART-012 are gone.

## Owned Concepts
- `Lit` (9 methods): `Draw`, `DrawGround`, `LightTile`, `LightFoot`, `Sync`, `SourceOf`, `GroundLamp`, `LitShader`, `SyncUniforms`. `Blaze` is the fire-light hook.
- `Grid` (13 methods): sky exposure and lamp light per tile; `AtF` is the float lookup.
- `Shadows[F]` (5 methods) with `Casters` (3 methods: `Objects`, `VisibleTiles`, `White`); `Caster`. `Cast` is an empty marker the pipeline sets during a bake (`sprite.Pass.Cast`).
- Daylight and the sky overlay; two embedded shaders (the lit-sprite shader `lit.kage` and the shadow pass shader).

## Allowed Dependencies
- Leaves: `art`, `camera`, `gfx`; layer `render` (the view).
- `application/game` read models; domain `sim/clock`, `sim/entity`, `world/lighting`, `world/roof`, `world/room`, `world/weather`.
- Ebitengine (`ebiten`); `embed`, `container/list`, `cmp`, `errors`, `strings`, `math`, `image`.

## Forbidden Dependencies
- The ebiten root package and `cmd/`.
- Feature packages and sibling layers that import light (`creature`, `env`, `overlay`, `sprite`, `world`): light sits below them.
- The simulation's RNG or clock; lighting reads the clock values in the view.

## Public Contracts
- `Lit.Draw` draws one lit sprite; `Grid.AtF` and `LightTile` are called per sprite. ARCH-004 recorded that `Grid.AtF`, `LightTile` and `Lit.Draw` did not inline before the move either.
- `Casters` is the only path from the shadow pass to the renderer.
- No creature reaches the shadow layer: the root's `TestCreaturesCastNoShadow` checks pawns and animals in sun, moonlight and lamp light add no caster, composite or lit draw.

## Invariants
- Lamps are brighter near than far in a room (`TestLampNearBrighterThanFarInRoom`); night is darker than noon (`TestSkyOverlayNightDarkerThanNoon`); day overlay is clear (`TestSkyOverlayDayIsClear`).
- Lights come only from light sources (`TestLightsComeOnlyFromLightSources`, a root guard).
- `Shadows[F]`'s type parameter is unused; it stays only while `render/env` declares the field as `Shadows[creature.FigurePose]`.

## Hot Paths
- `Lit.Draw` runs once per lit sprite; `LightGridLamps` rebuilds the grid's lamp light. `BenchmarkLightGridLamps` measures it: 142–153 µs before, 139–142 µs after on a quiet run, 0 allocs (ARCH-004).

## Determinism Rules
- Presentation only. Light values are functions of the view's clock, weather, roof and room read models. No RNG; no wall clock.

## Save/Mod Impact
- None. Shaders are embedded.

## Tests
- 5 source files, 6 test files, 40 `Test*` functions.
- Coverage 84 % (ARCH-004).

## Benchmarks
- `BenchmarkLightGridLamps` (in the package).

## Known Design Constraints
- `Shadows[F]` keeps a dead type parameter until `render/env/environment.go` drops its `creature.FigurePose` type argument (left for that package's owner).
- The root keeps a trimmed `shadow.go` with the box and blob caster lists.
