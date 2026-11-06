# ART-015 Linked walls and rock: baked corners, stone tints, ore, smooth rock

## What changed
- **Linked families** (`families/linked.py` `_variants`) bake inner corners: 47 sprites per family, `<prefix>_<NN>` and `<prefix>_<NN>_c<h>`.
  - `h` is the hex of the filled corners: NE=1, SE=2, SW=4, NW=8.
  - A corner is filled when both of its cardinals and its diagonal link, which is the corner-filler rule from the rendering reference.
  - An unfilled concave corner is notched, with a wrapping rim or edge lumps, so it lines up with the neighbours' free edges. That removes the old rimless nub.
- **The eight families:**
  - `rock/rock_wall`, `rock/rock_smooth`
  - `rock/ore_{steel,silver,gold}`
  - `structure/wall_{wood,stone,steel}`
- **Neutral stone.** Rock, smoothed rock, stone walls, fences and floors, stone terrain and edges, and the `stone_chunk`/`stone_blocks` items are all drawn in neutral greys (`stone_neutral*`). The renderer multiplies in the stone tint.
- **Ore sprites** put their rock body in the masked `stone_tint` material and keep their metal flecks unmasked.
- **New sprites:**
  - steel doors
  - `terrain/stone_smooth_00..03` with edges
  - `item/stone_chunk`, `item/silver`, `item/gold`
  - `furniture/stonecutter_table_*`
  - `overlay/designation_smooth`
  - `ui_icon/order_smooth`, `ui_icon/stonecutter_table`
- **`cmd/normalgen`** extends an autotiled sprite's linked edges (read from metadata `autotile` + `mask`) before beveling, so joined tiles light as one surface instead of a grid. The written map keeps the sprite's own alpha.
- **Seamless rock.** Every lump touching a linked edge is centred on that edge. Each edge carries two low lumps, at 0.28 and 0.74 along it, with seeds fixed per axis and position. Filled-corner lumps are centred on the corner. Neighbouring tiles therefore render the same blob, cut in half, and meet without a seam. Interior lumps stay inside the tile and vary with (mask, corners).
- **Ore** shows as chunky metal crystals with dark rims, set on the top surface.
- **Validator and preview tools** read the tint groups from `palettes/base.yaml`.
- **Palette:**
  - `materials.yaml` gains `stone_neutral`, `stone_neutral_dark`, `stone_tint`, `silver`, `gold`.
  - `base.yaml` gains `tints.stone_tint`, set to the doc's RGB values for sandstone, granite, limestone, slate and marble. The user chose these values explicitly; they come from the user-supplied rendering reference.
- **Capture scene:** `-scenario walls`, saved by `make art-capture` as `ingame_walls.png`.

## Verification
- `make art`, `make normals`, and `validate_assets.py`: 0 errors. The validator needs a Python with Pillow.
- `go test ./cmd/normalgen`: `TestLinkedEdgeHasNoBevel`, `TestLinkedMaskFromMetadata`.
- `TestGeneratedArtCoversRenderer` covers every linked key and the new singles.
- Visual review of the walls capture at zoom 0.5 and 1.2:
  - no concave nub;
  - continuous rock masses;
  - distinct per-stone tints;
  - walls joining rock.

## Known limitations
- Doors link as walls (repo convention); the reference treats doors as separate.
- No mountain-roof or fog-of-war art.
