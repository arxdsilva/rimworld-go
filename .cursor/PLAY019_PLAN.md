# PLAY-019: RAM-aware parallel work, hardware-aware graphics defaults, crops on their own tile

## Context
Three requests:

1. **Before any parallel work, check that the machine has enough RAM for the agents.** The last batch ran 13 agents, each running `-race` test suites, on this 18 GB / 11-core Mac; the kernel shows about 2.3 million pages swapped out since boot.
2. **The game should check the player's machine has enough RAM, and pick graphics settings that run smoothly by default unless the player overrides them.** Today:
   - There is no hardware detection at all.
   - The Effects level only changes particle counts. Shadows, normal-map lighting, the water shader and grass detail have no setting.
   - Nothing records which settings the player chose.
   - Every sprite family except pawns is decoded at startup, about 1 GB of pixels kept in RAM after GPU upload (`art.go:342-367`, `sp.pix` never freed).
   - The tint-bake and lit-pair caches are unbounded.
   - Total RAM use for a 250 or 500 map has never been measured.
3. **Crops grow over the tile to their north.** `crop_batch.go:443` anchors each unit's foot at the bottom of the footprint (`origin.y + 128`), but the art puts the root at the tile centre (`origin.y + 64`; `plant_base.py:13`, `crops_common.py:9`). So:
   - every plant draws half a tile too far north: a ripe potato's back row sits entirely on the north tile, and ripe corn about 97% on it;
   - growth barely shows, because a +0.5 bias dominates the stage heights;
   - sway slides the bases sideways.

   The test fixture encodes the same wrong convention, and no test checks a unit against its tile.

**User decisions:**
- Low RAM: warn only. The game never blocks anything, and big map sizes show a warning.
- Hardware detection runs at first launch only; no runtime quality changes.
- Crops stay on their own tile.

**Found on the way:** `scripts/package_release.sh:22` doesn't ship `assets/generated/atlas`, so a release build can't load the pawn atlas and the art catalog fails. Fixed here.

## 1. Parallel work checks RAM first (rule + tool)
- **`internal/platform/sysinfo`** (new, platform layer, no cgo), returning `Total` and `Available` RAM and the core count:
  - macOS: `unix.SysctlUint64("hw.memsize")`; available = total × `kern.memorystatus_level`/100;
  - Linux: `unix.Sysinfo`, plus `MemAvailable` from `/proc/meminfo`;
  - Windows: `GlobalMemoryStatusEx` through `windows.NewLazySystemDLL("kernel32.dll")`.
  - `golang.org/x/sys` becomes a direct dependency (it is already v0.44.0, indirect).
  - Unit tests per OS through small fakes; a build check for the three OSes (`GOOS=windows CGO_ENABLED=0 go build ./...`).
- **`cmd/rambudget` + `make agent-budget`:** prints total and available RAM, the measured peak of each agent kind, and the most agents to run at once, both heavy (`go test -race` on a big package, a Blender run) and light. The peaks are measured once in this batch (`/usr/bin/time -l` max RSS of `go test -race` on `internal/application/game` and `internal/adapters/inbound/ebiten`, a Blender family run, a capture) and kept in the tool, so they can be re-measured later.
  - Budget = available − 4 GB reserve (OS, IDE, the lead).
  - Max heavy = budget / heavy peak; light agents count at their own peak.
- **Rule** (`.cursor/PARALLEL_WORK.md` §1 and §5, skill `parallel-work` step 0, plus a feedback memory):
  - Before launching any agent, and again before each later phase, the lead runs `make agent-budget` and never runs more agents than it allows.
  - Heavy agents are staggered, not all started at once.
  - Agents pass `GOFLAGS=-p=2` on their test runs, which caps parallel package builds.
  - The lead re-checks free memory if an agent reports swapping or slow tests.

## 2. Game: RAM check and hardware-aware graphics defaults
**Measure first (lead, baseline).** Record max RSS at the main menu, on the default 250 colony, and on a grown 500 colony (`/usr/bin/time -l` with `-fpsprobe`). Add a memory line to the fps probe (Go heap plus process RSS) so later runs report it.

