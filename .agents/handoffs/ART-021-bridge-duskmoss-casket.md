# Task Handoff

Task ID: ART-021 (PLAY-014, MAPGEN-3)
Status: complete

## Summary
New original art:
- **Bridge:** `structure/bridge_00..15`. A flat 128×128 timber deck tile: five boards in three tones with dark seams, and a dark edge beam with nail heads on each side open to water. Boards run perpendicular to the walking direction. There is no baked shadow; the renderer adds one on the water.
- **Casket:** `structure/sleeper_casket_{closed,open}`. A 1×2 heavy metal capsule with a panel and a teal light at the head and a frosted lid; when open, the lid is slid askew over an empty cradle.
- **Duskmoss:** `plant/duskmoss_{young,mature,dormant}_v0..3`. Slate and grey-blue fibres on a charcoal root mass, with no outline.
- **Icons:** `ui_icon/bridge` and `ui_icon/open_casket`.
- **Palette:** `assets/palettes/materials_mapgen3.yaml`, loaded through the `materials_*.yaml` glob.
- **Sound:** `sfx.casket_open` (procedural).

## Notes
- `make art` re-renders whole families with GPU noise. The 807 untouched files it changed were restored.
- Duskmoss reads a little spidery at full zoom.
