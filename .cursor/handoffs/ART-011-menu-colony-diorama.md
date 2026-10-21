# Task Handoff

Task ID: ART-011
Status: complete

## Summary

The main menu's background is now a dusk colony diorama instead of a code-drawn gradient. It is one scene built in Blender from the existing sprite builders: a lit wood room with beds, table and lamp, a campfire with four colonists, a workbench lamp, a stockpile, crops, autumn trees, a grazing dustbuck, and a rock ridge behind the buttons. It is rendered through the canonical stage. The game draws it with a slow pan/zoom, flickering fire glow, breathing lamp glow, the in-game campfire flame, embers, dust motes, and a scrim that keeps the title, buttons and bust readable. Sub-menus show the same scene dimmed.

## Files Changed

- `assets/source/blender/build.py`: path resolution for live MCP use (stale `/Users/asilva` fallback removed); `SCENES`
- `assets/source/blender/artlib/stage.py`: `isolate`, `no_render`, `place`, `mat_tinted`, `swap_materials`, `flat`, `render_scene`; `_rig` reuses an existing camera/sun
- `assets/source/blender/artlib/families/terrain.py`: `_edge_img` split out (output byte-identical)
- `assets/source/blender/artlib/scenes/menu.py` (new)
- `assets/generated/ui/menu/*.png`, `assets/metadata/ui_menu/menu_diorama.yaml` (generated)
- `internal/adapters/inbound/ebiten/menu_diorama.go` (new) and `menu_diorama_test.go`
- `internal/adapters/inbound/ebiten/{app,art,capture,fire,burial_anim,screen_menu}.go`, `app_test.go`, `cmd/game/main.go`
- `Makefile` (`art-menu`; `art-capture` adds the menu), docs

## Public Contracts Changed

`AppConfig.LoadMenuScene` is optional. When nil, `NewApp` locates `assets/metadata/ui_menu/menu_diorama.yaml`. If the load fails, the gradient backdrop is drawn. `-capture-ui menu` captures the main menu without starting a game; `-capture-ticks` picks the moment of the motion.

## Invariants / Decisions

- One canonical rig (ART §21): the scene uses `stage.render_scene`, with no scene camera or light.
- Figures (pawns, animal) cast no sun shadow; numpy blob shadows stand in, matching the game's metadata-driven shadows.
- The dusk grade is position-independent and baked in. All screen-relative darkening is done in Go, because cover-fit crops differently per aspect ratio.
- Motion runs on `menuTick` (Update ticks off the colony screen), never the simulation; it is deterministic.
- The `ui_menu` metadata family is skipped by the sprite catalog.

## Verification

- `go test ./...` passes; 9 new tests (cover-fit at five aspect ratios, drift speed under 0.5 px/tick, flicker/breath/mote ranges, scrim, fallback, asset decode, catalog skip, capture without sim).
- Blender 5.2.2: `build(["menu"])` takes 16 s for the full 3200x1800 render; the base PNG is 2.4 MB and each glow layer about 63 KB.
- Captured the menu at ticks 0 and 2700 (1280x720): bust, title and buttons are readable, and the room stays clear of the rail.
- Terrain rebuild after the refactor: tiles and edges are byte-identical (two rock chunks differed by ±3/255 GPU noise and were restored).
- `validate_assets.py` was not run: Pillow is not installed on this machine.

## Determinism Impact

None (presentation only).

## Save / Mod Impact

None.

## Art / Visual Impact

New generated UI art under `generated/ui/menu/`. No world sprite changed.

## Screen / UI Impact

Main Menu §2 Background: diorama behind the bust. Sub-menus (`fillMenu`) show it at 55% dim.

## Known Limitations

- The scene is one render, so objects that overlap don't keep their own outlines (an object-ID contour pass could add them).
- Captures were only checked at the default 1280x720 window; other sizes are covered by tests, not by eye.

## Follow-ups

- Optional: an object-ID contour pass; seasonal variants of the scene.
