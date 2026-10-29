# render/sprite package

## Package Purpose
Draws one world sprite (by key, tint, colour scale and scale), lit through `render/light`, and owns the keys around it: the linked-building grid and its keys (walls, rock, fences, conduits), the stone and stuff tints, the fence sprites and their vector fallback, and the small sprite-key helpers (tile hash, grass tints, two-digit suffixes). A `Painter` reads the frame's `*render.View`, its catalog, its light layer and its own caches, never the renderer. ARCH-004 wave 4.2 (agent R3).

## Owned Concepts
- `Painter` (11 methods) built by `NewPainter(c *art.Catalog, lit *light.Lit)`: `Draw`, `DrawCS`, `DrawScaled` (take `Pass{Cast, Baking, Blood}`), `DrawLinked*`, the stone cache.
- `Pass` (cast, baking, blood flags for one draw).
- `ColorScale` ([4]float32) with `TintCS`, `ItemCS`, `LerpCS`, `MulCS`, `GrassTint`, and the stone and stuff tints (`StoneTint*`, `StuffTint`).
- `LinkGrid[K Kinds]` (16 methods) over `Kinds` (2 methods: `Building`, `Power`) returning `LinkCell`; `LinkFlag`, `LinkedDraw`, `LinkKey`.

## Allowed Dependencies
- Leaves: `art`, `camera`.
- Layers: `render` (the view), `render/light`.
- `application/game` read models (`ItemView`, and the building and plant views).
- Ebitengine (`ebiten`, `vector`); stdlib (`fmt`, `image/color`, `slices`, `strings`).

## Forbidden Dependencies
- The ebiten root package and `cmd/`.
- Feature packages and every other `render/*` package (sprite sits below creature, env, overlay, world).

## Public Contracts
- `Draw`, `DrawCS` and `DrawLinkedSprite` are the per-sprite entry points; the ARCH-004 inline check reported costs 71, 71 and 78 (inlinable); `DrawScaled` did not inline before the move either.
- `LinkGrid` is generic over the link kinds, so overlay and sprite share the grid without an import cycle.

## Invariants
- Walls and rock link to each other, and fences are one-way (`TestWallLinksRock`, `TestRockLinksWall`, `TestFenceOneWay`).
- Every link mask resolves to a sprite (`TestMaskAll16`).
- Grass and plants have no outline (the `art` guard).

## Hot Paths
- `Draw` and `DrawCS` run once per sprite per frame; they must stay inlinable and must not allocate (ARCH-004 gate: allocs/op unchanged, draw calls identical).

## Determinism Rules
- Presentation only. Tile hashes and grass tints are pure functions of the tile coordinates and the biome key; no simulation RNG and no wall clock.

## Save/Mod Impact
- None.

## Tests
- 5 source files, 3 test files, 24 `Test*` functions; 702 lines of tests against 862 lines of source.
- Coverage 88.6 % (ARCH-004).

## Benchmarks
- None in the package.

## Known Design Constraints
- The placement ghost, `buildKey`, the terrain tables, the plant keys and the pawn layers stay in the root for the world pipeline (ARCH-004 wave 4.4).
- ARCH-004 recorded 1,036 source lines for this package; the tree has 862 (the count drifted after a later wave).
