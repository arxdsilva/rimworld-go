# PLAY-015: Areas, pens, Home, snow clearing and fire

## Context
You pasted a full spec of RimWorld-style areas, zones, colonist and animal restrictions, pens and the Home area, and asked for a plan. Exploration shows partial foundations with large gaps.

**Allowed areas** (SIM-046) exist minimally:
- a 10-area bitset with paint and rename;
- the manager is reached only from the Assign tab and has no Invert, Delete or colours;
- assignment is a cycle button in the Assign tab;
- enforcement is a hard path wall plus a job-start target check;
- no Home choice, no animals, no overlay, no Expand/Clear submenus;
- the haul destination isn't checked, and sleep ignores areas.

**Pens** are painted zones. There is:
- no marker, no flood-fill enclosure, no settings, no roping;
- no livestock-versus-pet split (cats behave like cows);
- gates and doors let every tame animal through;
- no Food or Auto-cut panels;
- nothing for livestock without a pen.

**Home:**
- it auto-expands around buildings only, with no on/off toggle;
- repair ignores Home;
- roof plans are one-shot flags in the Structure category, not areas.

**Fire** doesn't exist: the domain package is a stub, lightning is only a visual, and nothing burns. **Snow** is one map-wide number.

Your decisions:
- **Scope:** I read your "Also a fire system" answer as cumulative: areas, pens and Home, **plus per-tile snow and a snow-clearing area**, **plus a full fire system**. Tell me if you meant to drop snow clearing.
- **Pens:** painted pens **auto-convert** to marker pens on load.
- **Allowed areas:** **soft, spec-faithful**. Only destinations must be inside; paths prefer the area but may cross outside.
- **Unpenned livestock:** roams, raises an alert, and **may wander off the map**.

**Originality:** we follow the spec's behaviour and architecture. Names, numbers, text and art are ours:
- 10 areas instead of 8;
- our pen-marker cost;
- our fire rates;
- no "Snow and sand removal" wording, because there are no sand drifts in the game. Ours is **"Snow clearing area"**.

**Goldens.** Some behaviour changes are by design:
- gates hold livestock;
- areas become soft;
- per-tile snow;
- lightning fires.

These move some scenario goldens (the pen scenario, grown, the rain/storm ones). Each re-pin will be justified field by field, as the ENG-003 re-pins were. `TestTemperateGenerationUnchanged` and map generation must not move.

---

## 1. Allowed areas (rebuilt to the spec)
### 1.1 Domain (`internal/domain/world/area`)
- **Stable ids.** Each `Area` gets `ID uint16` (stable; never reused in a colony), `Name`, `Color` (assigned from our 10-colour palette at creation) and `Bits`. `MaxAreas` stays 10.
- **New methods:**
  - `Delete(id)`;
  - `Invert(id, w, h)`, which flips only real map cells and leaves the padding bits alone;
  - `ByID`;
  - `Clear`.
- **Migration:** old saves' positional areas get ids 1..n in order. Assignments map index i to id i+1.

### 1.2 Game
**Assignment values:** each pawn stores an `AreaRef`:
- 0 = Unrestricted;
- a special value = **Home** (uses the live Home mask);
- otherwise an area id.

Deleting an area resets every pawn using it to Unrestricted (message "N pawns are now unrestricted").

**Commands:** `CreateArea`, `DeleteArea`, `InvertArea`, plus the existing `PaintArea`, `RenameArea` and `AssignArea`. `AssignArea` now accepts animals and the Home ref.

**Soft traversal** replaces the hard `areaQuery`:
- for restricted pawns, path costs outside the area are ×4;
- A* may still cross outside when that's the only route;
- the reachability check uses normal regions and is no longer area-limited.

**Destination eligibility** (inside the area or Home) for:
- every work giver's target **and** the haul destination (`DestX/Y`);
- hunting chases: the prey leaving the area for 600 ticks abandons the hunt;
- food;
- **bed and sleep spot choice** (`Sleep` is removed from the exempt list; with no bed inside, the pawn sleeps on the floor inside the area);
- recreation;
- wander.

**Overrides:** drafted, mental break, fleeing, the combat Attack response, rescue, tend and surgery (as today), and animals following a drafted master.

**Animals with areas** (non-pen tame animals: see §2.1): area-respecting wander, food, sleep spot, and trained hauling.

**Cleaning, repair and firefighting** need the target inside Home AND inside the pawn's area. Unrestricted means Home only.

**Read model:** `AreaView{ID, Name, Color, Cells, Ver}`. Cells are shared copy-on-write per area version.

