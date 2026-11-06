# NST-E1 art: display storage furniture

**Scope:** the 7 display pieces of `NEAT_STORAGE_GAP_ANALYSIS.md` (NST-001..004, 008, 009, 011), in `assets/source/blender/artlib/families/neatdisplay.py`, with generated sprites, normal maps, metadata and icons.

**Sprites:** 25 world sprites (6 rotatable pieces x 4 facings, the fixed pedestal) and 7 `ui_icon/<slug>`. Slugs: `display_case`, `round_pedestal`, `small_display_shelf`, `display_shelf`, `two_tier_display_shelf`, `narrow_display_cabinet`, `display_cabinet`.

**Visual impact:** new furniture only; no existing sprite changed.
- Frames are `wood`, `wood_plank` and `bark`. Boards are planks like the shelf's.
- Open shelves and cabinets share a dark crested back. The case and cabinets share a `felt_wine` lining. Small accents are `brass` and `snow`. No palette entry was added.
- The pipeline has no alpha for glass. The lining stays in view through an open top with `glass` streaks across it, and only the upright front pane is solid `glass`. Items draw over the sprite, so they read as behind the glass.
- The stuff colour shift multiplies the whole sprite (`objects.go:119` `TintCS`), so stone stuff also tints the glass and felt, as it tints everything on the shelf. Checked with a simulated granite and marble multiply: it stays readable.

**Metadata:** `capacity`, `slot_scale` and `attachments.slot<N>` go through the existing `extra` and `power.render(after=...)` path; `meta.py` needed no change. Slots are canvas px for the footprint's first cell (top left).
- Each facing's slots turn about the cell's own centre, not the footprint's. That keeps them right for the first cell of a turned 2x1.
- Slots are numbered north to south, then west to east, in each facing, so later stacks overlap earlier ones correctly.
- `slot_scale`: case 0.72, pedestal 0.55, shelves 0.42, two tier 0.36, cabinets 0.4.
- The renderer should put the item's tile centre on the slot (as `drawPotPlant` does with `0.5*k`); the contact sheet assumes this.

**Normals:** done for these 25 only. `-dirs` of `cmd/normalgen` accepts single files because `WalkDir` does: `go run ./cmd/normalgen -dirs <file>,<file>,...`.

**Verified:**
- Script check of all 25: slot count equals `capacity`, `slot_scale` present, slot x inside the first cell, sprite and icon exist.
- `validate_assets.py` reports no error for these slugs. Its errors at last run were for other families' sprites still in progress.
- Contact sheet at native and 2x, with sample items, on three floors, beside the shelf and sibling families.

**Not verified:** not seen in the real renderer, because the slot draw (WP-D) was not available to this agent. Not listed in `ART_DIRECTION` §13 or any checklist; the task forbids `.cursor` edits beyond this handoff, so the lead updates those.

**HUD/UI:** art only. Architect cards are WP-C, slot drawing is WP-D.
