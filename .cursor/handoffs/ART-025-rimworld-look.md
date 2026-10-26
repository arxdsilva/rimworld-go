# ART-025: RimWorld-style look (camera, outlines, palette, limbless pawns)

## Why
User decision (2026-10-26): redo the art direction so it is very close to
RimWorld's readable top-down look. This copies style only; it is
clean-room, with no RimWorld art traced, sampled or referenced. The UI
chrome part shipped as ART-024 and the storyteller busts as ART-023.

## Contracts (`artlib/contracts.py`)
Picked from a 3×3 prototype grid (shear 0.30/0.35/0.40 × lean 75/82/90):

| Constant | Old | New |
|---|---|---|
| `SHEAR_K` | 0.5 | 0.35 |
| `FIGURE_LEAN_DEG` | 55 | 82 |
| `ANIMAL_LEAN_DEG` | 40, hard-coded twice | 70, now a contract |
| `OUTLINE_PX` / `DARKEN` / `SATURATE` | 4 / 0.32 / 1.25 | 6 / 0.18 / 1.05 |
| `RAMP_INTERP` | – | `EASE` (± `RAMP_SOFT` 0.06), so toon bands get soft edges |
| `SHADOW_T` / `LIGHT_T` | 0.38 / 0.86 | 0.30 / 0.80 |
| `OUTLINE_INNER_PX` | – | 2 |

- `OUTLINE_INNER_PX` uses a figure holdout pass so that hair and hat outlines and pants outlines don't spill onto the face or body.
- Env overrides for scratch renders: `RWG_OUT_ROOT` and `RWG_PALETTE_DIR`.

## Palettes
`tools/palette_mute.py --contrast 0.8 --outline 0.18,3`:
- saturation ×0.8 and light/shadow ×0.8 toward base;
- light band value ×0.96;
- shadow hue up to 8° warmer;
- emissive entries and neutral tint greys skipped;
- a header marker so it can't be applied twice.

## Pawns (`families/pawn.py`, 1061 → about 480 lines)
- Round head on a body capsule, with dot eyes. No arms, legs or hands; RimWorld draws none either, and test hand dots read as flails.
- The 6 builds are kept as capsule widths. Apparel is thin shells. 8 hair styles, 3 beards, the tuque and the face scar are kept; the east-hair notch is fixed.
- South, east and north are rendered; west is a mirrored east. There are no frames.
- Body attachments: `head`, `hold`, `back`.
- 147 sprites (was 40,472, about 470 MB); the atlas is 1 page.

## Renderer (`internal/adapters/inbound/ebiten`)
- `figureLayers(art, Look, facing, dress)` serves the colony, portraits and the colonist bar, so builds and beards now show in the colony.
- `facingKey` mirrors west for pawns, held items, the face scar, shadows and animals. The lit shader inverts the normal X when a sprite is mirrored (`TestMirroredDrawInvertsNormalX`).
- All frame machinery is gone. Motion is now transforms:
  - walk bob, sway and lean;
  - per-action tool arcs, lunges, recoil, lean and dip;
  - a buckle followed by a rotation when falling.
  - Strike timing still lands at trace age 0.
- Items grip on `hold`; slung items hang on `back`; items are drawn behind the body facing north.
- Pivots, crops, the lying shift and marker heights come from body metadata.
- `projShearK = 0.35` (`light.go`) matches the metadata (`TestProjShearMatchesMetadata`). `sunElevation` uses it.

## Re-rendered
- pawn, animal (south/east/north only, canvases centred on the tile, corpses included);
- structure, furniture, power (fence writes into structure);
- terrain, rock, item, weapon, tool, effect;
- plant, trees (still no outline), overlay, weather, the UI icons (on a grey `#303236` disc);
- the menu diorama, where plants are excluded from the scene outline.

`grave_dug_*` was already clipped before this pass; not fixed.

## Verify
- `validate_assets.py`: 5367 sprites, 0 errors. `atlasgen -check` passes.
- Full ebiten `-race` package: passes after `world_labels_test` was re-derived from metadata.
- `BenchmarkDrawArt500` at z1: 16.9 → 17.2 ms, inside the 10 % budget.
- Captures in the session scratchpad (`look/before`, `look/b2`, `look/protoA`).

## Known follow-ups
- The muted palette brings wall tops, rich soil and mud close in value, so walls rely on the outline and the darker south face.
- Odd `origin_px` y values put sprites on half pixels at exactly 1x zoom. This predates the pass and is minor.
- Long-legged animals look narrow facing south or north at lean 70.
- Normals are not rotated for rotated (lying) sprites. This predates the pass.
