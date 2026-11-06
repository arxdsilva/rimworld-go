# STK-SH: only colonists outdoors cast a shadow (agent SH)

On 2026-11-01 the user asked to "remove shadows from anything that isn't a colonist or inside rooms". The result: only colonists (the player faction's humanlike pawns) cast a shadow, and only while they stand on a tile with no built or natural roof. Nothing else casts one: buildings, walls, rock, plants and trees, items, roofs (eaves), bridges, animals, mechanoids, raiders, visitors, allies, guests and prisoners. This replaces ART-010's box and blob casters and finishes what SHADOW-creatures-cast-none (2026-10-31) started.

## What changed
- **`render/light/shadow.go`** is rewritten as the colonist pass.
  - `Colonist(pw)` decides who counts: not Hostile, Prisoner, Visitor or Ally, no GuestFaction, not Embodied, not carried. `Casts(pw, w)` adds the roof check: no `roof.Built` or `roof.Natural` on the tile. A planned roof is not a roof yet.
  - `Foot` holds the ground pivot and scale. `ColonistQuad` builds the blob: 0.62 tiles across and 0.34 deep, offset and stretched along `ShadowDir` by `min(ShadowLen(1), 1.2)` tiles.
  - `ColonistAlpha` is `ShadowAlpha × 0.8 × (0.5 + 0.5·el/el_max)`. The blob is dark at noon, faint at dawn and dusk, a faint blob under the moon, and absent when no light is up.
  - `Shadows.Draw(v, light, screen, feet, dayOfYear)` sends every blob in one `TriBatch` draw straight to the screen. A procedural ellipse texture is made once; there is no layer, no blur and nothing allocated per colonist. `SetOff` is the Options toggle.
- **Deleted from `render/light`:** `shadow_blur.kage`, `Caster`/`Casters`, `ShadowSpec`/`ShadowKind`, `ShadowSpecFor`/`ShadowFallback`/`SpecOf`, `BoxHull`/`AddBox`/`BlobQuad`/`BoxFootprint`, `RoofRuns`, `ShadowBlend`, `ShadowCompositeScale`, `ShadowLayerScale`, `BlurRadius`, `ShadowMaxLen`, and the layer, scratch and blur state.
- **`render/world`:** the new `Pipeline.ShadowFeet` lists the colonists in view where they are drawn (interpolated motion plus `creature.PawnFeetX/Y`, scaled with `CreatureAt` when zoomed far out). `DrawArt` draws them under every object, but not on the whole-map tier. `Hosts.Casters` is gone.
- **Root:** `shadow.go` (the box and blob caster lists) is deleted, and `layers.go` no longer passes a caster host.
- **`render/overlay`:** the eave shadows are gone (`DrawEaves`, `EaveEdges`, `EaveAlpha`, `EaveWidth`, `RoofDrawState.Edges`), and so are the bridge water shadows (`BridgeShadowSides`, `BridgeShadowRects`, `bridgeShadowClr`).
- **`render/env`:** `DamageShadowFactor` is deleted.
- **`art`:** `Meta.Shadow` and the parsing of `shadow:` are deleted.
- **Options:** the Shadows row now reads "Soft shadows under colonists standing outdoors."

## Assets and pipeline
- Deleted: `overlay/shadow_blob` and `overlay/shadow_box`, each with its png, `_n.png` and yaml.
- Stripped all 2438 `shadow:` blocks from 2097 metadata files, `atlas/pawn.yaml` included, plus the inline line in the 6 `ui_portrait` files. The images are unchanged.
- The pipeline no longer writes the blocks:
  - `stage.py`: `_shadow_meta`, `SHADOW_KINDS`, `NO_SHADOW_FAMILIES`, `FLAT_H`, `figure_lean` and the `shadow_kind` parameter are deleted. Their callers are updated: `power.render`, `fortify`, `defense`, `fence`, `shippart`, `trees` and `furniture`.
  - `meta.write`: the default block is gone.
  - Hand-written blocks are removed from `overlay`, `structure`, `effect`, `power`, `climate` and `portrait_cut.py`.
- `validate_assets.py` rejects any `shadow:` block. Results: 6214 sprites, 0 errors (`/usr/local/bin/python3` has Pillow). `atlasgen -family pawn -check` is ok, and `TestNoOrphanSprites` passes.

## Tests
- `TestOnlyColonistsOutdoorsCastShadows` (root) replaces `TestCreaturesCastNoShadow` and runs whole `DrawArt` frames at noon sun, low sun and moon:
  - a colonist outdoors casts one shadow, under its feet;
  - the same colonist in a roofed room casts none;
  - a prisoner, a raider and an animal, standing among walls, rock, a tree, items, a bridge and a lamp, cast none and trigger no draw.
- `TestJourneyColonistShadowAndToggle` (`journey_shadow_test.go`): one shadow at 08:00. Options › Graphics › Shadows turns it off and then on again.
- `render/light`:
  - `TestColonistShadowFollowsTheSun`, `TestColonistShadowFadesWithTheSun` and `TestOnlyColonistsOutdoorsCast` (the full predicate table);
  - `TestColonistShadowPass`: one draw, texture reused, no allocation per colonist;
  - kept: `TestSunElevationFromLight` and `TestShadowTintBluerAtLowSun`.
- `TestShadowsOffSkipsTheLayer` is rewritten. `BenchmarkShadows` is now the colonist pass; `BenchmarkShadowsHiDPI` is deleted with the half-size layer.
- Deleted with the code: the eave tests, the bridge-shadow tests, `TestDamageShadowFactor`, the box/blob/roof-run/layer tests and the corpse-shadow term in the VisPad test.

## Bench (DrawArt500/z1, HEAD worktree vs this tree, -count=3 interleaved twice, no -race)
- HEAD: 1.11 to 1.30 ms/op, 411 draw calls, about 925 KB/op, 3909 allocs/op.
- This tree: 0.92 to 1.05 ms/op (one outlier at 1.38), 398 draw calls, about 580 KB/op, 3465 allocs/op. The tree also carries the other STK agents' in-flight changes, so not every point of the difference is this change.

## Notes
- No HUD change is needed beyond the drawn blob, which is the player-visible representation, and the Options label.
- A released prisoner walking home has no Prisoner flag. `GuestFaction` keeps them out of `Colonist`. A `PawnView.Colonist` field would be cleaner if one is ever added.
- `render/light` lost about 550 lines, so the lead may lower its budget lines.