### 1.3 UI (spec §4–§8)
**Architect → Zone:**
- "Expand allowed area ▸" opens a submenu: one row per area (colour, name) plus "Manage allowed areas…";
- "Clear allowed area ▸" opens the same list;
- picking an area arms the paint tool;
- left-drag paints rectangles; Escape cancels.

**Overlay:** while an area tool is armed, or its row is hovered in a selector, that area's cells draw as a translucent tint in its colour. Cells are baked per chunk by area version, in the physical-pixel map pass.

**Manage allowed areas dialog** (centred modal). Each row has:
- a colour swatch;
- the name;
- Rename (inline edit);
- Invert;
- a red ✕ Delete (with a confirm when pawns use it).

Below the rows: "New area" (disabled at 10) and "Close". It opens from the Architect submenu and from the Schedule, Animals and Assign tabs.

**Selectors:**
- **Schedule tab:** an "Allowed area" column with a dropdown per colonist: Unrestricted, Home, each area, Manage….
- **Animals tab:** an "Area / Pen" column (§2.6).
- **Pawn inspector:** a line "Area: Indoors", which opens the same dropdown.
- **Assign tab:** its Area column is removed (per the spec, areas belong to Schedule and Animals).

**Tests:**
- domain: delete, invert (padding untouched), ids;
- game: soft path crossing between two disconnected islands;
- destination filters per job (incl. haul destination, beds, food, hunting abandon);
- the overrides matrix;
- Home as an area;
- deleting resets references;
- animals in areas;
- save and old-save migration;
- UI: dropdowns, manager actions, submenus, the overlay bake.

## 2. Pens (marker-based)
### 2.1 Pen versus area animals
A roster field `animal.Roster.Penned bool` (data, in the creatures mirror too):
- **Penned:** farm species (cows, chickens, sheep, goats, pigs, horses, muffalo-likes, alpaca-likes, ducks, yaks…);
- **Area animals:** pets and trainable animals (dogs, cats, wolves, bears, elephants…).

The flag is listed explicitly per species in the roster and reviewed against our 61 species.

### 2.2 Pen marker and enclosure
**New kind `construction.PenMarker`:** 1×1, 10 wood, Architect → Misc.

**Enclosure** (`game/pens.go`): flood-fill from the marker over cells a *penned animal* can stand on.
- **Barriers:** fences, **closed gates and doors**, walls, rock, deep water, other buildings that block movement.
- **Not enclosed:** the flood reaches the map edge or exceeds 2,500 cells. The pen shows "Not enclosed" and holds nothing.
- **Recompute:** lazily when the structure version changes (a fence built or broken, a gate built). Cached per marker.
- A broken fence makes the pen **Not enclosed**: an alert "Pen 2 is open", and penned animals inside start roaming.

**Gate semantics:**
- penned animals **cannot** pass gates or doors on their own;
- they pass only while being led by a handler;
- area animals pass gates as today;
- wild animals pass neither.

### 2.3 Settings (marker inspector)
- **Animals:**
  - per penned species present in the colony: an allow checkbox plus ♂/♀ checkboxes;
  - "All" and "None" shortcuts;
  - default: every species, both sexes.
- **Food:**
  - grazing nutrition per day (grass and browse plants inside the pen, by growth);
  - herd demand per day for the animals assigned to it;
  - trough stock;
  - a verdict: "Enough grazing" / "Short by N per day: add hay or expand".
- **Auto-cut:**
  - a checklist of plant species (default: trees, cacti, brambles);
  - every 2 hours, matching plants inside an enclosed pen are designated for cutting.

### 2.4 Assignment and roping (Handling work)
**Assignment:** each penned animal has a target pen, the closest reachable enclosed pen accepting its species and sex. It is recomputed on settings or structure changes, or when the animal is tamed or born.

**New job `Rope` (Handling):**
- the handler walks to the animal and ropes it (60 work);
- the animal follows one cell behind the handler;
- the pair passes gates;
- the handler releases it inside the target pen.

**Triggers:**
- an animal outside any accepting pen when one exists;
- an animal in a pen that no longer accepts it.

**Rules:**
- the handler's allowed area applies to the animal's cell and the pen;
- the job is refused and postponed when unreachable.

### 2.5 Unpenned livestock
- **Roaming:** with no accepting enclosed pen, livestock roam freely (wander radius 30 around their current spot, grazing anywhere).
- **Alert:** "Unpenned livestock: N".
- **Leaving:** after 3 in-game days unpenned, each day an animal that is 40+ cells from Home has a 12% chance to wander toward the nearest edge and leave ("Bessie wandered off").
  - Being roped or penned resets the day count; this is the hysteresis.
  - A handler roping it cancels the leave.

