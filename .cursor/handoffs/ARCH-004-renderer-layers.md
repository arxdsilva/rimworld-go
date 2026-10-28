# ARCH-004: the renderer's draw layers leave the root

## Why
`Renderer` had 398 methods over 64 files and a 74-field struct (ARCH-001 numbers): every draw layer — lighting, environment, creatures, overlays, sound cues, the minimap, the world pipeline — was a method on it, reading whatever renderer field it liked. The layers become packages under `render/`, each one type with a constructor, reading one `*render.View` (this frame's camera, art, world, pawns, clocks and options, filled once at the top of `Renderer.Draw`) and never the renderer (`.cursor/SOLID_GO.md` §7, the plan's §4b). Layers stay concrete — `*render.View` is a struct, `*art.Catalog` and `*light.Lit` are pointers — so the hot paths (lit sprites, the light grid, the grass and crop batches) keep their shape and PERF-002's numbers.

## Order and ownership
Lead-only throughout: `renderer.go`, `render/view.go`, `draw_art.go` until wave 4.4, the budgets, this handoff. Every move: the lead's serial `gopls rename` batch of the members root code reaches, then one agent per package on disjoint files, a root `<pkg>_compat.go` of aliases and Renderer method wrappers, tests moved with their code, then the lead's gate (build, vet, guard, journeys, the whole tree under `-race`, the benchmark set).

| wave | package | agent | status |
|---|---|---|---|
| 4.1 | `render/motion` | R1 (opus) | done — `State[B]`/`Track[B]` (746 + 292 lines, 61.8 %): tracks, the animation clock, `Pose`, `Vec`, `MoveState`, gaits and easing; the type parameter is the root's animal body player, dropped when the creature layer moves; `animState`, particles, bursts and the sprite-pose draws stay in the root `anim.go` for `render/creature` |
| 4.1 | `render/light` | R2 (opus) | done — `Lit` (`Draw`, `DrawGround`, `LightTile`, `LightFoot`, `Sync`, `SourceOf`, `GroundLamp`, `LitShader`, `SyncUniforms`; `Blaze` is the fire-light hook), `Grid`, `Shadows[F]` with its `Casters[F]` interface (the root's figure pose and caster lists stay root in a trimmed `shadow.go`), `Cast.Silhouette`, daylight, the two shaders (2,481 lines, 84 %). `LightGridLamps` rebuild 142–153 µs before, 139–142 µs after on a quiet run; 0 allocs; `Grid.AtF`/`LightTile`/`Lit.Draw` did not inline before either |
| 4.2 | `render/sprite` | | |
| 4.3 | `render/env`, `render/creature`, `render/overlay`, `render/cue`, `render/minimap` | | |
| 4.4 | `render/world` | | |

## Benchmarks
Reference: `.cursor/handoffs/ARCH-004-bench-4.1.txt` (five runs each on the quiet machine after wave 4.1; the ARCH-002 baseline was recorded under load and is discarded). Five-run averages of `DrawArt500` still spread ±40 % between runs (the benchmark's cost is the simulation's own setup: `phaseAnimals` and `madvise` dominate its profile), so a wave's gate is a back-to-back A/B with `-benchtime 3s -count 3` against a worktree of the previous commit, not a comparison of averages: wave 4.1's reads 1.01 ms before vs 0.99 ms after on `DrawArt500/z1`, 418 draw calls and 3,826 allocs in both. Gate per wave: `benchstat` on `DrawArt500/250`, `ChunkLayer500`, `MinimapLayer500`, `LightGridLamps`, `GlobeFrame` (ns/op within noise, allocs/op unchanged, draw-calls/op identical) and `go build -gcflags=-m=2` on `View.WorldToScreenF`, `light.Lit.Draw`, `light.Grid.AtF`, `sprite.Painter.Draw`.

## Findings
- Two layers took a type parameter for a root type they must not import (`motion.State[B]` for the animal body player, `light.Shadows[F]` for the figure pose): sound as a seam, and the creature layer removes both when those types move.
- The map-pixel guard assumed every receiver is a plain identifier; a generic receiver is an `*ast.IndexExpr` (fixed in `world_scale_test.go`).
- `render.View` needed no new field for either layer; the root wrappers pass `r.cast`, `r.figureBlood`, the env values and the tier as parameters.
