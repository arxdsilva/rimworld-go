# NST-E3: neat storage bulk art

Task: 11 original bulk storage pieces (NST-010, -012, -013, -020..-027) in `assets/source/blender/artlib/families/neatbulk.py`.

- Sprites: 26 world sprites (`furniture/<slug>_<facing>`; `furniture/<slug>` for fixed ones), 11 icons (`ui_icon/<slug>`).
- Rotatable (4 facings): open shelf, narrow tall shelf, tall shelf, half pallet, crate. Fixed: mini pallet, pallet, small crate, large crate, tall crate, corpse dump spot.
- Metadata goes through `render_asset`'s `extra`: `capacity`, `slot_scale`, `attachments.slot<N>`. The emit path needed no change. Slots are for the first cell of the faced footprint, listed north to south so later stacks overlap earlier ones correctly. Dump spot: the 4 drawn heap points come first.
- Slot scales: shelves and crates 0.4, pallets 0.44, dump spot 0.45. The slot is where the item's tile centre lands; the contact sheet assumes that, so WP-D's draw must agree.
- Palette unchanged. Items are never baked in. Dump spot: rag and stakes cast no shadow (`visible_shadow = False`), which on the tarp read as a stain.

Decisions:

- Crate tops are spaced slats nailed over a dark opening, not an open box. Tried and rejected: burlap discs (read as drums), rumpled cloth heaps (read as boulders, so as contents), flat cloth (read as window panes). The slats read as a crate at game size and look the same full or empty. The card text "A lidless box" (NST-023) still holds loosely; the lead may reword it.
- Shelves: tall units step their rows (front board low, back board high) so both rows stay in sight in all facings.

Not done:

- Normal maps: `cmd/normalgen` takes directories only, so the lead runs `make normals atlas art-check` once. Until then the validator reports missing normal maps for these 26 sprites; it reports nothing else for them.
- Not inspected in the real renderer: the slot draw (WP-D) is not in yet. Checked on a contact sheet with sample items, at 64 and 128 px per tile, on light and dark floors.
- ART/§13 checkboxes are the lead's (`.cursor` is out of scope here).

Visual impact: new sprites and icons only; no existing asset changed.
