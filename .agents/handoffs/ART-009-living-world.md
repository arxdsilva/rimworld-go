# ART-009 Living world: wind, weather, cozy vegetation, burial visuals

## What changed
- **Spec:** ART §9 (vegetation set and cozy rule), §16 (weather, corpses, graves and pyre), §27.3 (environment motion, normative), §28 (variant keys).
- **Vegetation art** (`plant.py`, about 22 s to build; 34 → 217 sprites):
  - **Trees:** oak, birch and pine × 3 variants. Oak and birch also have `_autumn` and `_bare` (winter, snow-capped). Metadata carries `sway`.
  - **Bushes:** `bush_00..03`, `berry_bush_00..02` (+ `_harvestable`).
  - **Grass:** `grass_00..07` (some flowering) and `grass_tall_00..03`.
  - **Crops:** 8 crops × 6 stages (sown, sprout, young, mature, harvestable, wilted) × 3 variants.
  - The old ids are kept as aliases.
  - New palette materials: `leaf_pine`, `leaf_pine_hi`, `leaf_autumn_red`, `leaf_autumn_amber`, `bark_birch`, `snow`.
- **Corpse, burial and weather art:**
  - Animal corpses in 3 lying poses each, and `bones_00..01`.
  - Graves (`grave_dug` / `grave_filled` × south/east), headstones 00..03, the pyre and its burning state, and `tool/shovel`.
  - A new `weather` family: snow cover (edge and full), drifts, puddles, frost, and raindrop/splash/snowflake textures.
- **Environment renderer** (`environment.go`):
  - **Wind field:** the world wind plus two gust fronts moving downwind at 1.2× the wind.
  - **Lean:** the exact steady-state response of a damped spring (k=38, ζ=0.35), so there's no per-plant state. Per-kind limits through tanh; a 0.3 s canopy lag on trees.
  - **Contact:** a per-frame index of moving creatures. Plants within 0.6 tile get a spring push away from the creature, kept in a per-tile map and pruned once settled. They settle in 1.08 s.
  - **Trees:** shake when brushed, and drop leaves in fall.
  - **Rain:** wind-slanted, motion-blurred streaks drawn in one batch, with splashes and puddle ripples.
  - **Snow:** flakes in four sizes with flutter and drift.
  - **Sky:** storms darken the scene, with deterministic lightning; blizzards add fog.
  - **Ground:** snow cover blended across tiles and banked against walls, puddles by wetness, frost on freezing mornings, and snow burying grass.
  - **Keys:** species, variant, stage and seasonal keys come from prebuilt tables, with no per-frame strings.
  - **HUD:** weather and a wind arrow.
  - **Particles:** a separate weather pool, capped at 1400 for rain and 1100 for snow.
- **Grass:** denser (1 in 5 soil tiles), 8 tuft variants, and tall meadow grass on half the rich-soil tufts.
- **Burial renderer** (`burial_anim.go`):
  - Human corpses are drawn from the deceased's own layers, lying (`fall4`) with the dead tint, plus the rot tint and flies when rotten.
  - Bones variants; dug and filled graves with headstones; the pyre with a large flame, glow, and embers and smoke (emitted per advance, never in Draw).
  - Work rows: dig (`mine` frames with the shovel), bury (`pick` frames, soil pats), cremate (`bench` frames, embers). A carried body is a wrapped bundle.

## Verification
- Tests:
  - `environment_test.go`: 22 tests, plus a smoke test
  - `environment_bench_test.go`: `BenchmarkDrawPlants` about 0.54 ms for 1000 plants and 6 movers (budget 1.2 ms)
  - `burial_anim_test.go`
- ebiten coverage 52.3% → 57.9%.
- `make art-check`: 3159 sprites, 0 errors.
- Reviewed by eye on frame quads:
  - rain storm: slanted streaks, puddles and ripples, autumn trees, HUD "Storm 4.0"
  - snow: flakes, snow-capped bare trees
  - late game in rain: three crop stages, the compound, the raid
  - burial: bodies, then filled graves with a cross and a wooden marker, with "laid to rest" messages
- README media regenerated: every gallery item is now an animated WebP, plus new lategame, rain, snow, grass and burial clips.

## Known limitations / follow-ups
- Snow cover needs about 3 in-game hours of snowfall to show strongly. The snow clip starts at tick 12000 for that reason.
- No Architect icons for the grave and pyre yet; they borrow the stone and campfire icons.
- Concurrent `make art` runs share Blender's temp render file. Run families one at a time, or with a private `TMPDIR`.
- No biome plants, and no far-zoom LOD for vegetation.
