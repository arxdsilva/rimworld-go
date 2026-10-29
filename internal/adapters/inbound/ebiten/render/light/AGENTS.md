# render/light package

## Package Purpose
The renderer's sun, moon and lamp lighting: the light grid (sky exposure and lamp light per tile), the lit-sprite shader (`lit.kage`) and its pair images, the light sources, contact and cast shadows with their blur, daylight and the no-art sky overlay. It reads the frame's `*render.View` and its own state, never the renderer. The shadow pass asks the renderer for its casters and figure silhouettes through `Casters`. ARCH-004 wave 4.1 (agent R2).

## Owned Concepts
- `Lit` (9 methods): `Draw`, `DrawGround`, `LightTile`, `LightFoot`, `Sync`, `SourceOf`, `GroundLamp`, `LitShader`, `SyncUniforms`. `Blaze` is the fire-light hook.
- `Grid` (13 methods): sky exposure and lamp light per tile; `AtF` is the float lookup.
- `Shadows[F]` (9 methods) with `Casters[F]` (5 methods: `Objects`, `Creatures`, `CastFigure`, `VisibleTiles`, `White`); `Cast` (`Silhouette`), `Caster`, `Creature[F]`.
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
- `Casters[F]` is the only path from the shadow pass to the renderer.

## Invariants
- Lamps are brighter near than far in a room (`TestLampNearBrighterThanFarInRoom`); night is darker than noon (`TestSkyOverlayNightDarkerThanNoon`); day overlay is clear (`TestSkyOverlayDayIsClear`).
- Lights come only from light sources (`TestLightsComeOnlyFromLightSources`, a root guard).
- `Shadows[F]` takes a type parameter for the figure pose; the creature layer resolves it.

## Hot Paths
- `Lit.Draw` runs once per lit sprite; `LightGridLamps` rebuilds the grid's lamp light. `BenchmarkLightGridLamps` measures it: 142–153 µs before, 139–142 µs after on a quiet run, 0 allocs (ARCH-004).

## Determinism Rules
- Presentation only. Light values are functions of the view's clock, weather, roof and room read models. No RNG; no wall clock.

## Save/Mod Impact
- None. Shaders are embedded.

## Tests
- 6 source files, 7 test files, 54 `Test*` functions.
- Coverage 84 % (ARCH-004).

## Benchmarks
- `BenchmarkLightGridLamps` (in the package).

## Known Design Constraints
- `Shadows[F]` is generic over the figure pose only because the root's figure type cannot be imported; this is moved debt that the creature layer removes.
- The root keeps a trimmed `shadow.go` with the figure pose and caster lists.
