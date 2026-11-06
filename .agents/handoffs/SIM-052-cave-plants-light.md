# Task Handoff

Task ID: SIM-052 (PLAY-014, MAPGEN-3 §1)
Status: complete

## Summary
Plants now see a simulation light level per cell:
- **Light sources:** daylight on unroofed cells; lamps (0.7, radius 6) and campfires (0.6, radius 5); plants with a glow (duskmoss 0.1, radius 1). Rock, walls and doors block light.
- **Caching:** the light grid is cached in `game/light.go` and rebuilt only on structure, glow-plant or load changes.
- **Moss** (`MinLight` 35) only grows on lit cells.
- **Duskmoss** (new original species):
  - grows in the dark on cave stone (`plant.CaveFertility` 40) and is very slow (about 40 days);
  - slows walkers (125%) and glows faintly;
  - withers in light: from 0.45, with harm stopping only below 0.35 (hysteresis), losing 8% of full growth per hour;
  - when withered away it is removed, with one notice per patch per day.

Every other species has no light rule, so its growth is unchanged.

**mapgen** (`steps/vegetation/cave.go`):
- dark cave floor gets duskmoss in separate clusters of 1–4 cells (about 15–25% coverage);
- lit cave mouths (within 2 of open sky) may get ordinary moss where the biome has it.

## Files
- `plant`: `Def.{MinLight, DiesInLight, Glow, Cave}`, `Duskmoss`, `light.go` (fertility for Cave species).
- `lighting.Cell`.
- `game/{light.go, plant_light.go}` and hooks in `plant_growth.go`, `world.go`, `plant_view.go`; `PlantView.{NeedsDark, NeedsLight, Withering}`.
- `mapgen/steps/vegetation/cave.go`.
- Renderer: `cave_plants_draw.go` (a faint cool glow, inspector light lines).

## Notes
- A campfire lights whether or not it is burning, which avoids rebuilding the grid on every fuel change.
- Crops and trees have no light needs yet, so the checklist item is `[-]`.
