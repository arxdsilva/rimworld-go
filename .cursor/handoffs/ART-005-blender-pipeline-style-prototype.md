# Task Handoff

Task ID: ART-005
Status: complete (renderer integration continued in ART-006)

## Summary

Original top-down art now renders from code in Blender (via Blender MCP for
iteration, headless `make art` for reproducible builds). ART Wave 0
contracts are frozen in `assets/source/blender/artlib/contracts.py`, the
material/base/faction palettes exist, and the full §34 vertical slice is
produced: terrain, rocks, autotiled walls, doors, floor, frame, bed/table/
chair/workbench/lamp, trees/crop stages/berry bushes/grass, wood/food/steel/
stone items, a 4-facing layered pawn (body, pants, shirt, vest, 3 hairstyles)
with tint masks, a rifle, overlays, and 12 UI icons. 107 sprites + 12 icons,
validator clean. The renderer still draws the vendor pack.

## Files Changed

- `assets/source/blender/**` (new: build.py, artlib/, template.blend, README.md)
- `assets/source/tools/{validate_assets,contact_sheet,compose_scene}.py` (new)
- `assets/palettes/{base,materials,factions}.yaml` (new)
- `assets/generated/**`, `assets/metadata/**` (generated)
- `assets/AGENTS.md`, `Makefile` (art, art-check, art-preview targets)
- `tools/blender-mcp/` (Blender MCP add-on copy + Claude desktop installer)
- `.cursor/ART_DIRECTION_AND_ASSET_PIPELINE.md` (Wave 0/1/2/3, §23, §33, §34 boxes)

## Public Contracts Changed

None for simulation. New presentation contracts: sprite metadata v1
(`id`, `image`, `scale`, `canvas`, `footprint`, `origin_px`, `attachments`,
`tint{group,mask}`, family extras), wall autotile mask N=1 E=2 S=4 W=8,
pawn layer order, @2x export.

## Invariants / Decisions

- Projection: orthographic top-down with oblique shear (height z drawn 0.5·z
  tiles up). Footprints stay exactly grid-aligned; fronts are visible.
- Category rule (§5): pawns/animals lean 55° toward the viewer about their
  feet so faces and facing read; everything else uses the plain shear.
- Light upper-left; exactly three palette bands per material (toon ramp);
  outline is a 2 px @1x chromatic post-pass, never pure black; contact
  shadow lower-right.
- Pawn apparel/hair render with the body as holdout so any layer subset
  composites correctly.
- Terrain tiles share border lattices so every variant tiles with every
  other variant of the same type.
- Clean-room: all geometry and colours are authored in code; no reference
  images, no asset libraries, no AI 3D generation were used.

## Verification

- `make art` (headless Blender 5.2.2) → 120 outputs in ~16 s.
- `python3 assets/source/tools/validate_assets.py` → 107 sprites, 0 errors.
- Two headless runs differ by at most 1/255 on a few pixels (GPU noise).
- Visual: `assets/generated/contact_sheet.png`, `prototype_scene.png`
  (gameplay zoom), `prototype_scene@2x.png`, `pawn_sheet.png`.
- Not inspected in the real renderer yet; Go tests not run (no Go
  toolchain in the agent shell; no Go code changed).

## Determinism Impact

None. Visual variant choice in the composer is coordinate-hashed; the
renderer must do the same (never simulation RNG).

## Save / Mod Impact

None yet. Metadata IDs are namespaced `core:` so plugin packs can add
their own namespace.

## Art / Visual Impact

New original visual identity: warm earth terrain, wood construction with
rimmed wall tops, flat three-band shading with chromatic outlines, compact
big-headed colonists whose skin/hair/clothes are runtime-tinted.

## Screen / UI Impact

12 icons generated (`generated/ui/icons`); no screen uses them yet.

## Known Limitations / Next

- Phase 3: Go loader for metadata + tinted sprite cache in the ebiten
  adapter; replace vendor pawns/ground/rectangles; wall autotiling;
  in-game screenshots on light/dark terrain; Go port of the validator.
- Terrain transitions between types, water, snow are missing.
- Only wood walls/doors/floor; no 2×2/3×N buildings, animals, decals.
- No walk/downed/sleeping presentation; renderer should add bob/rotation.
- §34 visual identity signed off by the owner (2026-10-05); in-renderer reproduction still pending.
