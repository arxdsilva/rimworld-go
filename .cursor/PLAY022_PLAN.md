# PLAY-022: bigger animals, natural animal motion, puddles that blend

## Context
Three requests from play (2026-10-27):
1. "XS animals should take 1 tile, up to 6 tiles for XL; all animals feel too small", done the way RimWorld does it.
2. "Make the animation better; it does not seem natural."
3. "Fix puddles: they look bad and don't blend with the environment."

**What exists:**

*Sizes:*
- Body length in tiles comes from two mirrored tables: Go `sizeLength` (`internal/domain/animal/animal.go:825`) and Python `SIZE_LENGTH` (`assets/source/blender/artlib/families/animal_roster.py:14`): XS 0.44 … XL 2.0, species trims ±15%.
- Sprites are baked at 128 px per tile (`contracts.py`; the renderer's `artTilePx`, `art.go:33`), with no runtime scale.
- Drawn boxes today: a rat 0.57×0.17 tiles, a deer 0.89×1.02, an elephant 2.12×1.43; a colonist is about 0.5×1.0.
- The sim never uses the length (classes drive the numbers), so goldens don't move.
- **RimWorld:** body size runs from about 0.2 (rodents) to 4 (elephant-class) times a human. Every creature stands on ONE tile in the simulation and is only drawn bigger.

*Animation* (`anim.go`, `animal_figure.go`, `draw_combat.go`, `motion.go`, rig `animals/rig.py`):
- Frames: standing, `walk01..08`, `rest`, `graze1..2`, `flinch1..2`, a natural weapon. Facings S, E and N (W mirrored).
- **Foot skating.** The walk frame advances with distance at one `strideLen = 1.6` tiles per cycle for EVERY species (`anim.go:140-146`). The baked planted foot travels only about 0.76·stride·length per cycle (`rig.py:848-858`). So bodies move 4-40× further than the feet: a deer about 5×, a rat about 35×.
- **Pops.** At a stop (`Blend < 0.05`) the frame cuts from any walk phase to standing. Starts begin mid-cycle (the stride keeps accumulating, `motion.go:353`). Standing ↔ graze ↔ rest are hard cuts.
- **Graze** flips every 0.75 s (`draw_combat.go:498`), even on bare ground.
- **Bobbing:** the renderer's bob peaks at walk01/05 while the baked one peaks at 03/07, so they cancel. Breathing is under 1 px.
- **Speeds:** wandering animals move at running speed (an M animal ≈ 3.75 tiles/s, faster than a colonist's 4 tiles/s jog), with no walk/run difference; a tortoise isn't slow (`animal.go:432-467`).
- **Fast game speed:** at 6× the cycle reaches about 112 frames/s against a 60 Hz draw (aliasing).
- The path smoothing (Bézier corners, eased starts and stops) is already good; keep it.
- **RimWorld:** animals have no leg animation at all; they glide with a bob. Ours has walk cycles, so to read as natural the feet must stay planted and transitions must be smooth.

*Puddles* (`environment.go:896-984`, art `assets/generated/world/weather/puddle_00..05`, `weather.py:194-221`):
- They are per-tile stickers chosen by white noise (`h < 0.35·wet`), so they sit on a grid and pop in at full opacity.
- **The art:** about 0.9 alpha, a fixed pale grey-teal ((147,174,182), unlike the game's water `#6cb1bd`), a dark rim that reads as an outline, and straight cuts where 4 of 6 variants hit the tile edge.
- **The normals:** dome normals (normalgen doesn't skip them; `isGroundKey` excludes them), so they light like raised gel blobs.
- **The placement:** drawn over grass tufts, and on water, sand, mud and farm furrows. They vanish below 32 px per tile and hard-cut at snow ≥ 0.5.
- The flat wet-ground darkening (`snow_cache.go:238-268`) isn't tied to them.
- **What works and can be reused:** the snow veil (a chunk-baked mask with per-corner depth, smooth across tiles: `snow_veil.go`, `snow_cache.go`).

**User decisions:**
- Size means the AREA a body covers: XS ≈ 1 tile, S ≈ 1.5, M ≈ 2, L ≈ 4, XL ≈ 6 (an elephant about 3 long and 2 tall). The sim footprint stays 1 tile.
- Re-render every animal, with large animals at a capped resolution the renderer scales up, so animal memory stays about where it is.

## Part A: sizes
1. **Calibrated table (ART).**
   - For each class, measure today's median drawn area (the east standing frame's alpha box, in tiles) and scale its length by √(target / median). Rough result: XS ≈ 1.2, S ≈ 1.4, M ≈ 1.6, L ≈ 2.1, XL ≈ 2.8.
   - Class bases strictly increase; species trims stay.
   - Python `SIZE_LENGTH` (ART) and Go `sizeLength` (SIM) are changed together, with a comment naming the area targets.
2. **Capped density.**
   - Every animal sprite carries a new metadata key `ppt` (px per tile; absent = 128, so other families are unchanged).
   - Small classes keep 128. Larger ones render at a density that caps the canvas's long side, calibrated so that TOTAL animal pixels (colour + normals) ≤ 1.2× today's (measured, reported, checked).
   - Outline widths scale with the density.
3. **Renderer:** every art-pixel-to-tile step uses the sprite's own `ppt`:
   - `parseArtMeta` and `artOrigin` (`art.go:668`), `drawSpritePose`, `drawAnimalFigure`, `youngPose`, the figure pivot (`anim.go:61`);
   - shadow casters (`shadow_cast.go` `creatureCasters`, `castCreature`, `spriteKappa`), corpses, alpha boxes.
   - **Culling sized from the largest animal canvas:** `anim.go:75`, `draw_art.go:403`, `drawnReach` (`motion.go:40`), `visPad` (`lod.go`), `shadowMaxLen` (`shadow.go:49`).
   - **Picking:** a click or box select on any tile the animal's drawn body covers selects it (from the alpha box); a pawn or item on the exact tile still wins (`selection.go`, `multiselect.go:207`).
   - **Old-length thresholds** move to the class or the new table: `largeSize` (`animal_sounds.go:67`), `animalHeightK`/`animalRefSize` (`draw_combat.go:535-541`), the snore offset.

## Part B: natural animal motion
1. **Planted feet (ART + RENDER).**
   - The rig writes each set's ground travel per walk cycle into its metadata (`gait: <tiles>`), computed from the stride sweep, duty, length and `FIGURE_SCALE`. If needed, the sweep is widened to a natural range: the hind foot reaches forward under the body.
   - `trotSuffix` advances frames by distance ÷ that species' `gait`, not the global 1.6. Juveniles scale it with `youngPose`.
   - A test checks that a planted foot's screen drift per cycle stays under a small epsilon for a rat, a deer and an elephant.
2. **Smooth transitions (RENDER):**
   - a start resets the cycle to the "contact" frame and eases the stride in;
   - a stop finishes to the nearest neutral frame over the remaining ease-out (no cut from mid-stride);
   - standing ↔ graze ↔ rest pass through a short neutral hold, using the frames we have.
3. **One bob:** the renderer's `humps` bob is dropped for animals (the baked body rise carries the gait). The baked pitch and head nod are raised so they read at game scale (rig).
4. **Grazing and idling:**
   - graze only on cells with something to eat, in long bouts with per-animal timing (seconds, not 0.75 s): head down, then an occasional head-up pause;
   - breathing scaled to be visible on every size;
   - predators idle with the standing frame plus breathing.
5. **Walk vs run (SIM, our numbers):**
   - Wandering, grazing and penned milling "amble" at about half today's speed.
   - Errands walk at today's speed; fleeing, hunting and charging run (today's flee speed).
   - Speed-class differences widen so slow species (tortoise) are clearly slow.
   - The step-tick table and the mode → urgency mapping are in `animal.go` / `animals.go`. Goldens WILL move (animals take longer to wander): re-pin with the reason.
6. **Fast-speed aliasing:** the gait frame rate is capped at the display rate (a cycle never advances more than about half a cycle per frame; frames are picked from the eased position).

## Part C: puddles that blend (PUDDLE agent, opus: art rule)
- **Replace the per-tile stickers** with a smooth, chunk-baked puddle layer like the snow veil:
  - a low-frequency noise field (coherent patches across tiles, seeded per map) thresholded by `Wetness`, so puddles GROW and SHRINK smoothly as it rains and dries;
  - soft edges with no rim;
  - masked to open, flat soil-type ground: soil, gravel, dirt, grass ground. Never on water, ice, sand (drains), mud or marsh (they have painted puddles), floors, roads or farm furrows, nor under roofs.
- **Colour:** the terrain's own colour darkened (wet) with a faint sky-reflection tint that follows daylight (lit FLAT through the light grid: no dome normals). Low alpha with a slightly stronger centre.
- **Layering:** drawn under grass tufts (tufts poke through), above the terrain, tied to the existing wet-ground darkening (one veil, shared mask).
- **Other behaviour:**
  - fades out smoothly under snow;
  - kept at far zoom (baked into the chunk and map LOD);
  - rain ripples only inside puddle pixels.
- **Clean-up (remove-unused-assets rule):** retire the puddle sprites, normals, metadata and their `weather.py` builder, plus `puddleShape`. Normalgen and `isGroundKey` treat any remaining ground decal as flat.

## Execution (`.cursor/PARALLEL_WORK.md`)
**Lead contract first:**
- the metadata keys `ppt` (int) and `gait` (float tiles per cycle), parsed in `art.go`'s meta with defaults (128; the old 1.6);
- the size area targets and the memory cap rule;
- the motion urgency names (amble, walk, run) in the animal read model (`AnimalView.Gait string`) so the renderer can pick the cycle rate;
- the zoo spacing helper.

**RAM:** run `make agent-budget` before each wave.
- ART's Blender run is the art slot (long); prototype first (`ANIMAL_FRAMES=static`, `RWG_OUT_ROOT=<scratch>`), then the single full render that carries BOTH the size and gait changes.
- RENDER and PUDDLE are light, with about 2 GiB real-art tests. SIM is light.

| Agent | Model | Owns |
|---|---|---|
| ART | opus (art rule, `rimworld-go-art-assets` + `rimworld-go-creatures`) | `artlib/families/animal_roster.py`, `animals/*.py` (size, gait metadata, stride, bob, nod), the density pass-through in `stage.py`/`contracts.py` (additive, animals only), the regenerated animal and corpse PNGs, normals and metadata, a before/after review sheet |
| SIM | sonnet | `internal/domain/animal/animal.go` (`sizeLength` from ART's calibrated values; step ticks and urgency), the roster tests, `application/game/animals.go` (mode → urgency, `AnimalView.Gait`), goldens (re-pinned with reasons) |
| RENDER | opus (density math, gait playback, transitions) | ebiten `art.go` (meta keys), `anim.go`, `animal_figure.go`, `draw_combat.go`, `motion.go`, `shadow_cast.go`, `shadow.go`, `lod.go`, `draw_art.go` (the animal pass only), `animal_sounds.go`, `selection.go`, `multiselect.go`, `held.go`; tests |
| PUDDLE | opus (art rule: ground look) | ebiten `environment.go` (puddle parts), `snow_cache.go`/`snow_veil.go` (the wet and puddle mask), a new `puddle_veil.go`, `water_draw.go` ripple masking, `cmd/normalgen` (the flat-decal rule), `light.go` `isGroundKey` (one line), the removal of the puddle assets and `weather.py` `_puddle`; tests |
| Lead | session model | the contract, the zoo spacing and captures (`cmd/game/main.go`, `capture.go`), docs, verification, commits |

**Waves:**
1. ART (prototype) + SIM + PUDDLE.
2. RENDER, against the contract and fake metadata, early.
3. ART's full render once the RAM budget allows.

## Tests (TDD; never weaken: re-express pinned values with reasons)
- **Sizes:**
  - the class bases increase; the Go and Python tables match;
  - re-express `roster_test.go:182`, `TestXSBetweenOldAndS` and `TestOverlayAboveTallAnimals`;
  - real art: each class's median drawn area is within ±15% of its target, and total animal pixels ≤ 1.2× the baseline.
- **Renderer:**
  - `artOrigin` with `ppt` 64 vs 128 gives the same tile size;
  - the cull window keeps big animals drawn;
  - a body-tile click selects them;
  - markers sit above the new heights;
  - `TestCreaturesKeepMinimumSize` still holds.
- **Motion:**
  - planted-foot drift per cycle under epsilon (rat, deer, elephant);
  - a start begins at the contact frame;
  - a stop never cuts from mid-stride;
  - graze bouts last seconds, with no flip faster than the minimum hold (a no-flap style test);
  - the cycle rate is capped at 6×;
  - amble < walk < run speeds; a tortoise is slower than a deer;
  - the sim's urgency per mode; goldens re-pinned with reasons.
- **Puddles:**
  - coverage grows monotonically with wetness;
  - patches are spatially coherent (neighbour correlation);
  - never on water, sand, floors, furrows or roofed cells;
  - soft edges (alpha gradient);
  - flat lighting (no normal map);
  - present at far zoom;
  - fades under snow;
  - ripples only on puddles;
  - the orphan art check passes after the removal.
- **Always green:** `TestGeneratedArtCoversRenderer`, `TestUIDrawsThroughCanvas`, `make art-check`.

## Verification (lead)
1. Build, vet, gofmt, `make lint`, the Windows cross-build.
2. `make art-check`.
3. `make agent-budget`, then the full `GOFLAGS=-p=2 go test -race ./...` run alone (the known local `TestAtlasCoversKeys` failure is expected).
4. Memory and fps before/after with the fair probe: grown 500 and the zoo. Report max RSS, footprint, the art stats line, update and draw p99.
5. Captures and short clips, looked at:
   - the zoo by class;
   - colonists beside a rat, a deer, a horse and an elephant at zoom 1 and 2;
   - a deer walking, stopping and grazing (frame strips across 2 s to check planted feet and transitions);
   - 6× speed;
   - a storm then drying (`-scenario rain`, several tick counts: puddles grow and fade, on grass, gravel and next to sand or floors);
   - far zoom.
6. Docs:
   - ART spec: §11.1 sizes as areas, `ppt`, §27.1 gait per species, the puddle section `:620-625`;
   - the creatures skill (sizes, gait);
   - assets/AGENTS.md;
   - handoff `.cursor/handoffs/ART-0xx-play022-animals-puddles.md`;
   - checklists.
7. Commits by area on `main` (contract, sim, renderer, art, puddles, dev, docs); no push.

## Critical files
- **Tables and rig:** `internal/domain/animal/animal.go` (`:432-467`, `:825`), `animal_roster.py:14`, `animals/rig.py` (`:841-860`, `:986-1002`, `:1011`, `:1037-1064`, `:1129`), `stage.py`, `contracts.py`
- **Renderer:** `art.go`, `anim.go` (`:61`, `:75`, `:140-183`), `animal_figure.go`, `draw_combat.go` (`:494-572`), `motion.go` (`:323-353`), `shadow_cast.go`, `selection.go`
- **Puddles:** `environment.go:896-1000`, `snow_cache.go:238-268`, `snow_veil.go`, `cmd/normalgen/main.go:53-62`, `light.go:920`, `weather.py:194-221`
- **Reuse:**
  - the motion system's Bézier and eased path (`motion.go:581-620`);
  - `youngPose`;
  - the art alpha boxes (`art_memory.go`);
  - the snow veil chunk-bake pattern;
  - `TestPuddlesFollowWetness` (re-express);
  - the `ANIMAL_FRAMES=static` / `RWG_OUT_ROOT` prototype path.

## Appendix: lead contract (2026-10-27)

**C1. Sizes.** Measured today (the east standing frame's alpha box, class medians): XS 0.11, S 0.24, M 0.63, L 1.59, XL 2.72 tiles². Lengths that reach the area targets (×√(target/median)):

| Class | Area target | Old length | New length |
|---|---|---|---|
| XS | 1 | 0.44 | 1.3 |
| S | 1.5 | 0.6 | 1.5 |
| M | 2 | 0.95 | 1.7 |
| L | 4 | 1.4 | 2.2 |
| XL | 6 | 2.0 | 3.0 |

- The species trims (±15%) stay. An XS+ (1.50) may now be longer than an S- (1.28): the AREA classes still order, and a rat is long and thin while a hare is tall.
- ART edits Python `SIZE_LENGTH`; SIM edits Go `sizeLength`. A Go test (SIM) reads `animal_roster.py` and checks the two tables match.
- **Revised 2026-10-27 after ART's full static render:** XS 1.45, S 1.65, M 1.8, L 2.2, XL 3.0. The outline doesn't grow with the body, and bird, reptile and insect sprites had inflated old boxes (their thin-outline wrapper never applied), so XS and S landed 17–20% under target at 1.3 and 1.5. M moves to 1.8 to keep a gap above S.

**C2. Canvas and density (ART).**
- **One canvas per species, sex and FACING:** the union of that facing's poses plus PAD. Today it is the union over every facing plus the whole tile, mostly empty. Measured: tight per-facing canvases at 128 px per tile everywhere would be 1.20× today's pixels.
- **Shared within a facing:** all frames of one species, sex and facing keep one canvas, `origin_px` and `ppt`.
- **Mirroring:** east margins stay symmetric about the footprint centre, so west is east mirrored.
- `origin_px` may fall outside the canvas (the renderer must accept any value).
- **`ppt`** (integer, written in EVERY animal and corpse metadata file):
  - 128, unless a facing canvas's long side at 128 would exceed 384 px; then ppt = the even integer ≤ 128 × 384 / long_px.
  - Modelled: 1.05× today's pixels, with only the longest L and XL facings below 128 (down to about 63).
  - Baked outline widths scale by ppt/128, so their on-screen width is unchanged.
- **Budget:** animal + corpse PNG pixels (colour + normals) ≤ 1.2 × today's 291.3 M (animal 281.2 M + corpse 10.1 M). ART measures and reports it. If the total is over, the 384 px cap comes down, but never below 64 ppt.

**C3. `gait` (ART writes, RENDER reads).**
- `gait: <float>` is an adult's ground travel, in tiles, per 8-frame walk cycle: the distance a planted foot moves backward under the body in one cycle, the same in every facing.
- Written on every file of the set.
- The renderer advances walk frames by distance ÷ (gait × the view's `Scale`). Absent → 1.6 (legacy).

**C4. `AnimalView.Gait` (SIM writes, RENDER reads).**
- `game.GaitAmble` / `GaitWalk` / `GaitRun`, or "" when not stepping (read as walk).
- **amble:** wandering, grazing, penned or herd milling.
- **walk:** errands (going to food, a bed or a pen, being led, leaving).
- **run:** fleeing, hunting or chasing, attacking or charging, manhunter.
- **Step ticks:**
  - walk = today's step;
  - run = today's flee step (60%);
  - amble ≈ 2× walk.
- Speed classes widen (our numbers): a tortoise is clearly the slowest, and every gait is slower than a deer's same gait.
- No-flap: a gait holds for at least its current step (the step's ticks are fixed when it starts).

**C5. Graze.**
- SIM: `Action == "graze"` only when an idle grazer stands on a cell with something it eats (grass or a plant); otherwise "".
- RENDER: graze bouts last seconds with per-animal timing (head down, occasional head-up), with a minimum hold (no-flap test).

**C6. Puddles.**
- `drawPuddleVeil` (`puddle_veil.go`, PUDDLE) is called from `draw_art.go`:
  - at the live tier, after the furrows and before the tufts;
  - at the chunk tier, after the ground bake and before the furrows and tufts;
  - at the map tier, after the map bake.
- PUDDLE never edits `draw_art.go`. The per-tile puddle decals in `drawGroundCover` go; the snow, drift and frost decals stay.

**C7. Picking (RENDER).** A click or box select on a tile covered by an animal's drawn body (its standing frame's alpha box at its drawn position) selects it. A pawn, item or building on the exact tile still wins.

**C8. Thresholds.** Renderer thresholds that read `AnimalView.Size` (marker lift, bar lift, the snore offset, `largeSize`) are recalibrated by RENDER to the new table (`largeSize` was the old L length 1.4; now the L class is 2.2 × 0.85).

### Ownership

| Agent | Model | Owns | Forbidden |
|---|---|---|---|
| ART | opus | `assets/source/blender/artlib/families/animal_roster.py`, `artlib/families/animals/*.py`, animal-only additive changes in `artlib/stage.py`, `artlib/contracts.py` and `artlib/meta.py`, `validate_assets.py` animal checks; the regenerated `assets/generated/world/animal/*`, `assets/generated/world/item/corpse_*`, `assets/metadata/animal/*`, `assets/metadata/item/corpse_*`, the `ui/icons` animal icons only if their builder reads the length; a review sheet `assets/generated/animal_sizes.png` | Go code, `weather.py` |
| SIM | sonnet | `internal/domain/animal/*` (sizeLength, step ticks, gait speeds, tests), `internal/application/game/animals*.go`, `action_view.go` (graze rule), `animal_view_species.go`, the golden tests' pins (with reasons) | `internal/adapters/**`, assets |
| RENDER | opus | ebiten: `art.go` (beyond the contract), `anim.go`, `animal_figure.go`, `draw_combat.go`, `motion.go`, `shadow_cast.go`, `shadow.go`, `lod.go`, `draw_art.go` (the animal pass), `animal_sounds.go`, `selection.go`, `multiselect.go`, `held.go`, `art_memory.go` (stats only), their tests, `animal_species_test.go`, `lod_test.go` | `environment.go`, `snow_*.go`, `puddle_veil.go`, `water_draw.go`, `light.go` |
| PUDDLE | opus | ebiten `puddle_veil.go`, `environment.go` (the puddle parts: `puddle`, `puddleShape`, `puddleKeys`, the ripples' mask), `snow_cache.go`/`snow_veil.go` (the wet mask), `water_draw.go` (ripple masking), `light.go` (`isGroundKey`, one line), `cmd/normalgen`, `weather.py` (`_puddle` removal), the `puddle_*` assets and metadata, the `orphan_allow.txt` entries; their tests | animal files, `draw_art.go` |
| Lead | session | the contract, `cmd/game/main.go`, `capture.go`, docs, checklists, handoff, commits | |
