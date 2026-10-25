# MAPGEN-3: cave plants, road re-routing, bridges, sealed vaults (+ README)

## Context
MAPGEN-2 left four gaps, and you sent a detailed spec, diagrams and screenshots for closing them. The spec describes three **separate systems that share one map**:
1. **Cave vegetation** (ecology: plants, light, caves).
2. **Bridges** (a construction foundation layer, terrain support, pathing).
3. **Sealed ancient rooms** (generated structures: fog, triggers, encounters).

The fourth item is **road re-routing** in validation.

Your choices:
- **Sealed rooms:** a hive threat plus sleeper caskets.
- **Bridges:** built by players and generated on roads.
- **Load:** light only, which the spec refines as follows:
  - a wooden wall is fine;
  - stone and steel walls are refused;
  - flooring is refused.

**Originality (AGENTS.md):** the spec quotes RimWorld wiki stats and decompiled constants. We follow its **behaviour and architecture**, but every name, number, text and sprite is our own:

| Spec term | Our name |
|---|---|
| Bryolux | **duskmoss** |
| Ancient shrine/danger | **sealed vault** |
| Cryptosleep casket | **sleeper casket** |
| Ancient danger warning | **"Ominous walls"** letter |

We copy none of its numbers: not 12 wood / 1,500 work, 51% light, 60 days, +30 path cost, 15% glow, 4×3 modules, 65% mechanoids, 1–5/1–2 counts, or 25%. Screenshots and wiki images are only references for feel, never traced.

