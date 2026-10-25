# Task Handoff

Task ID: UI-024 (PLAY-012)
Status: complete

## Summary
The UI scales crisply with the screen. It lays out in **logical** pixels (all old constants unchanged) and draws through a scaling canvas: rects, lines, images and triangles are multiplied by the scale and text is shaped at the real pixel size, so nothing is stretched. `LayoutF` renders at the screen's true resolution on high-density screens (HiDPI); the map keeps its apparent size there (tile = TileSize × zoom × device scale) and the UI scale never zooms the map.

- Scale s = device scale × UI scale. Auto (default): `min(window w/1280, h/720)`, clamped 1–2, rounded down to 5 %; presets 75–200 %. Options › Accessibility "UI scale" cycles Auto (shows its value) and the presets; Graphics "Sharp on high-density screens" toggles HiDPI; both apply live and are saved (settings version 2; older files load with Auto and HiDPI on). Captures force scale 1 and stay identical.
- Input: `currentCursor()` is logical (UI hit tests), `mapCursor()` is physical (map picking, camera, selection, box select, zones, minimap/globe picking).
- `TestUIDrawsThroughCanvas` fails if a file draws raw without being on the reasoned exemption list (world renderer, backgrounds, globe, world-space overlays).

## Numbers
500×500 grown colony, z0.25, Retina: HiDPI on (2560×1440) 120 fps avg, 1 % low ~111; off 120 / ~110 (vsync-capped).

## Files
`ui_scale.go`, `ui_canvas.go`, `font.go`, `app.go` (`LayoutF`, `mapViewport`), `input_frame.go`, `camera.go`, `renderer.go`, `minimap.go`, `globe.go`, the widget kit and every UI screen; `config.Settings` (`UIScaleAuto`, `Graphics.HiDPI`, presets); `cmd/game -hidpi` dev flag.

## Notes
- Spacing and row heights are still per-file constants (logical); a central token set is a follow-up.
- `TestAtlasCoversKeys` fails only on machines with stale local pawn sprite PNGs (git-ignored); a clean checkout passes.
