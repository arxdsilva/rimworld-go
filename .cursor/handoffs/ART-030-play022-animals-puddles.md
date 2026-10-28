# Task Handoff

Task ID: ART-030 (PLAY-022; plan and lead contract `.cursor/PLAY022_PLAN.md`)
Status: complete

## Summary
Three requests from play (2026-10-27):
- "XS animals should take 1 tile, up to 6 tiles for XL; all animals feel too small" (the way RimWorld does it);
- "the animation does not seem natural";
- "puddles look bad and don't blend with the environment".

User decisions:
- size is the AREA a body covers (XS ≈ 1 … XL ≈ 6 tiles²), and the sim footprint stays one tile;
- every animal is re-rendered, with the longest bodies at a capped density the renderer scales up, so memory stays about the same.

**Sizes (art + sim):**
- **Lengths:** XS 1.45, S 1.65, M 1.8, L 2.2, XL 3.0 tiles, measured on a render to reach the area targets. The class median drawn areas are XS 0.97, S 1.48, M 1.98, L 3.75, XL 5.90 tiles².
- **Tables:** Go `sizeLength` and Python `SIZE_LENGTH`/`SIZE_AREA` are mirrored by a test. `validate_assets.py` fails a class more than 15% off its area.
- **Canvases:** one per facing, the union of that facing's poses only, instead of every facing plus the whole tile.
- **Density:** `ppt` is 128, or lower where a facing would pass 384 px: L 80–128, XL 64–128, corpses 88–128.
- **Pixels:** animal + corpse pixels are 0.88× before (257.5 M of 291.3 M).

**Motion:**
- **Art:**
  - Every sprite carries `gait`, the ground one walk cycle covers: rat 0.23, deer 0.82, horse 0.98, elephant 1.14 tiles. Measured slip is ≤ 1.3 px.
  - The stride is 1.7× wider; the body pitches 0–4° twice a cycle; the head nods ±9° (was ±3.5°).
  - Bird, reptile and insect bodies get their thin outline.
- **Sim:**
  - A step fixes its gait. Wandering, grazing and milling amble at 2× the walking step; errands walk; fleeing runs.
  - Hunting and charging report run but keep the walking step, so no fight changes.
  - The slow class went +6 → +12 ticks: tortoise 26 against a deer's 16.
  - `AnimalView.Gait` reports the gait. Grazing shows only on grass or an edible plant.
- **Render:**
  - Walk frames advance by distance ÷ gait, so the feet stay planted.
  - Walks start and stop on the standing-height frames (`walk03`/`walk07`). Standing, grazing and resting hold before they change.
  - Graze bouts run 3–8 s head down, then 1–2 s looking up.
  - No renderer bob for animals; breathing scales with the body; animals face sideways on diagonals.
  - The cycle advances at most 0.3 per drawn frame (no aliasing at 3×).
- **Sprites at their own `ppt`:** placed, mirrored, culled (on the posed canvas; cell pads measured from the art) and cast at it.
- **Picking:** a click or box select on any tile of a body picks the animal, after whatever stands on that tile.
- **Sizes follow through:** markers, bars and snores follow the new heights; `largeSize` is 1.87.

**Puddles:**
- One chunk-baked veil with the wet-ground darkening, at every zoom tier (`puddle_veil.go`, `snow_cache.go`).
- A low-frequency relief per map holds water below a level that rises with `Wetness`: puddles grow and shrink in the same hollows, with soft shores and no rim.
- Only open soil, rich soil and gravel hold water; the veil sits under the grass tufts, lit flat with a sky tint that follows daylight.
- It fades under snow, and ripples only land on water.
- The puddle sprites, normals, metadata and builder are deleted. Snow cover and frost lost their dome normals: ground decals are lit flat.

**Dev:** the zoo spaces animals 5 tiles apart.

**HUD:** the representation is the map itself: body sizes, gaits and grazing are seen directly. No text line was added for the gait, because the motion shows it.

## Commits
- d75c262df contract
- f0d1bcf5f sim
- dd959b334 art
- 6c151cadf puddles
- 1ff86e0bb render
- this docs commit

## Verification
- **Tests:** the full `go test -race ./...` passes on a clean export of the tree (tracked files only). The art checks pass there too: `validate_assets` 0 errors over 5312 sprites, no orphans, pawn atlas OK.
- **Builds:** vet, gofmt and the Windows cross-build are clean. `make lint` has no new findings in PLAY-022 files.
- **Goldens:** scale golden (6 pins) and grown golden re-pinned, for the slower idle wandering and the two new saved fields (`Errand`, `StepGait`). The biome golden is unchanged.
- **Performance:** grown 500×500, 3 ticks/frame, 30 s, second (warm) runs, before (601d367eb) vs after:

  | Zoom | fps avg | 1% low | Update avg | Draw avg / p99 | Peak footprint |
  |---|---|---|---|---|---|
  | 0.8 | 116.7 → 117.1 | 48.4 → 54.6 | 2.03 → 2.00 ms | 2.81 / 3.48 → 2.93 / 3.58 ms | 1413 → 1476 MB |
  | 0.1 | 115.6 → 115.8 | 48.1 → 46.8 | 2.40 → 2.42 ms | 2.64 / 4.25 → 2.66 / 5.10 ms | 2165 → 2275 MB |

- **Captures looked at:** the zoo, the pen beside colonists, a ground sloth turning and walking (frame strip), and puddles soaked, drying and at far zoom.

## Left open
- **XS and S look big:** at 1 tile² an XS animal (a squirrel, a hen) covers about twice a colonist's drawn area, because colonists are drawn at about 0.5×1 tile. That is the requested target. If it reads wrong, lower `SIZE_AREA` for XS and S and re-render.
- **Disk:** animal PNGs went 79 → 143 MB and corpses 5 → 11 MB (bigger sprites compress worse). Blender writes colour PNGs at default compression; a lossless re-save shrinks them about 20% (pipeline-wide, not done).
- **Far-zoom draw p99:** +0.85 ms at zoom 0.1. The veil is about 0.3 ms of it (measured with the veil off); the rest is unexplained. Averages and 1% lows are unchanged.
- **Normals:** normalgen's fixed 8 px bevel is softer on 64-ppt sprites.
- **Slower chases:** cow, pig and ground sloth chase a little slower (the slow class).
- **Social scenario test:** it clears wildlife so a wandering wolf can't kill its cast.
- **Local disk:** stale git-ignored pawn PNGs (2026-10-22) make `make art-check`, `TestAtlasCoversKeys`, `TestNoOrphanSprites` and `TestIndexCatalogMatchesLoad` fail on this disk only.
- **Lint:** `make lint` carries 17 older findings outside PLAY-022.
