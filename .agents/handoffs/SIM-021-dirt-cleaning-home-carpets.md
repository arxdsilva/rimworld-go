# SIM-021 Tracked dirt, cleaning, the Home area, cotton and carpets

## What changed

### Dirt
Domain package `internal/domain/world/filth`; game-side code in `game/filth.go`.
- **Pickup:** boots pick up 6 dirt per step on outdoor soil or gravel. Grass halves it, and rain triples it at full wetness. Field work (sow, harvest, cut, mine, dig a grave) adds 12 every 30 ticks.
- **Shedding:** stone and built floors outdoors shed 3% per step.
- **Indoor trail:** each indoor step (not under sky) drops 8% of the carried dirt, at least 1, and only while the boots carry 10 or more.
- **Carpets:** stepping on one wipes off cloth 70%, leather 55%, straw 45% or wood 30% of the carried dirt. 15% of what it wipes stays on the carpet's tile.
- **Persistence:** indoor dirt never fades. Rain and snow wash outdoor dirt at the stain rate.
- **Scope:** colonists and raiders only; animals track no dirt.
- **Implementation:**
  - Dense `cells` with a sorted `nz` list.
  - `tread` holds per-tile floor and carpet flags, rebuilt with the rooms, so lookups are O(1) per step.
  - Every dirt write goes through `addDirt`, and every stain change through `stainMoved`. Both feed `filthMoved`.

### Cleaning (`clean.go`)
- **Queue (`cleanQ`):** Home tiles whose dirt plus stain reaches 120. A tile leaves only when it hits 0 or stops being Home. This is the hysteresis: a half-swept tile stays queued.
- **`assignClean`:** picks the nearest tile by (distance, index), skips tiles already reserved, makes at most 16 reachability probes, then reserves the tile.
- **Each work tick** removes 20, dirt first, then stain.
- **Work type:** Clean is on by default and last in the tie-break order. Older saves switch it on once (`CleanWired`).

### Home area (`home.go`)
- Painted with `DesignateHome`.
- Auto-grows by 2 tiles around every completed building except floors and graves.
- A new colony starts with a 13×13 Home around its landing spot.
- Older saves get Home around their buildings.
- `WorldView.Home` / `HomeVer`.

### Room cleanliness (`rooms.go`)
- Rooms lose up to 10 impressiveness: `filth.Penalty`, with an average of 200 filth per cell costing the full 10.
- `afterSleep` follows the lowered score.
- `RoomView` gains `Filth`, `Cleanliness` and `FilthPenalty`.
- `roomSum` is updated incrementally and stays exact: incremental sums are skipped while the rooms are being rebuilt.

### Content
- **Cotton** (9 days) yields 5 cloth. Wheat also drops 4 straw.
- **Carpets:**
  - Crafted at the workbench (straw ×10, wood ×6) or the tailoring bench (cloth ×8, leather ×6).
  - Installed as a walkable 1×1 carpet building whose material is the carpet item. Haulers deliver it, and deconstructing returns it.
  - Stockpiles take cloth, straw and carpets.
- **Build command:** it now swaps any material a building can't use for the default (a carpet gets a carpet item).

### UI and renderer
- **Architect:** Zones > Home / Clear home; Furniture > Carpet with a material picker and its catch %.
- **Work tab:** the Clean (Cln) column.
- **Inspectors:** "Dirty boots N%"; carpet catch; cloth and straw shown as materials.
- **Room labels** carry the cleanliness ("bedroom, dirty (-3)"). The hover panel shows Home area.
- **Home overlay:** a HUD toggle, held on while a Home tool is in hand.
- **Crop picker:** "cloth, not food" and "+4 straw".
- **Drawing:** dirt as boot prints and smudges (`dirt_draw.go`, after floors and before blood). Carpets are drawn under pawns.
- **Cleaning:** a broom pose with dust and the procedural `sfx.sweep`.
- **Art:** cotton stages, cloth bolt, straw bundle, the carpets (laid and rolled), the broom, and icons (ART §16.3).

## Verification
- **Coverage:**
  - `filth` 100%
  - game 88.5%
  - renderer package 71.7% (it was already below 80%)
  - construction 65%, item 71%, recipe 65% (all three were already below 80% before this work)
- **Tests:** the full `-race` suite is green.
- **Determinism:** the determinism and SameSeed tests pass ×3. A seeded property test with 3,000 random operations checks the incremental sums against a full recount.
- **Benchmarks:**

  | Benchmark | Result |
  |---|---|
  | DirtStep | 19 ns |
  | AssignClean (4,000 queued) | 0.012 ms |
  | RoomFilthRebuild (100²) | 0.032 ms |
  | StainDecay | 0.081 ms |
  | ColonyHour | ±0% |
  | DrawArt | +7% (2.65 ms) |
  | DrawDirt (1,000 tiles) | 25 µs |

- **Checks:** `make art-check` (7,588 sprites, 0 errors) and `make audio-check` pass.
- **Captures:** the Home tool with its overlay, the Carpet tool with the material picker, and the dirt atlas.
- **Sim trace (`roofs` scenario):** after 4 h, 21 indoor tiles carry a faint dirt trail (max 34). Build-up is slow on purpose.
- **Sim trace (`lategame` scenario):** no dirt appears, because its houses aren't detected as rooms (0 rooms, an existing quirk of the scenario), so every tile counts as outdoors.

## Known limitations
- Dirt is subtle at first: a tile needs many trips to reach the 120 cleaning threshold.
- No vomit or trash filth, and animals don't track dirt.
- Floors have no cleanliness or beauty properties of their own.
- No setting to turn off auto-expansion of the Home area.
- No firefighting or repair inside the Home area yet.
- The cloth carpet has a fixed colour; it isn't tinted.
