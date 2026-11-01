# Creatures cast no shadow (agent SH)

The user decided on 2026-10-31 that creatures (pawns, animals, mechanoids) cast no shadow at all. This retires ART-012's silhouettes, point-light shadows and contact pools. Buildings, rock, plants and items keep their box and blob shadows, and the Options Shadows toggle still covers them.

## What changed
- `render/light`: `shadow_cast.go` is deleted (projection, `Plan`/`PlanCreature`, `PointLight`s, `castCreature`, `creatureLights`, the contact batch, the cast shader). `Grid.lampReaches` and `Rect` are deleted too. `Shadows.Draw(v, lit, host, screen, w, sw, sh, tile)` draws only boxes and blobs and returns early when nothing casts. `Casters` is no longer generic: it has `Objects`, `VisibleTiles` and `White`. `Cast` is now an empty bake marker. `shadow_blur.kage` lost the `R1` height term and its blocker search, so it blurs by `R0` only (`BlurRadius`). `Lit.Draw` and `DrawGround` lost their `cast` parameter.
- `render/creature`: every `pass.Cast` branch is gone (pawn, animal, anim cull, carried ease), as is `FigureLying`.
- `render/world`: `Shared.Cast` became `base.Cast`, set only by `WithBake`. Water's baked-frame flag now reads `LOD.Baking`.
- Root: the `shadowHost.CastFigure`/`Creatures` methods and `Renderer.cast` are deleted.
- `art`: `Sprite.OpaqueHeightPx` is deleted, because only the creature shadow κ used it. `OpaqueBox` remains.

## Tests
- `TestCreaturesCastNoShadow` (root) runs pawns and animals with art at noon sun, low sun, and moonlight with lamps (outdoors and in a roofed room). Creatures alone produce no layer and no composite. Next to a wall and lamps, the pass gets the same casters and composites with or without creatures, and no figure gets a lit draw.
- `render/light/shadow_pass_test.go` is rewritten without creatures. Deleted: the root `shadow_cast_test.go`, `render/light/shadow_cast_test.go` and `BenchmarkCreatureShadows`. `keyOf` and `figureArt` moved to `anim_test.go`.

## Open
- (Done by the lead) The dead `Shadows[F]` type parameter is gone (`light.Shadows` everywhere).
- (Done) Pawn and animal sprites no longer carry `shadow:` blocks. `stage.NO_SHADOW_FAMILIES` skips them, `validate_assets.py` rejects one, and the 4014 existing blocks were stripped by a script that matches the pipeline's new output (the images are unchanged, so no re-render was needed).
- Bench A/B, `DrawArt500/z1` with -count=3 interleaved twice, HEAD against HEAD plus this patch: 414 draw calls in both, allocs 3798 → 3749, ns/op 1.02–1.17 ms → 1.01–1.05 ms.
- No HUD change: removing the shadows is itself the visible change.
