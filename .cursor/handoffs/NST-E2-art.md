# NST-E2: neat storage racks (art)

- `assets/source/blender/artlib/families/neatracks.py`: 9 pieces, 30 world sprites, 9 icons (`ui_icon/<slug>`). Original forms: pegged wood weapon rack; stepped steel rack (lays flat); tall steel rack with floor sockets and overhead combs (on end); narrow wardrobe and wardrobe with doors swung open, rod and empty hangers; two-rod textile frame; hook gantry over a drip tray; strapped safe with a wheel; ribbed drum with a yellow placard.
- Metadata goes through `power.render`'s `extra` and `after`, so no writer change: `attachments.slot0..N` (canvas px, first cell of the faced footprint), `slot_scale`, `capacity`. A slot is the point the stack's centre sits on. Slot scale follows the sibling families: 0.42 for 3 and 4 slots, 0.36 to 0.4 for 6.
- A dark plate along the faced footprint's north edge fills pockets the shear leaves behind tall frames; otherwise the contour line fills them. The meat hooks are built after facing so their curl shows in all four facings.
- Palette: no new entries. `corn_yellow` (drum placard) and `fur_black` are existing materials.
- Deviation to settle: the renderer draws item sprites unrotated, so the vertical rack shows weapons level unless WP-D rotates them there. No metadata key was added for it.
- A steel-built weapon rack or textile rack draws as wood, like the shelf (`StuffTint` is zero for steel).
- Not done: normals (`cmd/normalgen` takes directories only, lead runs `make normals atlas`), so `validate_assets.py` reports missing normal maps for these sprites and nothing else about them. Not inspected in the real renderer: WP-D's slot drawing does not exist yet; checked on a Pillow contact sheet at game size and @2x on three floors, sample items on the slots.
- HUD/UI: art only; the Architect cards are WP-C. No checklist box changed (lead owns those for NST).
