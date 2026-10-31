# TEMP art: climate buildings and layered clothing (agent AR, 2026-10-31)

- Source: `assets/source/blender/artlib/families/climate.py` (passive cooler, vent) and
  `apparel.py` (worn layers, items, icons), registered in `build.py`; validator layer lists in
  `assets/source/tools/validate_assets.py`. Built with `make art FAMILIES="climate apparel"`
  (normals and the pawn atlas regenerated). Key list: ART spec §39.
- Keys: `furniture/passive_cooler`, `furniture/passive_cooler_empty`,
  `furniture/vent_<open|closed>_<facing>`; `pawn/button_down_*`, `pawn/jacket_*`,
  `pawn/tribalwear_*` (six builds, masks), `pawn/cowboy_hat_<facing>`, `pawn/bowler_hat_<facing>`;
  `item/` and `ui_icon/` `tshirt, button_down, pants, jacket, tribalwear, cowboy_hat, bowler_hat`;
  `ui_icon/passive_cooler`, `ui_icon/vent`.
- Visual impact: the jacket sits inside the parka and duster shells (no bleed, checked in
  composites in every facing); every garment tints by stuff through its `cloth` mask; icons carry
  a `tints.cloth` preset because the HUD draws icons untinted.
- Pending wiring (other agents): renderer keys for all of the above. Until `item/tshirt` is named
  in Go, `TestNoOrphanSprites` reports the tshirt item and icon. Not yet inspected in the running
  game (needs the wiring).
