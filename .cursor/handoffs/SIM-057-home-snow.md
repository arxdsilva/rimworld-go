# Task Handoff

Task ID: SIM-057 (PLAY-015 §3–4)
Status: complete

## Summary
**Home:**
- an auto-expansion toggle (`SetHomeAuto`, saved as `NoHomeAuto`, default on), shown as the "Auto" button in the top strip;
- it grows around finished buildings (incl. pen markers) and newly painted stockpile, grow and pen zones (2-cell margin);
- repairs are limited to Home;
- the Zone tools are "Expand/Clear home area";
- "Build/Remove roof area" moved to Zone. The persistent `roof.AreaBuild` keeps roofs re-planned after a collapse.

**Snow:**
- per-tile depth 0..255 on open-sky cells (not walls, water or rock; roofed cells 0);
- it builds by weather intensity, melts by temperature and halves in rain;
- each chunk updates once per climate interval at its own tick, worked out from weather and clock alone, so loads are exact;
- walking is slowed above 76, grass stops regrowing from 26;
- the renderer veil uses corner-averaged per-cell depth;
- `SnowDepth` is the average where snow lies;
- an old save's single depth fills every open cell.

**Snow clearing area:** painted in cyan; the `ClearSnow` job (Cleaning, after ordinary cleaning) takes cells ≥ 38, with 20–80 ticks of work, and respects allowed areas.

## Files
- `game/home.go`, `roof.go`, `snow.go`, `snow_clear.go`.
- Hooks in `weather.go`, `grass.go`, `tick.go`, `priorities.go`, `defense.go`, `rooms_index.go`, `save.go`, `game.go`.
- ebiten `home_auto_ui.go`, `snow_veil.go`, `snow_clear_draw.go`.

## Notes
- The Home toggle sits in the top strip (where the overlay toggles live), not the lower-right.
- Grass tufts still fade by the map-wide snow figure (per-tile would need a grass rebake).
- Benchmark: 0.75 ms per full climate step at 500².
