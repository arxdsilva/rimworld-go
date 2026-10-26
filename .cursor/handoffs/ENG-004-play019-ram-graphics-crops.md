# Task Handoff

Task ID: ENG-004 (PLAY-019; plan `.cursor/PLAY019_PLAN.md`)
Status: complete

## Summary
**User decisions (2026-10-25):**
- Parallel work checks RAM before spawning agents.
- The game checks the computer's RAM and picks smooth graphics defaults unless the player overrides them; it warns only and never blocks.
- Hardware is detected at the first launch only; there are no runtime quality changes.
- Crops stay on their own tile.

**Parallel work:**
- `cmd/rambudget` (`make agent-budget`) reads free RAM (`internal/platform/sysinfo`: macOS sysctl, Linux `/proc/meminfo`, Windows `GlobalMemoryStatusEx`) and prints how many heavy, art and light agents fit after a 4 GiB reserve.
- The rule is in `.cursor/PARALLEL_WORK.md` §1/§3/§5 and skill `parallel-work` step 0: run it before each launch and phase, and agents test with `GOFLAGS=-p=2` and targeted `-run` only.

**Memory diet** (`art_memory.go`, `tint.kage`):
- The game indexes the art and decodes on first use. A background prewarm keeps deflated copies (~46 MiB) of every file family except animals.
- CPU pixels are freed after the GPU upload. Crop units, tilled overlays and grass tufts keep theirs (`cpuReadKeys`).
- Tints bake on the GPU in a 128 MiB LRU.
- The fps probe reports the Go heap and the art memory.

**Graphics tiers** (`config/graphics.go`, ebiten `settings.go`, `screen_options.go`):
- Settings v5 adds Quality, Shadows, Lighting, Water, Grass, Detected and Chosen.
- The first start, or the first after an update, picks High (≥ 12 GB and 6 cores), Medium (≥ 8 GB) or Low, keeps rows the player changed and saves once.
- Options › Graphics adds Quality (Custom once a row is changed), Detect again, and the four detail rows.
- Renderer knobs are applied live:
  - shadows off skips the cast-shadow layers;
  - lighting off draws with a flat colour scale and makes no lit pairs;
  - simple water uses the baked frames;
  - grass keeps 60% or 30% of tufts.
- The lit-pair cache is an LRU.
- `-graphics low|medium|high` applies a tier for one run.

**RAM warnings** (`ram_estimate.go`, `screen_menu.go`, `screen_landing.go`):
- The estimate is 1850 MiB + 6600 B per tile, fitted to the measured peak footprint.
- The main menu shows a notice below 6 GB, remembered in `Gameplay.RAMNoticeSeen`.
- Sizes estimated over 70% of RAM warn in the picker and the site panel.
- `cmd/game` reads the hardware once; captures get none.

**Crops** (`crop_batch.go`): units root at the art's tile-centre point (`origin.y + 64`), not the footprint bottom. `cropFit` moves a unit south (root ≤ 0.95), then shrinks it to its tile. Sway pivots at the root, and the scale clamp is 0.35–2.0.

## Measurements
Apple-silicon Mac at 2560×1440, `/usr/bin/time -l` with `-fpsprobe 15`, window visible.

| Run | Max RSS before → after | Peak footprint after | fps avg / 1% low / p99 before → after |
|---|---|---|---|
| 48x40 | 2715 → 1046 MiB | 2242 MiB | 119.1 / 87.1 / 11.5 ms → 119.7 / 87.1 / 11.5 ms |
| 250 | 2453 → 1085 MiB | 2141 MiB | 84.2 / 9.0 / 111.5 ms → 119.9 / 86.6 / 11.5 ms |
| grown 500 | 2612 → 1054 MiB | 3422 MiB | 115.3 / 56.0 / 17.9 ms → 119.6 / 83.9 / 11.9 ms |
| menu (20 s) | 2478 → 1148 MiB | 1388 MiB | n/a |

Draw p99 stayed within 1 ms of the baseline. A probe with the window hidden or the screen locked is throttled by macOS (about 9 fps); update and draw times still compare.

## Tests
- New: `sysinfo_test.go`, `graphics_test.go`, `rambudget/main_test.go`, `ram_estimate_test.go`, `graphics_settings_test.go`, `graphics_knobs_test.go`, `art_memory_test.go`, `crop_place_test.go`.
- `TestCropUnitsStayOnTheirTile` covers every crop, stage, pattern and the LOD; 101,183 units failed it at the old anchor.
- Full `GOFLAGS=-p=2 go test -race ./...` is green except `TestAtlasCoversKeys`, a known local failure: the git-ignored pawn PNGs are older than the atlas.

## Left open
- With Lighting off, the LOD ground, blood, dirt and crop batches still go through `lit.kage` with flat light and no pairs. A plain draw would need their vertex colours relit.
- Animals are not prewarmed (4608 sprites, ~765 MiB decoded): they fill on first sight, which a heavy raid can show as cold fills. The pawn atlas still holds 204 MiB of deflated pages; smaller pages in `cmd/atlasgen` would cut that.
- Tall crops show fewer plants per tile (ripe corn one stalk, wheat two clumps), and the 4-unit pattern's west column overhangs about 0.08 tile.
- No low-RAM notice on the boot screen (no widgets there).
- The estimate constants are from one Mac; re-measure on Windows/Linux when possible.
