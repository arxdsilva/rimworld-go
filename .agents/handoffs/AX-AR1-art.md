# AX-AR1: A2/A3 art (decor, furniture, walls, floors)

AX wave 1, 2026-11-01, clean-room original art. Run with `make art FAMILIES="decor flooring"`.

## Files
- `assets/source/blender/artlib/families/decor.py` (new): the A2 pieces, the extra wall materials, the double sleeping spot and the icons.
- `assets/source/blender/artlib/families/flooring.py` (new): the A3 floors and their icons.
- `assets/palettes/materials_decor.yaml` (new): jade, plasteel and uranium walls, silver and gold trims, concrete, pavers, grout, sterile tile, carpet neutrals, clay, and the `flood_white` emissive.
- `assets/source/blender/build.py`: adds `decor` and `flooring` to FAMILIES.
- Generated output: about 300 world sprites with metadata and normal maps, 18 floors (lit flat, no normal map) and 29 icons.
- `.cursor/ART_DIRECTION_AND_ASSET_PIPELINE.md`:
  - Wave 4 rows "material variants" and "furniture families" are now `[-]`; what's missing is the in-game review.
  - New §40 lists every key.

## Keys (all relayed to the lead)
### Structure
- `structure/ornate_door_<ew|ns>_<open|closed>` (2x1/1x2).
- `structure/animal_flap_<ew|ns>_<open|closed>`, with a `cloth` mask.
- `structure/wall_<plasteel|uranium|silver|jade|gold>_*`, 47 linked sprites each.

### Overlay
- `overlay/double_sleeping_spot_<facing>` (2x2).

### Furniture
- Tables:
  - `table_1x2_<south|east>` and `table_2x4_<south|east>`; north reuses south and west reuses east.
  - `table_2x2` and `table_3x3`.
- `couch_<facing>`: `cloth` mask, with `seat` and `seat2` in tiles.
- `small_shelf_<facing>`.
- `plant_pot`: the `plant` attachment is in canvas px.
- `animal_sleeping_box`, `egg_box`.
- `column`, `urn`, `large_stele`, `grand_stele`: `stone_tint` masks, so the renderer must add them to `stoneMaskKind`.
- `wall_torch_<facing>`: `fire` attachment and `flame_scale` 0.4.
- `wall_lamp_<facing>` and `wall_lamp_glow_<facing>`.
- `flood_light` and `flood_light_glow`.

### Floors
- `structure/floor_<straw_matting|concrete|paved_tile|steel_tile|silver_tile|gold_tile|sterile_tile>`.
- `structure/floor_stone_tile_<stone>` and `structure/floor_flagstone_<stone>`, for granite, limestone, marble, sandstone and slate. Each stone has its own look, and the renderer multiplies its tint like `floor_stone`.
- `structure/floor_carpet`: one neutral texture. The renderer multiplies the carpet colour over the whole tile as a colour scale, so all 63 colours share it.

### Icons
`ui_icon/<piece>` for each piece, plus `straw_matting`, `concrete`, `paved_tile`, `steel_tile`, `silver_tile`, `gold_tile`, `sterile_tile`, `stone_tile`, `flagstone` and `carpet_floor`.

## Decisions
- **Wall-mounted fixtures:** each is drawn on its own tile, hung at the edge toward the wall behind it (`axview` WallMounted).
- **North-facing fixtures:** the wall to their south, drawn later, hides the fixture.
- **Carpet:** one tintable texture rather than one per colour (§35, tint over duplicates).
- **Wall materials:** silver and gold walls are new too, because silver and gold currently fall back to the wood wall art.

## Verification
- `make art-check` passes: the validator reports 6641 sprites, 0 errors. Pawn atlas, orphans, plant outlines and the audio manifest also pass.
- The orphan check already passes, because word literals from the AX-0 names reach the keys. No orphan_allow lines were added.
- Review sheets were built in the scratchpad. Floors are tiled 2x2, tinted and seamless.

## Open
- **Renderer wiring:** A2/A3 must wire the keys:
  - `WallPrefix` for the new wall materials;
  - `stoneMaskKind` for the column, urn and steles;
  - carpet colour as a colour scale;
  - stone tile and flagstone tinted by stone.
- **In-game review:** pending until the keys are wired, so both ART rows stay `[-]`.
- **Retire once the migrations land:** `furniture/table_small` and `structure/floor_stone`, plus their icon sources in `overlay.py` ICONS (`table`, and `floor` if it changes).
- **No HUD/UI of its own:** this task is art only, so the owners' renderer work is the player-facing representation.
