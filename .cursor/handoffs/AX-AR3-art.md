# AX-AR3: A6/A7 art (recreation, books, bookcases)

AX wave 2, 2026-11-02, clean-room original art. Run with `make art FAMILIES="recreation library"`.

## Files
- `assets/source/blender/artlib/families/recreation.py` (new): the A6 pieces and their icons.
- `assets/source/blender/artlib/families/library.py` (new): the books, both bookcases and their icons.
- `assets/palettes/materials_ax_ar3.yaml` (new): `felt`, `felt_dark`, `felt_wine`, the `book_*` covers, `page_cream`, `schematic_blue`/`schematic_line`, and the emissives `tv_screen_glow` and `tv_picture_glow`.
- `assets/source/blender/build.py`: adds `recreation` and `library` to FAMILIES.
- Generated: 34 recreation sprites (12 of them glow layers), 18 library sprites, 14 icons, plus metadata and normal maps.
- `.cursor/ART_DIRECTION_AND_ASSET_PIPELINE.md`:
  - wave 2 "basic resources/items" and wave 4 "furniture families" rows extended; both stay `[-]` until the in-game review;
  - new §41 lists every key.

## Keys (all relayed to the lead)
### Recreation
- `furniture/ring_toss`, `ur_board`, `chess_table`, `poker_table`.
- `furniture/billiards_table_<south|east>`: use `axisFacing`.
- Televisions, each with a `_glow_<facing>` layer (emissive `tv_screen_glow`):
  - `furniture/tube_television_<facing>`;
  - `furniture/flatscreen_television_<facing>`;
  - `furniture/big_screen_television_<facing>`.
- `furniture/telescope_<facing>`.

### Library
- `item/novel`, `item/textbook`, `item/schematic`.
- `furniture/bookcase_<facing>` is empty, with `bookcase_half_<facing>` and `bookcase_full_<facing>`. Suggested switch: 0 / 1..5 / 6+ books.
- `furniture/small_bookcase`, `small_bookcase_half` and `small_bookcase_full`. Suggested switch: 0 / 1..2 / 3+ books.

### Icons
`ui_icon/` plus `ring_toss`, `ur_board`, `chess_table`, `billiards_table`, `poker_table`, `tube_television`, `flatscreen_television`, `big_screen_television`, `telescope`, `bookcase`, `small_bookcase`, `novel`, `textbook` and `schematic`.

## Decisions
- **Glow in every facing:** the shear camera only shows top and south faces, so each television has a status lamp on its top. That lamp gives every facing an on state.
  - The flat panels tilt back 12° (flatscreen) and 9° (big screen), so a sliver of lit screen also shows when they face east or west.
- **Bookcase top:** the case is open on top, so the fill shows from above in every facing, not only through a south-facing front.
- **Felts:** they keep their colour, green for billiards and wine for poker. Stuff tint is the renderer's whole-sprite scale, as for the tables, so no `cloth` mask is used.
- **No quality tints for books:** quality stays an inspector label. Drawing one tint per tier would add 21 near-duplicate sprites (§35).
- **Small bookcase:** not rotatable, matching its Def.

## Verification
- The validator reports 6753 sprites, 0 errors, and the pawn atlas passes.
- Review sheets are in the scratchpad (`ar3/*.png`), on light and dark ground. Lit and unlit TVs and every facing were checked.
- `make art-check` passes in full: validator, pawn atlas, orphans with no new allow lines, plant outlines and the audio manifest. The word literals from the AX-0 names reach every key.

## Open
- **Renderer wiring:** A6 and A7 add `build_keys.go` cases and the bookcase fill switch. Until then, the keys stay unread.
- **In-game review:** pending until the keys are wired.
- **No HUD/UI of its own:** this task is art only, so the owners' renderer and pane work is the player-facing representation.
