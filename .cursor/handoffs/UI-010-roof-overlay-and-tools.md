# UI-010 Roof overlay and roof tools

## What changed
- **Architect (Structure):** two area tools.
  - **"Build roof"** emits `DesignateRoof`, with icon `roof_build`. Description: "Colonists roof these tiles. A roof needs a wall within 6 tiles; closed rooms are roofed automatically."
  - **"Remove roof"** emits `DesignateRemoveRoof`, with icon `roof_remove`. Description: "Take roofs down and keep them off, even in closed rooms."
  - While either is held, the Roof overlay is forced on (`roofOverlayForced`, synced each frame and on select, deselect and close).
  - The Build roof drag ghost marks unsupported tiles in red with "×" (`drawRoofUnsupported`).
- **Roof overlay:** R key, or the HUD "Roof" button in the top strip left of the clock (icon `overlay_roof`, click id `hud.roof` in `applyClick`). Styles:

  | Roof | Style |
  |---|---|
  | Built | blue, with a lighter edge facing unroofed tiles |
  | Natural | slate, stippled |
  | PlanBuild | amber dashes |
  | PlanRemove | red hatch |

  While the overlay is on, a legend tab sits under the top bar: "Roof: blue built · slate natural · dashed planned · hatched removal".
- **Hover panel:** a roof line. In priority order: "Roof removal planned", "Roof planned", "Natural roof (rock)", "Roofed", "Unroofed (room too large to auto-roof)", "Unroofed". Walls get none. The room line adds ", roofed N%".
- **Spec:** `MAIN_UI_SCREENS_SPECIFICATION.md` (Roof overlay paragraph under overlays) and ART §16.1.

## Tests
- `roof_ui_test.go` (8): tools and their exact strings, the emitted commands, the ghost decision, forcing, the HUD toggle, the R toggle, the legend.
- `roof_draw_test.go`: labels, the room percentage, overlay colours, overlay on only when toggled or forced.

## Known limitations
- The R key toggles inline in `renderer.go` rather than through `toggleRoofOverlay` (same effect).
- No other overlays (beauty, temperature) share the button strip yet.
