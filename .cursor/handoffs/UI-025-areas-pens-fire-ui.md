# Task Handoff

Task ID: UI-025 (PLAY-015 UI)
Status: complete

## Summary
**Areas:**
- the manager modal: swatch, rename, Invert, Delete with confirm, New (off at 10), Close;
- Architect "Expand/Clear allowed area >" submenus list the areas plus Manage;
- the tinted overlay for the armed or hovered area (row runs rebuilt on `Ver`);
- the Schedule tab "Allowed area" dropdown, and the pawn inspector "Area:" line;
- the Assign tab lost its Area column.

**Animals tab:** Master dropdown, Follow toggle, Area/Pen column.

**Pen marker inspector:** Animals, Food and Auto-cut. The pen outline shows when selected, and open pens are dashed red.

**Home:** the "Auto" toggle; the Zone tools renamed or moved.

**Fire:**
- flames by size (8 fps frames, per-cell offset), embers, smoke and orange light;
- one positioned `amb.fire` per cluster;
- the flash, `sfx.lightning_strike` and thunder from sim strikes;
- the Extinguish entry in the right-click menu;
- the Work tab Firefight column;
- the dev "Start fire here";
- ash decals.

**Snow:** the per-cell veil and the cyan snow-clear overlay.

## Files
- ebiten:
  - `area_menu.go`, `area_overlay.go`, `area_select.go`, `areas_ui.go`;
  - `pen_ui.go`, `pen_draw.go`, `animals_ui.go`;
  - `home_auto_ui.go`, `fire_world_draw.go`, `snow_veil.go`, `snow_clear_draw.go`;
  - hooks across `archtools.go`, `hud.go`, `draw_art.go`, `light.go`, `sound_director.go`, `environment.go`, `orders.go`, `work.go`, `devmode.go`, `trough_ui.go` (herd from marker pens).
- Art (ART-022): `effect/fire_*`, `overlay/ash_*`, `structure/pen_marker`, the six new icons (`materials_play015.yaml`).
- Sounds: `amb.fire`, `sfx.lightning_strike`.

## Notes
- The submenus and dropdowns are small centred modals (there is no popup primitive).
- The sound budget is now 19.7 of 20 MB.