**Memory diet (agent MEMORY):**
- **Lazy decoding.** The game switches to the lazy art index (`indexArtCatalog(root, true)`, which tests already use), decoding a sprite on first use. A background prewarm covers the families every colony shows (terrain, plants, buildings), like the pawn atlas's `prewarmAsync`.
- **Free CPU copies.** Release a sprite's decoded pixels once its GPU image is made, unless something still needs them on the CPU (alpha boxes are kept). Tint bakes move to GPU draws so they no longer need CPU pixels.
- **Bounded caches.** The tint and lit-pair caches get an LRU bound.
- **Checks:**
  - RSS before and after on the menu, the 250 colony and the 500 colony;
  - fps probe and p99 frame time (no first-draw hitches above today's p99);
  - `TestGeneratedArtCoversRenderer` and the art tests stay green.

**Hardware tiers** (`sysinfo` at startup; our own thresholds, set from the measurements):

| Tier | Rough rule | Settings |
|---|---|---|
| High | ≥ 12 GB and ≥ 6 cores | Today's look: full effects, shadows, lighting, animated water, full grass, HiDPI, vsync |
| Medium | ≥ 8 GB | Reduced effects, shadows on, lighting on, simple water, reduced grass, HiDPI only if the device scale ≤ 2 |
| Low | Below that | Minimal effects, no cast shadows, no normal lighting, simple water, sparse grass, HiDPI off |

**New graphics settings** (`config.Graphics`, settings v5): `Quality` ("high", "medium", "low", "custom"), `Shadows`, `Lighting`, `Water`, `GrassDetail`, plus `Detected` (the tier found) and `Chosen []string` (rows the player changed by hand).

**First launch:**
- There is no settings file yet. Detect the tier, set the graphics fields to it and save.
- **v4 → v5 migration:** run detection once, keeping any graphics value that differs from the old defaults (it was the player's choice).

**Options › Graphics:**
- a Quality row: High / Medium / Low, plus "Custom" once a single row is changed;
- rows for the new knobs, each with a tooltip saying what it costs;
- "Detect again", which re-applies the detected tier and clears Chosen;
- the existing Effects, VSync and HiDPI rows.
- The player's changes are always kept.

**Renderer knobs (agent GRAPHICS):**
- Shadows off: skip the cast-shadow layer (`shadow.go:598`, :696).
- Lighting off: draw sprites without the normal-map lit shader (daylight colour scale only, no pair images).
- Water "simple": the existing reduced-motion path (`water_draw.go:50`).
- Grass detail: tuft density in `grass_batch.go`.
- Effects and HiDPI as today.
- Each is read live from the settings (`syncRenderer`), with a test per knob.

**RAM warnings (agent WARNINGS; warn only, never block):**
- **Low-RAM notice:** if total RAM is below the measured minimum (game peak on the default colony × 1.5 + 2 GB, rounded), a notice on the main menu (and the boot screen) reads "This computer has N GB of memory; the game needs about M GB to run smoothly. It will use low graphics settings." It is dismissible and remembered for this amount of RAM.
- **Map sizes:** the estimated RAM per size comes from the measured 250/500 numbers. Sizes whose estimate exceeds 70% of total RAM show a warning in the size picker and the site panel: "Needs about N GB; this computer has M GB." They stay selectable. This replaces the fixed "needs a capable computer" label at 350+, and the 400 "Large map" site warning that disagrees with it.

## 3. Crops stay on their own tile (agent CROPS)
- **Anchor fix:** `crop_batch.go:443` becomes `fy := (Origin[1] + fh*artTilePx/2)/kpx - box.Min.Y`, the root at the tile centre per the shared plant convention. That fixes the position, the growth scaling and the sway pivot together. The art is unchanged: the main-menu scene and the placement rule rely on it.
- **Containment:** after placement, a unit whose top would cross the tile's north edge first moves south (its root no further than 0.95 of the tile), then scales down until it fits. So tall crops (corn, wheat) stay on their tile too. Recheck the growth-scale clamp (`cropScaleMin`/`cropScaleMax`), since the stage ratios now spread wider.
- **Tests:**
  - fix the fixture (block bottom at `origin.y + 64`);
  - a new test with real art (`testArt`): for every crop, every stage and every unit pattern, the drawn quad stays inside its tile (top ≥ tile top, bottom ≤ tile bottom, small epsilon), at zoom 1 and at the LOD path;
  - growth visibly rises stage to stage;
  - the sway pivot sits at the root.
- **Visual review** (art rule: opus): captures of the farm (`-capture-ui grow`) zoomed in and out, compared before and after.

## 4. Execution
**Lead contract first:**
- the `sysinfo` package;
- the new config fields and settings v5 migration skeleton;
- the `rambudget` tool and peak measurements;
- `make agent-budget`;
- the packaging fix;
- the baseline RSS measurements;
- the rule, skill and memory updates.

Then run `make agent-budget` and launch only what it allows (expected on this machine: about 11 GB available, so 3-4 heavy agents), staggering the heavy ones.

| Agent | Model | Owns |
|---|---|---|
| CROPS | opus (art rule: art placement and visual review) | `crop_batch.go`, `crop_batch_test.go` (+ a new real-art containment test) |
| MEMORY | opus (caches, background decode, concurrency) | `art.go`, `art_atlas.go` (prewarm), `light.go` (lit-pair cache bound), tint bake code; memory tests |
| GRAPHICS | sonnet | `shadow.go` / `water_draw.go` / `grass_batch.go` knobs, `settings.go` (ebiten) first-launch apply, `screen_options.go` Graphics rows; tests |
| WARNINGS | sonnet | main-menu/boot notice, `game.MapSizes()` estimates and `screen_landing.go` size labels and site warnings; tests |

**Lead verification:**
1. `go build`; `GOOS=windows CGO_ENABLED=0 go build ./...`; `GOOS=linux go build ./...`.
2. fmt, vet, lint.
3. `go test -race ./...` (with `GOFLAGS=-p=2`).
4. Determinism and goldens.
5. `make art-check`.
6. RSS and fps before/after: menu, 250 and 500 colonies.
7. Captures at each quality tier, plus the crop captures.
8. Docs: handoffs; the checklists (graphics options, performance, crop art); README (system requirements and the Options line); `.cursor/PARALLEL_WORK.md`; `assets/AGENTS.md` note on crop anchoring.
9. Commits by area on `main`; no push.

## Critical files
- `internal/platform/sysinfo/` (new), `internal/platform/config/settings.go`, `cmd/rambudget/` (new), `cmd/game/main.go`, `Makefile`, `scripts/package_release.sh`
- `internal/adapters/inbound/ebiten/{crop_batch.go, art.go, art_atlas.go, light.go, shadow.go, water_draw.go, grass_batch.go, settings.go, screen_options.go, screen_landing.go, fps_probe.go}`
- `internal/application/game/scenarios.go` (MapSizes)
- `.cursor/PARALLEL_WORK.md`, `.cursor/skills/parallel-work/SKILL.md`