**Generation order** (the spec's dependency chain, mapped onto our pipeline):
1. World tile.
2. Geography: elevation, rock, caves, water.
3. Environment: rocks, roofs, terrain.
4. Structural features: road bridges in the `roads` step; vault footprints in a new `vaults` step right after `start`, before ruins.
5. Populate: vegetation incl. cave plants (never on bridges, vault walls or caskets); vault contents in mapgen; defenders and occupants in the game.
6. Finalize: validate (reachability, invariants, roads); then in the game, fog, rooms and vault triggers.

---

## 1. Cave vegetation (duskmoss, ordinary moss needs light)
**Domain:**
- **Cell light.** `lighting.Cell(outdoor Level, roofed bool, artificial float64) Level`. An unroofed cell gets the outdoor daylight; a roofed cell gets only artificial light.
- **Game light grid** (`game/light.go`, new). Artificial light per cell from lamps (radius 6, 0.7 at the source, falling off) and campfires (radius 5, 0.6), plus duskmoss's own faint glow (0.1, radius 1).
  - It is cached and rebuilt only when `structVer` changes or a lamp, campfire or duskmoss is added or removed.
  - `cellLight(x, y)` = max(outdoor if unroofed, artificial).
  - The renderer's existing sprite lighting is unchanged: this is the simulation's light.
- **plant.Def** gets new fields, all ours:
  - `MinLight` (0 = no rule, so every existing plant behaves exactly as today);
  - `DiesInLight` (light level that harms it, 0 = never);
  - `Glow`;
  - `Cave` (eligible on cave floor).
- **Ordinary moss:** `MinLight` 0.35. It grows only on lit cells (daylight outdoors, or lamplight), so it doesn't grow in dark caves.
- **New species `Duskmoss`** (appended):
  - a slate-blue tangled cave moss with `MinLight` 0, `DiesInLight` 0.45 and `Glow` 0.1;
  - very slow (GrowTicks ≈ 40 days);
  - `WalkCost` 125 (its strands catch feet);
  - Browse (a little nutrition for grazers), Ground, `Cave`;
  - ColdKill −40 °C, HeatKill +45 °C;
  - growth on rough or smoothed stone treated as fertility 40 for Cave species (others still 0 there).
- **Light damage.** While `cellLight ≥ DiesInLight`, a duskmoss loses 8% growth per in-game hour of exposure (in `growChunks`; it withers to nothing and is removed with the message "Duskmoss withered in the light" once per area). Darkness lets it regrow.
- **Hysteresis:** a plant stops being harmed only below 0.35, so a lamp flickering at the threshold doesn't flap (a no-flap test).

**mapgen** (`steps/vegetation`, a cave pass after the surface plants):
- **Eligible:** cave-floor cells (`F.Cave`, terrain Stone or Gravel) that are not Blocked, not on a cave-floor puddle, and **dark** (the cave cell has a natural roof).
- **Placement** (your cell diagram):
  - duskmoss in small tight clusters of 1–4 touching cells (mostly 2–3), seeded on about 7% of eligible cells, with clusters never touching;
  - roughly 15–25% of dark cave floor covered; most of the floor stays bare.
- **Cave mouths:** lit cave cells (unroofed or within 2 of open sky) may get an occasional **ordinary moss** where the biome lists moss (the "another eligible plant" in your diagram). They never get duskmoss.
- Growth starts at 30–100%.
- No grass in caves. Plants never go on bridges, vault cells or features (`Blocked`).

**Art and renderer:**
- **Duskmoss sprite:** low, tangled blue-grey strands spreading from a root mass, in a dark cool palette (slate blue, grey-blue, muted blue-green, charcoal). No contour. Not neon, not crystal.
- **Renderer:** a faint cool point light at each duskmoss in the sprite lighting (subtle: it must not light up the cave).
- **Inspector:** the plant readout shows "Needs darkness" / "Needs light" and "Withering in the light" when harmed.

**Tests** (from the spec's acceptance table, as ours):
- moss grows on a lit cold-biome cell; moss doesn't grow in a dark roofed cave;
- duskmoss exists and grows in a dark cave;
- duskmoss near a lamp, or under a removed roof, withers and is removed (with the no-flap test);
- the duskmoss walk cost is felt;
- mapgen:
  - duskmoss only on dark cave floor, in clusters of 1–4 that don't touch;
  - ordinary moss only at lit mouths;
  - nothing on Blocked cells;
- the light grid rebuilds only on change;
- legacy goldens unchanged (existing species have `MinLight` 0).

## 2. Road connectivity and re-routing (validation)
- **Contract:** `Fields.RoadEnds [][2]mapgen.Point` (edge point, junction), written by the roads step.
- **roads package:** export `Route(c, from, to)` (the A*) and `Lay(c, path)` (straighten crossings, smooth, rasterise, bridges). If A* fails in the step, retry with rock cut at 25 everywhere; log only a final failure.
- **validate** `roadsConnected`, after the start repairs and carving:
  - BFS over road cells (Track, bridge cells, fords) from each edge point to its junction;
  - on a break, re-route from the farthest connected road cell with `Route` + `Lay` and log "road N re-routed";
  - if impossible, revert the dangling pieces to plain ground (logged).
- **Tests:**
  - a fixture with a rock blob or water gap painted over a road gets re-routed;
  - every road connects over 30 seeds × profiles;
  - connected roads untouched.

## 3. Bridges (a foundation layer)
**Model** (spec §2 / your layered-cell diagram). Each cell holds independent layers:
- terrain (river, marsh, mud: never changed by a bridge);
- **foundation** (the bridge);
- optional supported structure;
- items, pawns, plants.

The effective support for building is the higher of the terrain's and the foundation's. If the bridge is removed or destroyed, the terrain's limits return.

### 3.1 Domain (lead contract)
- **Support levels** `construction.Support` (None, Light, Medium, Heavy).
  - Terrain support (new `terrain.Support(t)`): water types None; every land type Heavy, so today's building rules on land are unchanged.
  - Kind requirement: `construction.Requires(kind, stuff)`:
    - walls of stone blocks or steel need **Heavy**;
    - wooden walls, doors, fences, gates and furniture need **Light**;
    - floors and carpets need **Heavy**, so flooring is refused on a bridge per the spec.
- **Bridgeable terrain** (new `terrain.Bridgeable(t)`): ShallowWater, DeepWater, Marsh, Mud. **Deep ocean is excluded:**
  - mapgen marks the sea (`Result.Ocean []bool` from the coast mask);
  - the game keeps a saved ocean bitset for new colonies;
  - legacy maps have no ocean mask, so every deep water there counts as bridgeable river.
- **Kind `construction.Bridge`** (appended):
  - wood only (vanilla-faithful: no stone or steel bridge);
  - a new `LayerFoundation` (appended), so a cell can hold a foundation and an object;
  - our numbers: Wood 6, Work 110, Salvage 3 (half back on deconstruction);
  - flammable like other wooden buildings (if the fire system burns wooden buildings: verify; burning destroys it);
  - provides Light support.
- **Research `research.Bridges`** (appended): a cheap first-tier project unlocking `build:bridge`.
- **Reasons:**
  - `ReasonNotBridgeable` "needs water, marsh or mud";
  - `ReasonOpenSea` "too deep to bridge (open sea)";
  - `ReasonNoSupport` "needs firmer ground than a bridge";
  - `ReasonBridgeInUse` "remove what stands on it first".

### 3.2 Game
**Walkability.** One helper, `g.cellWalkable(x, y)` = (terrain walkable OR a finished bridge) AND no layer blocker. It is used at every site writing the walkable bit:
- `world.go` setTerrain;
- `move.go`;
- `placement.go` removal;
- `tick.go` completion;
- `setup_mapgen.go` placeGenerated;
- `burial.go`.

Completion sets the tile walkable.

**Movement.** A bridge moves at 100% (it uses `treadFloor`, ignoring the wet ground's slowness) and doesn't soak walkers (no wading wash or "Soaked"). Pathing prefers it to wading. Raiders and animals use it too.

**Placement.**
- A bridge only on bridgeable non-ocean terrain without one already.
- On a bridged cell, a kind is allowed only if `Requires ≤ Light`. Wooden walls and furniture are fine; stone and steel walls and floors are refused with `ReasonNoSupport`.
- On plain water, nothing but a bridge.

**Building access.** Frames over deep water are worked from an adjacent walkable or bridged cell, so spans grow out from the shore. Verify the construct job's reach (adjacent work positions); add an adjacency rule if needed.

**Removal and destruction.**
- Deconstructing a bridge is refused while a structure stands on it.
- When a bridge is **destroyed** (damage or fire), whatever it supports is destroyed too:
  - the wall or furniture breaks, with no refund;
  - items on deep water are lost, with the message "<item> sank";
  - items on shallow water, marsh or mud stay;
  - pawns standing there are moved to the nearest walkable cell (on deep water they'd swim: just relocate).
  - Then walkability is recomputed.

**Water helpers.** `isWaterAt`, `dryWalkable` and `waterPlacement` treat a bridged cell as dry footing. Stockpile zones are allowed on bridges; growing zones are not.

**Generated bridges.** `applySite` places `Result.Bridges` as finished wooden bridges.

**Saves.** The foundation is a building (gob), and the ocean bitset is additive.

### 3.3 Generated road bridges (mapgen `steps/roads`), per your diagram 2.6
- **Where:** where a road crosses **river or lake water** (shallow or deep, never ocean), it crosses on a **wooden bridge**: a straight, 2-wide, contiguous strip from bank to bank, so it reads as brown road → planks → brown road.
- **Cost:** water costs 12 per cell for routing and diagonal water steps cost extra, so crossings come out square to the flow and short.
- **Straightening:** at each crossing, take the last dry bank cell before the water and the first after it, and replace that stretch with a straight 2-wide strip snapped to the river's local normal (from `F.RiverLine`) when the banks allow, otherwise the chord.
- **Cells:** every water, marsh or mud cell in the strip becomes a bridge cell (`Result.Bridges`, `F.Road`; terrain unchanged); dry cells become Track.
- **Marsh/mud crossings without water:** Track as today.
- **validate:** bridge cells count as walkable for reachability, road connectivity and I6 (water under a bridge is fine).
- **Tests:**
  - a road over a river makes one straight, 2-wide, contiguous bridge between dry banks;
  - Track touches both ends;
  - no ocean bridges;
  - deterministic.

### 3.4 Art and renderer
**Art** (spec 2.7 + your screenshot's feel, original):
- a **flat, low timber platform** in warm medium brown, desaturated tan and amber-brown, with repeated rectangular planks and darker seams;
- a slightly darker edge beam on sides open to water;
- no towers, cables or tall railings;
- adjacent cells align into one structure.

Produced as 16 connection variants by open-side mask (planks run across the span, so the mask decides the direction), plus a construction frame and a `ui_icon/bridge`.

**Renderer:**
- the foundation layer draws after terrain (over the animated water) and before floors and objects;
- a thin shadow on the water along open edges;
- Architect tool "Bridge" in the structure category (placeFill), locked by research;
- inspector "Wooden bridge" with support "light: no stone walls or floors";
- hovering a bridged cell: "Bridge over deep water" / "over marsh".

### 3.5 Tests (from the spec table)
- placement:
  - over a shallow river: valid;
  - over soil or ocean: invalid;
- completion makes deep water walkable and pathable, and the path uses it;
- a wooden wall on a bridge is allowed, a stone wall refused, a floor refused;
- destroying the bridge destroys its wall, sinks items on deep water and relocates pawns;
- deconstruction is refused while in use;
- the research lock;
- save round trip;
- no "Soaked" on a bridge;
- legacy maps (no ocean mask) still allow river bridges.

## 4. Sealed vaults (ancient rooms: dormant hive + sleeper caskets)
### 4.1 mapgen: new step `features.Vaults`, after `start`, before `ruins`
**Attempts.** One vault per 250×250 of map area (rounded, minimum 1, at most 3). Attempts can fail (small maps, little rock), and the map then skips them rather than forcing one. None on SeaIce.

**Layout** (original, compositional, per your footprint diagram):
- **Casket hall:** `k` casket bays across (k = 1–4), each 3 cells wide holding one 1×2 casket, with 1–2 rows of bays facing each other.
- **Contents:** a strip in front of the bays holding loot stacks and defender spots.
- **Shell:** a 1-cell intact wall ring.
- **Sizes:** the interior is (3k + 1) wide × 7 or 10 tall; with the ring, 6–15 wide × 9–12 tall.

**Placement:**
- the anchor cell is natural rock;
- the whole footprint plus a 1-cell margin is inside the map;
- ≥ 70% of the footprint plus a 2-cell ring is rock (embedded, at least partially);
- no cave, water, road, other vault or feature in the footprint;
- ≥ 30 from the landing spot;
- the interior is not reachable from the landing spot (it is sealed: the walls guarantee that).

**Carving:**
- **Interior:** terrain → StoneSmooth, with constructed **floor tiles** (Floor buildings of the local stone's blocks; a darker pad under each casket).
- **Ring:** intact **walls** of the local stone's blocks (70%) or steel (30%). They are buildings, not rock, so the room reads as artificial against the mountain.
- **Fields:** natural roof kept (thick where deep); `F.Rock` cleared; `Blocked` set on every vault cell.

**`Result.Vaults []Vault`:**
- `{ID, X0, Y0, X1, Y1, Walls []Point + WallStuff, Floors, Caskets []Casket{X, Y, Occupant}, Loot []Loot, DefenderSpots []Point}`.

**Occupants** are rolled at generation from the step's stream and hidden. Our table:

| Occupant | Share |
|---|---|
| Grateful sleeper | 30% |
| Hostile ancient | 25% |
| Sleeper + one hive crawler | 15% |
| Two hive beetles | 10% |
| Empty | 10% |
| Long-dead remains with gear | 10% |

**Loot** (our table, 3–6 stacks):
- Components 4–12;
- IndustrialMedicine 3–8;
- Silver 150–450;
- Gold 20–70;
- a Rifle (35%);
- a prosthetic, GlassEye or SparkTablet (15%).

### 4.2 Game (`game/vault.go`, `game/casket.go`)
**State machine per vault**, saved additively (`saveState.Vaults`): Generated → Warned → Breached → CasketsOpened → Cleared.
- `Building.Ancient bool`, additive, on vault walls, floors and caskets: not colony wealth, not a Home anchor, not repaired; deconstructable as normal once reachable.
- **Defenders** (separate from casket occupants, spec 3.14):
  - At `applySite`, spawn dormant hive insects on `DefenderSpots`. The count comes from threat points at landing: n = clamp(round(threatPoints/50), 2, 6) per vault (our divisor); brutes ≤ n/3.
  - **Peaceful configurations get none:** ThreatScale 0 (or the lowest preset) or `incidentDisabled("hive_swarm")`. The room and treasure stay.
  - New `animal.Animal.Dormant`: dormant animals skip thinking and needs entirely; inspector "Dormant".
- **Approach warning** (spec 3.16–3.17):
  - every 60 ticks, for each vault still in Generated, check whether any colonist stands inside its bounds expanded by 2;
  - if so, post the letter **"Ominous walls"** ("<Name> stops near the smooth walls in the rock. Something old is sealed inside, and it does not feel empty.") and set Warned.
  - It fires once, and never after the interior is revealed. It does not open anything.
- **Breach:**
  - when `fogOpen` reveals any interior cell (a wall deconstructed or destroyed, mining around it), the vault becomes Breached;
  - its defenders wake (`Dormant = false`, `Manhunter = hiveForever`, `Mode = Attack`);
  - danger letter "The vault is open. Hive insects stir inside!" and a History entry.
  - Rooms and temperature recompute through the existing room rebuild. Breaching does **not** open the caskets.
- **Caskets** (`construction.SleeperCasket`, 1×2, Placed, blocks movement, Ancient; `Building.Sleeper` = sealed with an occupant code; `CasketGroup` = vault ID):
  - **Opening:** the inspector action "Open casket" (typed command `OpenCasket{X, Y}`; a colonist walks there and works 250 ticks), or any damage to a casket (combat, fire).
  - **Grouped:** opening or damaging one opens **every casket in its group** (spec 3.12), each releasing its occupant.
- **Outcomes:**
  - grateful sleeper: generated via `pawngen`, wakes dazed and joins (the `wandererJoins` path), letter "A sleeper wakes and asks to stay";
  - hostile ancient: a one-person raider group with a weapon;
  - sleeper + crawler: both, the crawler hostile;
  - beetles: hostile hive insects;
  - empty: "The casket is empty";
  - remains: a corpse with old gear (or, if corpses don't exist, an item drop "old remains" with gear: verify).
- **After opening:** an empty casket can be deconstructed for steel (salvage 20). There is no reuse as a bed this round.
- **Cleared:** when every defender and hostile occupant of the vault is dead or gone (History entry).
- **Fog:** the interior starts hidden because it is sealed in rock behind intact walls. Contents exist in the simulation and are filtered from read models until revealed; the exterior walls show once adjacent ground is revealed.

### 4.3 Art, sound, UI
**Art:**
- `structure/sleeper_casket_closed` / `_open`: a 1×2 heavy metallic capsule with bevelled panels, a frosted viewport and a dim status light, closed and lid-slid-open; original.
- `ui_icon/open_casket`.
- Walls and floors reuse the stone, steel and floor sprites.

**Sound** (procedural): `sfx.casket_open` (seal crack, hiss, lid grind). The breach reuses the hive call sounds.

**UI:**
- inspector: casket "Sleeper casket (sealed)" + the Open button, its reason if no colonist can reach it, and "Opens every casket in this vault" in the tooltip;
- dormant insects read "Dormant";
- the letters above;
- the History tab.

**Debug views:** `cmd/mapgen` resources/composite draw vaults (wall ring, caskets, loot, defender spots) and bridges; the in-game "Map fields" terrain layer shows bridges and vault floors.

### 4.4 Tests (spec table, as ours)
- mapgen:
  - a vault is fully enclosed, embedded in rock, inside the map and not overlapping;
  - attempts fail gracefully;
  - layout and loot within tables;
  - deterministic.
- warning appears once when a colonist approaches; ignoring it leaves the vault untouched;
- breaching a wall reveals the room and wakes the defenders;
- peaceful settings give no defenders;
- opening one casket opens the whole group, and each occupant branch is covered with seeded fixtures (grateful joins, hostile attacks, empty, remains);
- a hostile occupant is hostile, a grateful one is not;
- a cleared vault's walls and caskets can be deconstructed;
- save round trip of vault state, Dormant, casket fields and Ancient;
- legacy goldens unchanged.

---

## 5. README and docs
- **README:**
  - the map-generation bullet gains duskmoss in dark caves, re-routed roads, road bridges and sealed vaults;
  - a new bullet: building wooden bridges (research, light support: wooden walls yes, stone and steel walls and floors no; destroyed bridges take what they hold);
  - a new bullet: sealed vaults (the warning, breach, dormant hive, grouped sleeper caskets with uncertain occupants);
  - the `cmd/mapgen` note lists vaults and bridges.
- **Checklists** (vanilla):
  - "Ancient ruins/debris" → `[x]`;
  - Water note: bridges;
  - a bridge construction item if listed;
  - "Wild vegetation": duskmoss and light rules;
  - the lighting item `[-]` (simulation cell light for plants);
  - "Geothermal" unchanged.
- **Checklists** (UI): the Architect bridge tool, the casket inspector action, and the plant readout.
- **Handoffs:** pick the next free numbers at the time (the other session writes handoffs too). SIM (cave plants + light), SIM (roads + bridges), SIM (vaults), ART (bridge, duskmoss, casket).
- Copy this plan to `.cursor/PLAY014_PLAN.md`.

## 6. Execution (parallel-work rules, ≤ 15 agents, disjoint ownership)
### Phase 0: lead contract (one commit)
**mapgen:**
- `Fields.RoadEnds`;
- `Result.Bridges`, `Result.Ocean`, `Result.Vaults` with the `Vault`/`Casket`/occupant types;
- the `features.Vaults{}` stub after start;
- I2/I5 extended for vault cells and bridges.

**construction:**
- `Support`, `Requires`, `LayerFoundation`;
- `Bridge`, `SleeperCasket`;
- `Building.{Ancient, Sleeper, CasketGroup, Occupant}`.

**Other domain:**
- `terrain.{Support, Bridgeable}`;
- `research.Bridges`;
- `plant.Def.{MinLight, DiesInLight, Glow, Cave}` and the `Duskmoss` species def;
- `animal.Animal.Dormant`;
- `lighting.Cell`.

**Game:**
- the placement reasons;
- `saveState.{Vaults, Ocean}`;
- the `OpenCasket` command stub;
- `g.cellWalkable` and `g.cellLight` stubs.

**Assets and sound:**
- asset ids;
- the `sfx.casket_open` recipe.

Build and all tests green.

### Phase 1 (parallel)
| Agent | Model (why) | Owns |
|---|---|---|
| **CAVE-PLANTS** | opus (light grid caching, growth/withering hysteresis, game + mapgen) | `game/light.go`, plant growth light rules in `plant_growth.go`, duskmoss/moss defs data, `steps/vegetation` cave pass, tests |
| **ROADS** | opus (A*, straight crossings, re-routing, validate invariants) | `steps/roads` (Route/Lay, straight bridges, RoadEnds, ocean avoid), `steps/start/validate.go` + `metrics.go`, `Result.Ocean` from `steps/water` (one hook), profile-test additions |
| **VAULT-GEN** | sonnet (layout against a spec) | `steps/features/vaults.go` + tests, loot and occupant tables |
| **BRIDGE-GAME** | opus (walkability sites, support rules, destruction cascade, saves) | game `bridge.go` + the walkable-bit sites, water helpers, placement support rules, destruction cascade, zones, generated bridges in applySite (hook), ocean bitset |
| **VAULT-GAME** | opus (dormant AI, warning trigger, breach, grouped caskets, occupant spawns, saves) | game `vault.go`, `casket.go`, the fog hook, Dormant handling, applySite vault hook, letters/history, the OpenCasket command and job |
| **ART** | sonnet (Blender) | bridge deck (16 masks), frame, `ui_icon/bridge`; duskmoss (with growth stages as other plants, no contour); casket closed/open; `ui_icon/open_casket`; render, normals, art-check |
| **RENDER** | sonnet | ebiten: foundation layer draw + water shadow, Architect bridge tool, duskmoss glow point light, casket draw + inspector button, Dormant label, plant light readouts, hover lines; `cmd/mapgen` bridges/vaults drawing |

Phase 1 rules for every agent:
- TDD;
- targeted `go test -race`;
- ≥ 80% coverage;
- no shared config;
- no commits;
- report in 200 words or fewer.

The lead verifies and commits per area.

### Phase 2: lead verification
1. `go build ./...`, gofmt, vet, staticcheck (no new findings).
2. `go test -race ./...` (only the known local atlas failure allowed).
3. Determinism ×3; legacy and scenario goldens unchanged.
4. mapgen goldens re-pinned once, with the reason; `BenchmarkGenerate500` ≤ 2 s.
5. `make art-check`, `make audio-check`.
6. Visual review:
   - `cmd/mapgen`: duskmoss clusters in dark caves, straight road bridges over rivers, vaults embedded in mountains;
   - in-game captures: a road bridge, a vault before and after breach, a duskmoss cave lit by a lamp.
7. The fps probe on a grown colony (the light grid and walkable helper must not regress).
8. Docs, README, checklists, handoffs; commits on `main` per area; no push.

## Critical files
- `internal/domain/world/{lighting, plant, terrain}`
- `internal/domain/world/mapgen/{mapgen.go, pipeline/pipeline.go, steps/{vegetation, roads, start/validate.go, features/vaults.go, water}}`
- `internal/domain/building/construction/construction.go`
- `internal/domain/research/research.go`
- `internal/domain/animal/animal.go`
- `internal/application/game/`:
  - `{light.go, plant_growth.go, bridge.go, vault.go, casket.go, world.go, move.go, placement.go, tick.go, setup_mapgen.go, water.go, zones.go, fog.go, animals.go, save.go}`
- `internal/adapters/inbound/ebiten/`:
  - `{draw_art.go, archtools.go, building_inspector.go, plant_inspector.go, bridge_draw.go}`
- `assets/source/blender/artlib/families/{structure.py, plant family}`
- `cmd/audiogen/recipes.go`
- `cmd/mapgen/render`
- `README.md`