### 2.6 Animals tab (spec §8)
New columns:
- **Master** (a dropdown of capable colonists; emits `SetAnimalMaster`);
- **Follow drafted** (a checkbox, for trained animals);
- **Area / Pen:**
  - area animals: the dropdown (Unrestricted, Home, areas, Manage…);
  - penned animals: the read-only pen name ("Pen 1"), "Roaming" or "Being roped".

Existing columns stay.

### 2.7 Migration and old saves
- **Painted pens:** on load, each painted pen zone becomes a `PenMarker` at the free walkable cell nearest the zone's centroid, with default settings. The zone cells are cleared.
- **Unfenced zones:** a zone that wasn't fenced becomes "Not enclosed", with the open-pen alert.
- **Scenarios:** the pen scenario and other set pieces build markers instead of painting zones.

### 2.8 Tests
- flood fill (enclosed, open at the edge, gate closed, broken fence reopens);
- livestock blocked by gates unless led;
- pets pass;
- roping end to end (handler leads through the gate);
- settings filter by species and sex, and changes re-rope;
- food capacity numbers;
- auto-cut designations;
- unpenned roaming, the alert and leaving (seeded), with the no-flap reset;
- migration of a painted pen;
- the UI inspector and Animals tab columns.

## 3. Home area and other painted masks
- **Auto-expand toggle:**
  - `homeAuto` (default on), saved;
  - command `SetHomeAuto`;
  - a house-icon toggle in the lower-right controls (next to the speed and overlay controls, per the UI spec);
  - turning it off keeps the existing Home.
