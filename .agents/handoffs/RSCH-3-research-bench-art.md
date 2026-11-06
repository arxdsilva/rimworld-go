# RSCH-3: research bench and multi-analyzer art

Original clean-room art through the Blender pipeline (`assets/source/blender/artlib/families/furniture.py`), following skill `rimworld-go-art-assets` (ART §1, §2, §12-§14, §35).

## What changed
- `furniture/research_bench_{south,east,north,west}`: 3x2 (2x3 east/west). A broad plank desk on six legs with an open book rack along the back, an open journal with quill and ink, a book stack, a rolled chart, a potted seedling, a flask and a candle. Same key as before; the old 2x1 sprites are overwritten (no orphans).
- `furniture/electric_research_bench_{facing}` plus `_glow_{facing}`: 5x2 hi-tech bench (the kind and key stay). A steel desk on drawer pedestals with a server tower, a twin-screen terminal, an analyzer cabinet, a holo pad, a sample rack, a data slate, a centrifuge and a microscope. Screens, lamps and the holo ring glow while powered, and roof lamps keep a lit part in every facing.
- `furniture/multi_analyzer` plus `multi_analyzer_glow`: 2x2, one facing (construction has no Rotatable). An enamel base with an instrument column and read-out, a signal tower and an open sample carousel with a teal ring. Glow is the on state (`poweredLook`), so no duplicate sprite.
- Worker attachments: `[1.5, 2.15]` (simple) and `[2.5, 2.15]` (hi-tech), centred on the south edge.
- Icons `ui_icon/research_bench`, `electric_research_bench`, `multi_analyzer` (`overlay.py` ICONS).
- `render/world/build_keys.go`: `"multi-analyzer"` → `furniture/multi_analyzer` (lead-approved exact edit).

## Checks
`make art-check`: 6216 sprites, 0 errors, no orphans. `TestGeneratedArtCoversRenderer`, `TestNoOrphanSprites` and `TestPowerArtCoversRenderer` pass with `-race`.

## Open
- The in-game look waits for the RSCH-1 footprints. Until then the renderer would draw the new canvases on the old 2x1 footprint.
- Research tree chrome belongs to the RSCH-2 tab (drawn UI, no sprites).