- **Auto-expand triggers:** finished buildings (as today) **and** newly painted stockpile and growing zones and placed pen markers (2-cell margin).
- **Repair** requires the building inside Home (and the pawn's area).
- **Architect → Zone:** "Expand home area" / "Clear home area" (renamed), with a blue overlay as today.
- **Roof areas:**
  - the "Build roof area" / "Remove roof area" tools move to the Zone category;
  - a new persistent `roof.AreaBuild` bit marks the build-roof area;
  - roofs there are kept built (rebuilt after a collapse) until the area is cleared;
  - the remove area keeps today's `NoAuto` rule.
- **Tests:** the toggle, zone-triggered expansion, repair in and out of Home, persistent roof area rebuild, save.

## 4. Per-tile snow and the snow clearing area
**Snow layer** (`game/snow.go`):
- 0–255 per cell (255 = deepest), modelled on the grass layer (per-chunk lists, a version, saved);
- builds up on **unroofed, non-water** cells while it snows: per climate step, by weather intensity, processed by chunk phase;
- melts with temperature;
- a rain block halves it;
- the global `SnowDepth` becomes the map average (HUD and weather logic).

**Uses switched to per-tile:**
- movement slowness (`weatherSlow`);
- grass regrowth;
- the renderer's snow veil (`WorldView.Snow []uint8` + `SnowVer`; the veil samples per cell).

**Snow clearing area:**
- a painted mask: Architect → Zone "Snow clearing area" / "Clear snow clearing area", cyan overlay;
- command `DesignateSnowClear`.
- **Job `ClearSnow`** (Cleaning work): cells in the area with depth ≥ 15%. Work scales with depth (20–80 ticks), then the cell goes to 0. It respects the allowed area.

**Tests:**
- build-up on open cells only;
- melt and rain;
- a roofed cell stays clear;
- movement slows per cell;
- the clearing job empties area cells;
- save;
- the old-save global depth seeds every open cell.

## 5. Fire system
### 5.1 Domain (`internal/domain/world/fire`, replacing the stub)
**State:** `Fire{X, Y int; Size uint8 (1..100); Age uint32}` and pure rules (all our numbers):
- **Growth:** +4 size per fire step (every 30 ticks) while fuel remains; −6 when no fuel is left; it dies at 0.
- **Spread:** at size ≥ 35, each 8-neighbour with fuel ignites with chance = flammability × size/100 × 0.06, ×1.5 downwind (from `WindX/WindY`), ×0.2 if the neighbour is roofed while the source is outdoors.
- **Rain and snow:** outdoor fires in Rain/Storm/Snow with intensity ≥ 0.3 lose 12 size per step; roofed fires are unaffected.

**Flammability** (new, data):

| Fuel | Flammability |
|---|---|
| Grass | 0.5 |
| Wild plants | 0.6 (trees 0.7, duskmoss 0.9) |
| Crops | 0.6 |
| Wooden buildings and bridges | 0.7 |
| Stone/steel buildings | 0 |
| Items: wood, cloth, leather, straw, hay, corpses, textiles | 0.7–0.9 |
| Food | 0.3 |
| Metal and stone items | 0 |

Terrain itself never burns.

### 5.2 Game (`game/fire.go`)
**Active set:** fires on cells, in per-chunk lists (no global scans); saved.

**Ignition:**
- **Lightning:** during Storm blocks, a seeded sim strike every 1–3 in-game hours on a random outdoor cell. On fuel it ignites; it is shown by the existing flash and thunder (which are now driven by the sim event).
- **Exploding animals:** the existing `burst()` "bursts into flames" hook.
- **Developer action:** "Start fire".

**Damage per fire step, by size:**
- buildings on the cell take HP damage; a broken wooden bridge triggers the MAPGEN-3 cascade;
- plants burn and are removed at the end, leaving an ash stain;
- grass on the cell is cleared;
- flammable items are destroyed;
- pawns and animals standing in fire take damage: humans get `body.Burn` wounds (new `health.Cause` Burn, appended), animals get Injury; adjacent cells take a small amount.

**Heat:**
- room fires add heat into the room like a campfire, scaled by size;
- outdoor fires warm a 5×5 area (like `ventWarmth`).

**Danger:**
- fire cells cost ×20 to path through;
- idle pawns and animals standing in or next to fire step out;
- animals flee fire within 4 cells;
- colonists treat fire as a threat to flee only when caught in it (not when drafted or fighting it).

**Firefighting:**
- `work.Firefight` is enabled: the first work column, on by default for capable colonists;
- new job `FightFire` (beat out from an adjacent cell, −10 size per 30 ticks, skill-free);
- eligible fires must be inside **Home** AND the pawn's allowed area (Unrestricted means Home only);
- a drafted pawn can be ordered "Extinguish" on any fire (a forced order, ignores Home and area).

**Messages:**
- "Fire in the colony!" alert while any fire burns inside Home;
- a message when lightning starts a fire;
- History entries for buildings lost.

**Read model:** `WorldView.Fires []FireView{X, Y, Size}` with a version.

### 5.3 Art, sound, renderer
**Art:**
- `effect/fire_{small,medium,large}_0..3` (flame frames, original, no outline);
- `overlay/ash_0..2` (a burnt ground decal);
- icons `ui_icon/extinguish`, `ui_icon/start_fire` (dev), `ui_icon/pen_marker`, `ui_icon/snow_clear`, `ui_icon/home_auto`, `ui_icon/roof_area`;
- a `structure/pen_marker` sprite (a small post with a painted sign).

**Sound** (procedural): `amb.fire` (a crackling loop, louder than the campfire) and `sfx.lightning_strike` (a crack before the thunder).

**Renderer:**
- flames by size with an animated frame, embers and smoke (existing particles), and orange light from burning cells (the existing light system);
- the ash decal;
- the fire loop positioned per fire cluster;
- the Extinguish order in the right-click menu;
- the fire alert;
- the Work tab's Firefight column is no longer "Not available".

### 5.4 Tests
- spread rate, wind bias, roof shielding;
- burn-out without fuel;
- rain extinguishing outdoors only;
- damage to buildings, plants, items, grass and pawns (a burn wound);
- a bridge collapse cascade by fire;
- lightning seeded and deterministic, only in storms;
- firefighting eligibility (Home and area matrix), drafted Extinguish anywhere;
- pathing avoids fire;
- animals flee;
- heat into a room;
- save round trip;
- performance: a 500-cell blaze ticks in ≤ 2 ms;
- no-flap test for pawns stepping out of fire.

---

## 6. Execution (parallel-work rules; ≤ 15 agents; disjoint ownership)
### Phase 0: lead contract (one commit)
**Commands:**
- `CreateArea`, `DeleteArea`, `InvertArea`;
- `SetHomeAuto`;
- `DesignateSnowClear`;
- `SetPenSettings` (Target marker, encoded species/sex lists);
- `Extinguish` (an order kind);
- `DevStartFire`.

**Domain:**
- `area.Area.{ID, Color}`, the `AreaRef` encoding, `Delete`/`Invert` stubs;
- `animal.Roster.Penned` (the data list reviewed by the lead);
- `construction.PenMarker`;
- `roof.AreaBuild` bit;
- job kinds `Rope`, `FightFire`, `ClearSnow`;
- the `fire` package types and flammability function signatures;
- `health.Cause` Burn.

**Game:**
- read models `AreaView{ID, Color, Cells, Ver}`, `PenView`, `WorldView.{Snow, SnowVer, SnowClear, SnowClearVer, Fires, FireVer, HomeAuto}`;
- save fields;
- stubs for every new hook so the build stays green.

**Assets and sound:**
- asset ids as listed;
- the `amb.fire` and `sfx.lightning_strike` recipes.

### Phase 1 (parallel)
| Agent | Model | Owns |
|---|---|---|
| **AREA-CORE** | opus (soft pathing, destination matrix, migration) | `world/area`, game `areas.go` (+ new area files), job-start and haul-destination checks, sleep/bed filter, animal area rules, Home ref, saves |
| **AREA-UI** | sonnet | ebiten manager dialog, Architect submenus, area overlay bake, Schedule tab column, inspector line, Assign tab column removal |
| **PEN-SIM** | opus (flood fill, gate semantics, roping, roaming/leaving, migration) | game `pens.go`, `rope.go`, gate rules in `fences.go`, assignment, unpenned logic, food capacity, auto-cut, pen-zone migration, scenario pen set pieces |
| **PEN-UI** | sonnet | ebiten pen marker inspector (Animals/Food/Auto-cut), Animals tab columns (Master, Follow, Area/Pen), pen region outline + "Not enclosed" |
| **HOME** | sonnet | game `home.go` toggle and zone triggers, repair-in-Home, roof area bit and rebuild; ebiten Home toggle button, Zone category labels/moves |
| **SNOW** | opus (per-tile layer, chunk phasing, replacing global reads) | game `snow.go`, weather hooks, grass hook, clearing area and job, save; ebiten per-cell snow veil + snow-clear overlay |
| **FIRE-SIM** | opus (spread, damage, heat, determinism, performance) | `domain/world/fire`, game `fire.go` (state, ignition, spread, damage, extinguish, heat, save, read model), the lightning sim event, the burst hook |
| **FIRE-JOBS** | opus (job eligibility, danger pathing, flee) | game `firefight.go` (work type, job, Home∩area rule, drafted Extinguish), fire avoidance in pathing, step-out and animal flee, alerts |
| **FIRE-RENDER** | sonnet | ebiten flames/smoke/light/ash, fire sound positioning, lightning flash from the sim event, Extinguish order UI, Work tab Firefight column |
| **ART** | sonnet (Blender) | the fire frames, ash decals, pen marker sprite, all new icons; art-check |
| **ROSTER** | haiku (data entry from a written list) | `Penned` flags in the Go roster and the art mirror, with a test that every species is classified |

Phase 1 rules for every agent:
- TDD;
- targeted `go test -race`;
- ≥ 80% coverage;
- report in ≤ 250 words.

The lead commits per area.

### Phase 2: lead verification
1. Build, gofmt, vet, staticcheck.
2. `go test -race ./...`.
3. Determinism ×3.
4. Golden re-pins only with written justification (a field-by-field diff note in the test comment).
5. `make art-check` and `audio-check`.
6. The fps probe on a grown colony with a 300-cell fire and deep snow.
7. In-game captures:
   - the area manager and overlay;
   - the Schedule area column;
   - a pen marker with an enclosure outline;
   - a handler roping a cow;
   - a storm fire spreading and being fought;
   - snow being cleared.
8. Docs:
   - handoffs (next free ids);
   - vanilla and UI checklists (areas, pens, Home, fire 12.5, snow);
   - `MAIN_UI_SCREENS_SPECIFICATION` sections for the area manager, Schedule area column, Animals tab and pen inspector;
   - README bullets.
9. Commits on `main`; no push.

## Critical files
- **Domain:**
  - `internal/domain/world/{area, fire, roof, weather}`;
  - `internal/domain/animal/animal.go` (Penned);
  - `internal/domain/building/construction` (PenMarker);
  - `internal/domain/sim/{command, job}`;
  - `internal/domain/ai/work`;
  - `internal/domain/pawn/health`.
- **Game** (`internal/application/game/`):
  - `{areas.go, pens.go, rope.go, fences.go, home.go, roof.go, snow.go, weather.go, grass.go, fire.go, firefight.go, flee.go, heat.go, clean.go, defense.go, save.go, game.go}`.
- **Renderer** (`internal/adapters/inbound/ebiten/`):
  - `{areas_ui.go, archtools.go, schedule_ui.go, animals_ui.go, pen_ui.go, hud.go, environment.go, fire.go, work.go}`.
- **Art and sound:**
  - `assets/source/blender/artlib/families/{effect.py, structure.py, overlay.py}`;
  - `cmd/audiogen/recipes.go`.
- **Docs:** `.cursor/MAIN_UI_SCREENS_SPECIFICATION.md`, both checklists, `README.md`.
