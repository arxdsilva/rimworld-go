# PLAY-010: play-feedback batch (14 items, up to 15 agents)

## Context
You played the game and asked for fourteen changes, listed below with your decisions. Everything stays deterministic and headless in the simulation: only the sim's RNG, no wall clock.
- Every threshold behaviour gets hysteresis or a cooldown, plus a no-flap test.
- Spatial queries use the indexes: the creature index `animal_index.go` (`nearestWhere`, `within`) and the per-cell `plantAt`, `bldgIndex`, `frameIndex`. Nothing scans all creatures or tiles per tick.
- All names, text, numbers and art are our own. The vegetation brief you pasted is reference only: no RimWorld plant names, prose, sprites or balance numbers are copied, and invented RimWorld plants (cave fungi, the vine, the gold-fruit bush, the hay crop) are left out.

| # | Request | Your decisions |
|---|---|---|
| 1 | XS animals too small | Bigger, but every XS stays smaller than every S |
| 2 | Door over a wall | Doors and gates may go over walls and fences, in any combination; the old piece is deconstructed first, then the new one is built |
| 3 | Attacked animals react | Every animal, any size: fight back or flee |
| 4 | Penned or helpless animals | If it can't fight back (pen, flee type, tame hit by its own colonist, too hurt, attacker out of reach) it flees on **every** hit, even if the only open spot isn't the farthest from the attacker |
| 5 | Danger near predators | Predators and pack predators only, chance-based, never while asleep, off on Wanderer and Gentle |
| 6 | Unarmed colonists flee | From an animal coming for them, unless drafted; even mid-order, mid-work or asleep |
| 7 | Colonists don't block | Colonists and tame animals walk through each other; wild animals and raiders still block; in combat (drafted, attacking or fleeing) a colonist never *ends* a move on another colonist's tile |
| 8 | Grass | Blended across the whole tile and into neighbours by composing many small tufts, never a green carpet (your brief §5); grass really grows (shoots fill out after grazing or cutting) |
| 9 | Trees | Visible trunk, a modest canopy that overhangs about half the tile above, canopies that blend; a canopy turns see-through when someone is under or behind it |
| 10 | Vegetation restyle | Every ground plant redrawn to the brief: flat, muted olive, transparent gaps, small composable sprites |
| 11 | New wild plants | Bramble thicket (slows walking), ground moss, desert rosette, ball cactus, elephant ear, fire lily, dandelions; per biome, browsable by grazers |
| 12 | Parallel limit | Up to 15 agents at once |
| 13 | Remove the unused vendor pack | `assets/Free 2D Animated Vector Game Character Sprites/` (203 MB, 2,000 tracked files) is only a fallback for when the generated art fails to load (`renderer.go:520-535`, `hud.go:245`). The game never draws it otherwise, and releases don't ship it. Remove it |
| 14 | Rules | Add two lasting rules. (a) Always remove unused assets and resources in the same change that makes them unused. (b) Grass and plants never have a contour |

---

## Phase 0: lead contract (one commit, before any agent)

0.1. **Agent cap.** Change "≤ 5" to "≤ 15" in `.cursor/PARALLEL_WORK.md:56` and `.cursor/skills/parallel-work/SKILL.md:8`. Save a feedback memory: rimworld-go allows up to 15 parallel agents.

0.2. **XS size (TDD).**
- Add `TestXSBetweenOldAndS` (`internal/domain/animal/animal_test.go`): every XS animal is longer than 0.35, and the largest XS (with trim) is shorter than the smallest S (with trim). Run it red.
- Then set `sizeLength[SizeXS] = 0.44` (`animal.go:648`).
  - The rat, chicken and the other plain XS animals go from 0.35 to 0.44.
  - The hive beetle (XS+) goes from 0.40 to 0.506, still below cat, hare, snowhare and goose (S−, 0.51).
- Green. `TestOverlayAboveTallAnimals` (`ebiten/animal_species_test.go:142`): rat 0.35 → 0.44, same ordering checks.
- The size reaches only the read model (`BodyLength` → `speciesView.size`), so no golden moves.

0.3. **Combat predicate (TDD).** `func (g *Game) inCombat(id) bool` in a new `game/combat_state.go`, tested in `combat_state_test.go`. It is true when the colonist:
- is drafted, or
- has a `job.Attack` job, or
- has a live `g.calm` entry (fleeing or calming down).

0.4. **Plant species enum.**
- Append to `plant.Species` before `numSpecies`, in this order: `Bramble`, `Moss`, `Agave`, `BallCactus`, `ElephantEar`, `FireLily`, `Dandelion`.
- Add their `String()` keys: `bramble`, `moss`, `agave`, `ball_cactus`, `elephant_ear`, `fire_lily`, `dandelion`.
- Def rows are zero placeholders that VEG-SIM fills in.
- New `Def` fields, additive:
  - `WalkCost int`: percent step time on that cell; 0 means none.
  - `Ground bool`: low ground plant, never blocks building.

0.5. **Read-model fields** in `game.go`:
- `WorldView.GrassGrowth []uint8`: 0..100 per tile, shared copy-on-write like `Grass`. A full tile reads 100.
- `WorldView.GrassVer uint64`: bumped on any grass or growth change. Presentation only, never saved.

0.6. **Art key contracts**, written into the ART spec:
- **Tree parts:** every `plant/tree_<sp>_<vv>[_season]` also exists as `…_trunk` and `…_canopy`, with the same canvas and origin, each with `_n`. The full sprite stays for the menu diorama, icons and fallback.
- **Ground vegetation:** `plant/<key>_<stage>_v<n>` with stage in `young` or `mature`, plus `plant/<key>_dormant_v<n>` for the leafless or dry look. Berry bushes also keep `_harvestable`.
- **Grass tufts:** `plant/grass_short_v0..5` and `plant/grass_tall_v0..3`. The old `grass_00..07` and `grass_tall_00..03` stay as aliases until the renderer switches.
- **Flat normals:** metadata `flat_normal: true` makes `cmd/normalgen` write a flat normal map (0,0,1) instead of the dome relief. The validator still finds a `_n.png`.
- **Tree builders move:** the lead moves the tree builders out of `families/plant.py` into a new `families/trees.py` (no output change, verified by a byte-identical render of three tree keys). `plant.py` imports it, so the two art agents never edit the same file.

0.7. **Vegetation art direction.** Rewrite ART spec §9 (plants) in our own words from your brief:
- **Identity:** small, isolated, flat-shaded sprites composed many per tile. The terrain shows through. Dense at landscape scale, sparse in a single cell.
- **Silhouette per type:** fine tapered blades; rounded mound; star rosette; woody tangle; flat cushion; thick-leaf rosette; ribbed ball; arrow-leaf fan; strap-leaf fan with orange flowers; tiny yellow dots.
- **Palette tokens:** a new `assets/palettes/materials_plants.yaml` holding:
  - dark olive #414E32
  - forest olive #556442
  - grass green #708052
  - sage #87936C
  - dry green #91915F
  - khaki #A49B70
  - stem brown #766247
  - dried foliage #857651
  - accent tokens for berries (blue-violet), fire lily (orange) and dandelion (pale yellow)
- **Lighting:** two-band flat shading, no strong highlights.
- **No contour, ever.** Grass and every plant (ground plants, bushes, crops, trees) render with `outline_width=0`. Shapes separate by colour value alone, using darker olive recesses and lighter tips.
- **Composition rules:** variants, mirroring, growth scale, no autotiling for vegetation.
- **Dormant and dry looks:** described per type.
- **Acceptance table:** blades readable at medium zoom; bush ≠ rosette; terrain visible; palette consistent; scale order grass < bush < tree; no repeat pattern; alpha only.
- **Tree brief:** trunk, canopy, overhang and see-through rules (§2.8–2.9 below).
- The new screens-spec item, the vanilla checklist rows and the UI/UX items for vegetation are added as `[ ]`.

0.7b. **Remove the vendor pack (lead, before agents start, so no agent trips over it).**
- **Delete:**
  - `git rm -r "assets/Free 2D Animated Vector Game Character Sprites"`;
  - `ebiten/pack.go` and `pack_test.go`;
  - the vendor `atlas` type and its loaders in `ebiten/sprites.go` (`loadAtlas`, `colonist`, `groundAt`, `rockAt`, `crosshair`). Shared helpers in that file stay (`tileHash`, `animFrame`, …);
  - `Renderer.LoadPack` / `LoadDefaultPack` and `r.pack`;
  - `App.pack`, `packWarn` and `loadPack`'s pack half (the art loading stays);
  - `AppConfig.LoadPack`, plus its stubs in `app_test.go:84` and `biome_ui_test.go:26`.
- **No-art fallback:**
  - `drawTerrain` draws plain terrain colours;
  - `drawPawn` draws its existing coloured rectangle;
  - `drawSelection` draws its existing stroke;
  - the HUD portrait shows nothing without art.
- **Test (TDD):** `TestRendererWithoutArtDrawsFallback` draws a frame with no art catalog and no panic, with pawns as rectangles.
- **Docs:**
  - delete the vendor section in `assets/AGENTS.md` and the tree line at :107;
  - ART spec :1038: drop "vendor pack kept as fallback";
  - add an addendum to `ART-001-vendor-pack-renderer.md` ("removed in PLAY-010; the folder is gone from the tree, still in git history");
  - remove the CC0 credit from the credits screen if one is there.
- **Check:** `grep -ri "free 2d\|vendor_char\|LocatePack"` returns nothing.
- **Note:** the 203 MB stays in git history. Shrinking the repository would need a history rewrite, which is out of scope unless you ask.
- **Commit:** `chore: remove the unused vendor sprite pack`.

0.7c. **New lasting rules (decision 14).**
- **`AGENTS.md`, non-negotiable list:**
  - "Remove unused assets and resources in the same change that makes them unused: sprites, sounds, fonts, metadata, palette entries, vendor packs, generated files and their loaders. Never keep a fallback asset nobody draws. Aliases are allowed only while something still reads them; delete them in the change that removes the last reader."
  - "Grass and plants (ground plants, bushes, crops, trees) never have a contour or outline."
- **Mirrors:**
  - the ART spec §9 (vegetation) and its asset-pipeline housekeeping section;
  - skill `rimworld-go-art-assets` (checklist line);
  - `.cursor/rules/` art-direction rule.
- **Memory:** a feedback memory for each rule.
- **Tooling (TDD):**
  - A new `cmd/assetcheck` mode (or a `validate_assets.py --orphans` flag) lists generated sprite keys, sounds and metadata files that no Go key table, palette, metadata or recipe references.
  - `make art-check` fails on orphans, except an explicit allowlist file `assets/metadata/orphan_allow.txt` (each line with a reason).
  - A unit test with a fake tree: one used key, one orphan, one allowlisted.
  - A no-contour check fails if a `plant/*` metadata or render call sets an outline (plant family: `outline_width` must be 0).
- **This batch's cleanup:**
  - After GRASS-DRAW and VEG-KEYS switch over, delete the superseded grass sprites (`grass_00..07`, `grass_tall_00..03` and their `_n` and metadata), every tree, bush or legacy alias no reader uses (`tree_00..02` once the `overlay.py` icon points at a new key), and the vendor pack (0.7b).
  - The orphan check must end at zero, or every remaining entry must be allowlisted with a reason.

0.8. **Shared facts for every agent:**
- `PawnView.Fleeing` exists; today only raiders set it (`raids.go:535`).
- Event log kinds are append-only (`eventlog.go`).
- `animal.State`, `construction.Frame` and the grass save fields are gob-encoded: add fields only.
- Concurrent `make art` runs share Blender's temp file, so every art agent uses its own `TMPDIR` under the scratchpad.
- Only the lead runs `make normals`, `make atlas` and `make art-check` across everything. An art agent may run `normalgen` and the validator on its own outputs if the tools accept paths, and reports if they don't.

---

## Phase 1: agents (12 at once, at most 15; disjoint files, shared files split by function)

| Agent | Model (why) | Owns | Must not touch |
|---|---|---|---|
| **ART-XS** | haiku (scripted Blender run, no design) | `families/animal_roster.py` (`SIZE_LENGTH["XS"]=0.44` + docstring); regenerated XS animal sprites | Go, `plant.py`, `trees.py` |
| **ART-TREES** | sonnet (Blender modelling against a brief) | `families/trees.py`, `scenes/menu.py` tree calls if their signature changes, tree sprites and metadata | `plant.py` ground plants, Go |
| **ART-GROUND** | sonnet (Blender modelling against a brief) | `families/plant.py` (grass, bushes, berry, shrub, reeds, cactus, healroot, new plants), `materials_plants.yaml` tokens use, `cmd/normalgen` flat-normal flag + test, `overlay.py` plant icons | `trees.py`, Go outside `cmd/normalgen` |
| **DOOR** | sonnet (clear spec, existing patterns) | `placement.go` (`canPlace`, `placeFrameStuff`, mark callers), `tick.go` **`constructTry`, `cancelRect`**, `orders.go` (`forcedJob`, `OrdersAt`), `frame_view.go`, `cancelFrame`, `domain/building/construction` (`Frame` fields), `ebiten/architect.go` ghost hint + interface method + stub, door tests | combat, animals, movement |
| **ANIMAL** | opus (AI behaviour, hysteresis) | `combat.go` **`hurt`**, `animal_needs.go` **`animalHurt`, flee helpers**, `animals.go` (`thinkAnimal`, `fleeCheck`, `fightCheck`, new `chargeCheck`), `handling.go` (`guardCheck`, `provoked`), `action_view.go` charge line, `domain/animal` (`State` fields, constants; not `sizeLength`), `eventlog.go` (append kinds), animal tests | `defend`/`threatTo`/`nearestThreatTo`, `nearestGrass`/`eat`/`browse`, `tick.go` |
| **PAWN-FLEE** | opus (threat hysteresis, ordering, reachability) | `combat.go` **`defend`, `threatTo`, `nearestThreatTo`, reach constants**, `tick.go` **per-pawn decision loop (≈L388-405)**, `raids.go` `Fleeing` view line, flee tests | `hurt`, animal files, `constructTry` |
| **MOVE** | opus (movement core, deadlock-free) | `occupancy.go` (all), `regions.go` `crowdCost.Reach` if needed, occupancy and blocking tests | `combat.go`, `water.go` `groundExtra`, `weather.go` |
| **VEG-SIM** | opus (sim, determinism, goldens) | `domain/world/plant` (defs, grass rules), `domain/world/biome` (plant lists), `generation` placement tests, `game/grass.go`, `game/plant_growth.go` (grass growth), `animal_needs.go` **`nearestGrass`, `eat`, `browse`**, `weather.go` **`weatherSlow`** + `water.go` **`groundExtra`** (plant walk cost), `save.go` **grass section only**, `orders.go` **`plantLabel`**, `placement.go` **only the `Ground`-plant clause in `canPlace`/`completeFrame`** (coordinated with DOOR through the lead), `biome_info.go`, VEG tests | animal fight and flee, occupancy |
| **VEG-KEYS** | sonnet (renderer key tables) | `ebiten/biome_art.go`, `environment.go` **`buildPlantKeys`, `seasonKey` tables**, `draw_art.go` **`plantSprite`**, `plant_inspector.go`, `lod.go` **`plantMarkColor`**, the hover readout line for grass growth (`hud.go` cell readout function), key tests | grass drawing, tree drawing |
| **GRASS-DRAW** | opus (batched rendering at 500², LOD) | `ebiten/grass_draw.go`, a new `ebiten/grass_batch.go`, `draw_art.go` **`drawTerrainArt`, `drawTuft`, `drawTufts`, `grassKey`**, `lod.go` **grass bake**, chunk version (`chunk_view.go`), grass tests and benchmarks | trees, plant key tables |
| **TREE-DRAW** | sonnet (renderer, existing alpha path) | `draw_art.go` **plant queue + `drawPlantObj`**, a new `ebiten/tree_fade.go`, `shadow.go` tree branch, `environment.go` **`treeSpeciesAt`, tree sway helpers**, tree tests | grass, key tables |
| **ART-CHECK** *(after the art agents)* | haiku (mechanical) | contact sheets, before and after captures into the scratchpad | any source |

- **The lead owns:** `game.go` view structs, `Makefile` (an atlas line for ground vegetation if GRASS-DRAW asks for one), `cmd/game`, `.cursor/**`, README, notices and commits.
- **Every agent:**
  - follows TDD, red then green, with coverage ≥ 80% on touched packages, or reports the gap;
  - runs targeted `go test -race -cover ./<pkg> -run '<theirs>'`, plus `go vet` and `gofmt -l`;
  - never runs the full suite, never commits, and never edits another agent's files;
  - reports in 200 words or fewer: what changed, test and coverage results, blockers, and failures seen in others' files.
- **Contract changes** go through the lead, additive only, and are broadcast to every affected agent.

---

## 2. Specifications per agent

### 2.1 ART-XS
- Set `SIZE_LENGTH["XS"] = 0.44` and change the docstring to "XS 0.44".
- Render with `TMPDIR=<scratch>/tmp-xs ANIMALS=rat,cinder_rat,chinchilla,fennec_fox,guinea_pig,squirrel,yorkshire_terrier,chicken,duck,hive_beetle make art FAMILIES=animal`. That covers all facings and frames, corpses, and the chicken `_f` set. Outline width stays 2 px.
- Re-render icons only if `ui_icon/<key>` changes.
- Deliver a contact sheet: old vs new, next to the cat, hare and goose at 64 px.

### 2.2 DOOR: replace walls and fences
- **Rule.** A **Door** or **FenceGate** may be placed on a cell holding a **Wall, Fence, Door or FenceGate of a different kind**. The cell skips `ReasonOccupied` and `ReasonBlocked`; every other check stays: bounds, scenario, rock, water, research, material.
  - Same kind over same kind stays `ReasonOccupied`.
  - A wall over a door, or floors and carpets on any of these, stay refused (`TestCarpetNotOnDoor` holds).
  - A cell already holding a frame stays `ReasonPlanned`.
- **Frame.**
  - `construction.Frame` gets `Replaces entity.EntityID` and `ReplaceMarked bool`.
  - `placeFrameStuff` sets `Replaces` and marks the old building for deconstruction with `markDeconstruct`. It sets `ReplaceMarked` only when the building was not already marked.
- **Order of work.** While `Replaces` exists:
  - `constructTry`, forced `OrderBuild` and `OrdersAt` refuse the frame with the reason "The wall must come down first" (fence, door or gate as the case may be).
  - Haulers may still deliver materials.
  - The frame inspector shows "Waiting for the wall to be removed".
- **When the old piece is gone.** Deconstruction works as today: refund, `markRoomsDirty`, `structVer++`, walkable for a moment. The frame then builds normally. Rooms merge for that moment, which is expected and documented.
- **Cancel both ways.**
  - Cancelling the frame (`cancelFrame`, `cancelRect`) clears the deconstruct mark it set.
  - Clearing the wall's deconstruct mark (a cancel designation) cancels the replacing frame and refunds delivered materials as a frame cancel does today.
- **UI.** The Architect ghost stays valid on such cells and the ghost text reads "replaces wall". This comes from a small read-model method, `ReplacesAt(kind, x, y) string`, added to the renderer's consumer interface with a stub in `app_test.go`.
- **Tests:**
  - a door over a wall ends as a door, with the wall's refund on the ground and the rooms re-detected;
  - the frame is never completed while the wall stands (step through with a builder present);
  - cancelling the frame restores an unmarked wall;
  - unmarking the wall cancels the frame;
  - a gate over a fence, and a door over a gate;
  - `TestCanPlaceReasons` extended: wall on wall and door on door are Occupied, carpet on wall is Occupied, a door on a planned wall is Planned;
  - a save round-trip keeps `Replaces` and `ReplaceMarked`.

### 2.3 ANIMAL: reactions, helpless flight, predator charges
**A. Reaction when hurt.** When an attacker exists (`by != 0`), an awake or sleeping animal always ends in `Flee` or `Attack` within one decision. A **fighter** is a Territorial, Predator, PackPredator or Legendary animal.

| Animal | Attacker | Reaction |
|---|---|---|
| Wild, flee class (Grazer, Skittish, Docile, Domestic, Pet) | anyone | flee (today's 900 ticks); the manhunter roll is unchanged |
| Wild fighter, not badly hurt | anyone, including its own species | attack the attacker; keep a current target (no hopping); the leash ends the chase (territorial 12, predator rules as today) |
| Wild fighter, badly hurt | anyone | break off and flee (existing), unless enraged or Legendary |
| Tame, any class | a colonist (friendly fire) | flee, never fight |
| Tame fighter (dog, wolf, bear…) | a wild animal, a raider, anything else | fight back (`guardCheck` and `fightCheck` let this fight run) |
| Tame flee class | anything except a colonist | flee |
| Hive | anyone | already enraged; unchanged |

- **Slaughter is not an attack.** Check the slaughter path. If it goes through `hurt`, slaughter hits are exempt from fleeing.
- **Traps and bursts** (`by == 0`): unchanged.

**B. Helpless animals flee on every hit (decision 4).**
- An animal **can't fight back** when any of these hold:
  - it is a flee class;
  - it is tame and hit by a colonist;
  - it is badly hurt;
  - the attacker is unreachable: a different region label for its movement layer, e.g. outside the pen fence or across water;
  - the attacker is ranged and out of melee reach with no path.
- **Each hit** sets `Flee`, refreshes `FleeUntil` and **re-picks a flee goal immediately**, even mid-flight. The goal is chosen in this order:
  1. today's `FleeGoal` 8 tiles away, if `walkableNear` finds it and it is reachable on the animal's layer (tame: fences block, gates pass; wild: both block);
  2. otherwise a bounded BFS (radius 8, at most 300 cells) over the animal's reachable cells, taking the cell that maximises distance from the attacker. In a pen this is the far side of the pen;
  3. otherwise any reachable neighbour cell not adjacent to the attacker;
  4. otherwise any reachable neighbour, so the animal moves on every hit.
- **No flap.** Between hits the existing flee window rules apply. A hit while already fleeing to a goal at least as far stays on that goal; otherwise it switches. The test hits a penned sheep every 60 ticks for 20 hits and expects it to move after every hit and never stand still twice in a row while hit.

**C. Predator charge (decision 5).**
- **Who:** a wild, awake (`!sleeping(a)`) Predator or PackPredator that is not fleeing, not badly hurt, not enraged, not tame, has no target, and only when `predatorsHuntColonists()` is true. That makes it off on Wanderer and Gentle.
- **Check:** every 60 ticks, staggered by id: `nearestWhere(pos, 3, ckPawn, standing colonist or raider)`.
- **Roll:** on a hit, **30%** with the sim RNG. After any roll, and after a charge ends, the animal waits for a new saved field `NextCharge = now + 600`.
- **Charge:** `Mode=Attack`, `Target=pawn`, a new saved field `ChargeSince=now`. It ends when the target is more than 10 tiles away, after 1,200 ticks, or when the target is down; then the cooldown starts.
- **Log:** the pawn gets "Charged by a <species>", the animal gets "Charged <name>" (two kinds appended).
- **Inspector:** the animal's action line reads "Charging <name>" (`action_view.go`).

**Tests:**
- a reaction table over all 10 behaviours × {XS, L} × {wild, tame} × {colonist, wild predator, raider}: never ignored;
- the no-flap test for a territorial animal over 3× the window;
- the penned-sheep test (B);
- the charge happens within the expected rolls for a fixed seed;
- no charge while asleep, on Gentle, or from grazers;
- the cooldown holds;
- a charge breaks off beyond 10 tiles;
- the existing tests stay green: `TestIndexMatchesBruteForce`, `TestHurtPredatorBreaksOff`, `TestTameWolfGuardsAgainstRaiders`, `TestProvokedAnimalsSometimesGoManhunter`.

### 2.4 PAWN-FLEE (decision 6)
- **Who flees.** An undrafted colonist that is **unarmed**: no `g.equip` entry, or `!canViolence`.
- **What is a threat.** Any animal with `Mode==Attack && Target==id`: a hunt, charge, manhunter or revenge.
  - It counts within **12 tiles**, or **16** while calm or fleeing.
  - The lookup uses `nearestWhere`.
  - Armed colonists keep today's 4/8 reach and fight back. Raiders keep 14/18.
- **Priority.** The unarmed-flee check moves **before** the queued-order step in the per-pawn loop.
  - Queued orders stay queued and resume after the calm window.
  - An interrupted forced job goes back on the queue (`forcedJobEnded`).
  - A sleeping colonist wakes and flees (today's behaviour).
  - Mental breaks still come first; drafted colonists are unchanged.
- **Flee goal.** Candidates, in order:
  1. `FleeGoal` at 8 tiles straight away from the threat;
  2. ±45° from that;
  3. ±90° from that.
  - Each candidate is checked with `walkableNear(·, 3)` and `reachable` (region labels).
  - When two pass, prefer one inside the home area.
  - If none passes, the colonist is cornered and stays put.
  - The 30-tick re-pick and `fleeCalm=600` stay.
- **UI.**
  - `PawnView.Fleeing = true` while fleeing or calm with a threat in release range.
  - `PawnView.Job` reads "Fleeing from <creature>".
  - The lead checks that the colonist bar and inspector show it, and adds it if not.
- **Tests:**
  - an unarmed colonist flees from a charging animal 10 tiles away;
  - it also flees from a manhunter 11 tiles away;
  - an armed one fights instead, and a drafted one does nothing;
  - a queued order waits, then resumes;
  - a sleeper wakes and runs;
  - a goal across a wall is never chosen when a reachable one exists;
  - a no-flap test at the 12/16 boundary (threat oscillating at 13-15 tiles keeps one state);
  - `TestUnarmedColonistCalmsDown` and `TestArmedColonistKeepsTarget` stay green.

### 2.5 MOVE: walk-through (decision 7)
- **Partners.** A colonist and a tame animal are pass-through partners; that covers colonist–colonist, colonist–tame and tame–tame.
  - A step into a cell held only by partners is `stepGo`: no waiting, sidestep, yield, `makeWay`, repath or arrive-near.
  - Wild animals and raiders keep today's rules, both as movers and as occupants, including the hostile "stop next to your target" rule.
- **Combat exception.**
  - A colonist mover that is `inCombat`, whose **final** step lands on a cell where another standing colonist stands, uses today's arrive-near logic. The same applies when that occupant is `inCombat`.
  - Exact-cell jobs (`arriveExact`) wait as today.
  - The unstack sweep moves the higher ID off a shared cell **only** when one of the two is `inCombat`. Outside combat, shared cells are fine.
- **Index.**
  - The occupancy index keeps one blocker per cell for blocking pairs.
  - Partners are recorded in a separate partner layer, a per-cell count plus the first id, so two partners on one cell are not a "conflict".
  - `occupancy.near` semantics for blockers stay; `TestOccupancyNearMatchesCells` holds for blockers.
- **Path costs.** `crowdCost` doesn't price partners for a partner mover (no `occupiedCost`, no `besideCost`). It still prices wild animals and raiders.
- **Tests:**
  - **Update deliberately** the tests that assert colonist–colonist blocking. Their old behaviour is the thing being changed, so the new assertion is the new rule, not a weaker one: `TestEnterBlockedByStandingPawn`, `TestIdlePawnStepsAside`, `TestHeadOnSwapResolves`, `TestCorridorHeadOnBothArrive`, `TestDoorChokepointQueues`, `TestThreeCycleNoDeadlock`, `TestCrowdToOneCellSpreads`, `TestNoSharedCellInvariant`. The last one becomes "no blocker pair and no two in-combat colonists share a cell".
  - **New tests:**
    - two colonists cross head-on in a 1-wide corridor in the time an empty corridor takes (±1 tick);
    - a colonist walks through a working colonist without waiting;
    - three drafted colonists ordered to one cell end on three cells;
    - a fleeing colonist doesn't stop on another colonist;
    - a dog and a colonist pass through each other;
    - a wild boar and a raider still block;
    - `TestBlockingSameSeedSameRun` holds.
- **Benchmarks:** `BenchmarkColonyHour` and `TickGrown/500` within +10%.

### 2.6 VEG-SIM: new wild plants, bramble cost, grass growth
**New species defs.** Our numbers, filled into the Phase 0 enum. All are wild, not crops, not sowable, `Ground: true`, with no wood and no yield.

| Key | GrowTicks | CutWork | Browse | WalkCost | ColdKill / HeatKill (tenths °C) | Notes |
|---|---|---|---|---|---|---|
| `bramble` | 40000 | 30 | yes | **150** | -300 / 450 | woody tangle; Trunk false |
| `moss` | 30000 | 10 | yes | 0 | -500 / 350 | flat cushion |
| `agave` | 50000 | 25 | no | 0 | -50 / 650 | desert rosette |
| `ball_cactus` | 60000 | 20 | no | 0 | -80 / 700 | ribbed ball |
| `elephant_ear` | 30000 | 15 | yes | 110 | 50 / 500 | tropical broadleaf |
| `fire_lily` | 30000 | 10 | yes | 0 | 0 / 450 | strap leaves, orange flowers |
| `dandelion` | 15000 | 5 | yes | 0 | -200 / 400 | tiny yellow flowers |

- `TestNativePlantsSurviveTheirBiome` must hold for every placement below; VEG-SIM adjusts the kill limits if a biome's climate needs it.

**Biome placement** (permille added to `biome.go` `Plants`):

| Biome | Plants added |
|---|---|
| Temperate forest | bramble 12, dandelion 15 (landing-site path only; the legacy `Generate` stays untouched) |
| Temperate swamp | bramble 15, moss 20 |
| Boreal | moss 30, bramble 8 |
| Tundra | moss 25 |
| Cold bog | moss 35 |
| Tropical rainforest | elephant_ear 25, fire_lily 10, bramble 10 (total 155‰, inside `TestBiomePlants` 140-180) |
| Tropical swamp | elephant_ear 20, moss 10 |
| Arid shrubland | agave 15, ball_cactus 10 |
| Desert | agave 10, ball_cactus 12 |
| Extreme desert | ball_cactus 5 |

**Rules for the new plants:**
- **Building:** `Ground` plants never block building. Placing a building on one clears it, as bushes and crops are cleared today in `completeFrame`.
- **Walk cost:**
  - `weatherSlow` multiplies step time by `WalkCost` on unfloored cells, using `plantAt` (O(1)). `groundExtra` adds `(WalkCost-100)/10` to path cost.
  - If the first-plan path query (`g.pathQuery`) can take a per-cell cost without a global rebuild, include it so first routes avoid brambles. Otherwise brambles affect step time and replans only, and the handoff records why.
- **Labels and info:** `plantLabel` names ("bramble thicket", "ground moss", "desert rosette", "ball cactus", "elephant ear", "fire lily", "dandelions"). The biome picker lists them.

**Grass growth (decision 8):**
- **State.** `growth []uint8` is allocated lazily, sized like `cells`.
  - A tile with grass and no growth entry is full (100), so the zero state equals today's state and saves stay small.
  - A sparse `growing` set, kept in a per-chunk list, tracks tiles under 100.
  - Saved as an additive field `GrassGrowth` that stays empty while nothing grows. Old saves load full.
- **Regrow.** When the regrow heap pops, the tile gets grass at **growth 15** (shoots).
- **Fill out.**
  - In `growChunks`' staggered per-chunk pass, every 250 ticks per chunk: `growth += rate × TempFactor`. The rate is chosen so 15 → 100 takes **3 days** in mild weather.
  - Only cells in that chunk's `growing` list are touched; reaching 100 removes the cell.
  - Snow or freezing pauses growth.
- **Grazing.**
  - `nearestGrass` collects candidates in the first ring with any grass, plus the next ring, and picks the highest growth. Ties go to today's ring order, so full fields pick exactly as today.
  - `eat` scales `GrazeFood` by growth/100, with a minimum of 30%.
  - Tiles under 30 growth are not grazed unless nothing else is within 30 tiles. This hysteresis keeps a grazer from bouncing between the same tiles.
- **Read model.** `WorldView.GrassGrowth` is shared copy-on-write. `GrassVer` bumps on seed, clear, regrow and every growth pass that changes a value.
- **Hover readout** (VEG-KEYS draws it): "Grass (growing, 40%)".

**Tests:**
- defs complete for the new species;
- every placement survives its biome;
- `TestBiomePlants` ranges hold;
- bramble walk time is 1.5× and a replan detours around a bramble patch;
- browse eats the new browse plants and never the cactus types;
- grass regrows at 15 and reaches 100 in 3 days ±10% at 15 °C;
- growth pauses under snow;
- grazers prefer the fuller of two tiles at equal distance;
- no-flap: a grazer next to a regrowing tile doesn't graze it at 20;
- a save round-trip with tiles growing;
- old saves load full;
- `TestTemperateGenerationUnchanged` and `TestZeroSiteMatchesLegacy` hold.

**Goldens:**
- `goldenLegacy` entries for non-temperate biomes **will move**. Re-pin with a field diff showing only plants changed and the reason.
- `TestScaleGolden` and `TestGrownGolden` should hold, because seeded grass is full and growth is empty. If they move, use the same diff method.

### 2.7 ART-GROUND: restyle every ground plant, plus the new ones
- **Look.**
  - Flat two-band materials from `materials_plants.yaml`.
  - **No contour on any plant sprite** (`outline_width=0`, including `_grass_geo`/`_grass_tall_geo`, which pass `outline=True` today). Crops are re-rendered with the outline off too; their shapes are otherwise unchanged.
  - `flat_normal: true` for grass, moss, dandelion and other low cover. Bushes keep a soft relief.
  - Every sprite has transparent gaps.
- **Grass.** `grass_short_v0..5`: 4–10 tapered blades each, leaning differently, some lighter blades for overlap, about 0.35 tile wide. `grass_tall_v0..3`: the same blade language, about 1.6× taller and fuller.
  - Dormant: `grass_short_dormant_v0..2` (straw-brown, shrivelled).
  - No flowers on grass. Wildflowers are now their own plant.
- **Existing plants, redrawn to the brief:**
  - **bush:** a low scalloped mound in dark olive with mottled clusters, four variants: two compact, one asymmetric, one open; plus dormant.
  - **berry bush:** the same mound with small blue-violet berry dots; young, mature and harvestable, plus dormant.
  - **shrub:** a low star rosette of 6–12 broad pointed leaves, four variants; the `_dry` look stays.
  - **reeds:** thin upright stems with brown heads.
  - **cactus:** keep an upright cactus distinct from the new ball cactus.
  - **healroot:** a low herb with medium-width leaves around a centre.
  - Crops are untouched.
- **New plants**, each with young, mature and dormant stages and 3–4 variants:
  - **bramble:** a horizontal tangle of brown woody stems with scattered olive leaves; dormant shows the stems only.
  - **moss:** a flat mottled grey-green cushion.
  - **agave:** 8–14 thick blue-green pointed leaves in a rosette.
  - **ball_cactus:** a squat ribbed sphere with faint ribs and no long spines.
  - **elephant_ear:** 6–10 large arrow-shaped leaves radiating outward, with gaps between them.
  - **fire_lily:** a strap-leaf fan with 2–3 small orange flower clusters.
  - **dandelion:** tiny green rosettes with 1–2 pale yellow dots (a small canvas).
- **Icons.** `overlay.py` plant icons are re-cut where they come from changed sprites (for example the `tree` icon comes from ART-TREES output; the lead runs `FAMILIES="plant overlay"` last).
- **Delivery.** A contact sheet per plant type at 1×, 2× and 64 px, and a composed 8×8 tile field mock-up (agent-made, scratchpad only) for the lead's review against the acceptance table in ART §9.

### 2.8 ART-TREES: trunk and canopy
- **Trunk.** A visible trunk from the ground to at least 55% of the tree's height, with a slight flare at the root and bark from `bark` / `bark_birch`.
- **Canopy.** Loose leaf clusters with gaps and **no solid core blob**.
  - It sits on the upper trunk, overhangs **about 0.5 tile into the tile above**, and spills no more than 0.2 tile to each side.
  - Total height is about 1.5 tiles above the base.
- **Blending with neighbours.** Feathered, irregular outer clusters so neighbouring canopies overlap into one forest edge. Variants differ in canopy height (±0.1) and lean.
- **Pine:** a visible trunk below tiered boughs that start at about 35% of the height.
- **Jungle:** a tall trunk with a broad, flatter crown.
- **Seasons:**
  - oak and birch keep autumn and bare;
  - bare shows the full branch structure;
  - pine keeps its snow look.
- **No contour** on trees or their parts (`outline_width=0`).
- **Parts.** Each full sprite plus `_trunk` (trunk and lowest limbs) and `_canopy` (leaves and upper limbs), with the **same canvas and origin**. Blob shadow metadata scales with the canopy.
- **Menu.** `scenes/menu.py` diorama trees use the new builders, so the lead re-renders the menu art.
- **Delivery.** Contact sheets, plus a 3×3 grove at 1× and 2×.

### 2.9 TREE-DRAW: parts and see-through canopy
- **Parts.** Draw the trunk part, then the canopy part, with one pose. The canopy keeps `canopyLag` sway. Fall back to the full sprite when the parts are missing.
- **See-through canopy.**
  - The target canopy alpha is **0.4** when any creature (colonist, raider or animal) stands on the tree tile, the tile above, or the two diagonal tiles above. The lookup is `l.creatures`, O(1) per visible tree.
  - The same applies when the hovered or selected entity is there, and when the Architect ghost overlaps those tiles. Otherwise the alpha is 1.
  - The alpha eases toward the target in about 0.2 s on the presentation clock, with a per-tree spring map that drops trees leaving view. Reduced Motion snaps it.
  - The trunk never fades. Shadows don't fade.
- **Order.** Trees keep y-sorting. A canopy over a creature on the tile above is now see-through; creatures on the tree tile draw over the trunk.
- **Tests:**
  - `TestGeneratedArtCoversRenderer` covers the tree keys and parts;
  - the canopy fades with a pawn above and stays opaque with no one near;
  - Reduced Motion snaps;
  - the trunk stays opaque;
  - the fallback works without parts;
  - a draw-order trace;
  - `BenchmarkDrawPlants` stays at or under 1.2 ms.

### 2.10 VEG-KEYS: key tables, inspector, LOD colours
- **Keys.** Map every new species and stage to the Phase 0 keys:
  - stage from growth: young under 55%, mature otherwise;
  - dormant when out of season, wilted or frozen;
  - variant from `tileHash`;
  - fallbacks to an existing bush or shrub key when the art is missing;
  - restyled bush, berry, shrub, reeds, cactus and healroot keys use the new stage and dormant keys.
- **Inspector.** `plantTitle` and `plantCategory` ("Wild plant"). A line "Slows walking" for bramble and elephant ear. A grass tile hover line shows growth.
- **Far zoom.** `plantMarkColor` gets one colour per new species.
- **Tests:**
  - `TestNewBiomeKeysFallBackWithoutArt` extended to the new species;
  - a stage and dormant key test;
  - inspector text;
  - the grass hover line.

### 2.11 GRASS-DRAW: composed tufts that blend across tiles
- **Composition.** Each grassy tile is drawn as **N tuft copies**, never a carpet. A tile is grassy if `Grass[i]==1` on any terrain the sim seeds: soil, rich soil, marsh, gravel, sand or mud. `TestGrassMatchesRendererRule` is updated to the sim's rule.

| Growth | Copies |
|---|---|
| < 25 | 1 |
| < 50 | 2 |
| < 75 | 4 |
| < 90 | 6 |
| ≥ 90 | 9 |

  - Copies sit in a jittered 3×3 grid: cell centre ± 0.18 tile.
  - Copies on the edge row and column may sit up to **0.15 tile past the edge**, so neighbouring tiles interleave and the field reads continuous while soil still shows between tufts.
  - Each copy picks a hashed variant, a horizontal mirror (allowed because grass has flat normals) and a scale: short 0.45–0.65, or tall 0.7–0.9 on rich soil or where a tile hash marks a tall patch. Tall patches use smooth `valueNoise` so they cluster instead of checkerboarding.
- **Batching.**
  - All tuft copies for the visible area are drawn in **one triangle batch per frame** from the ground-vegetation atlas page. The lead adds `make atlas` support for the `plant/grass_*` family if needed.
  - Vertex colour = ambient light × `grassTint` (biome and season) × snow fade.
  - Sway is per vertex: top vertices offset by the existing wind field, within the 0.35 rad lean limit.
  - No per-copy `DrawImage`.
- **Pass order.** Ground tiles and edges for the whole view first, then the tuft batch, then objects. A neighbour's ground can never paint over a tuft.
- **LOD:**

| On-screen tile size | Grass drawn |
|---|---|
| ≥ 48 px | full copy counts, live sway |
| 24–48 px | copies capped at 4 |
| 16–24 px | tufts baked into the chunk image (no sway); chunk version includes `GrassVer` |
| < 16 px | a soft grass tint baked into the map image |

- **Cut marks and snow:** unchanged. Grass under buildings, floors, frames and plants stays hidden (`grassFrameMask`).
- **Budgets:**
  - `BenchmarkDrawArt500` and `BenchmarkDrawArt250`: at most 6 ms and at most 6,000 draw calls at every zoom.
  - A new `BenchmarkGrassBatch`: at most 1.0 ms for a full screen of grass at z1.
  - `-fpsprobe` at 500², z0.25, 1% low at least 60 fps (the lead runs it).
- **Tests:**
  - copy count follows growth;
  - copies spill past the edge and never beyond 0.15;
  - the batch is one draw;
  - no grass under floors;
  - the chunk version changes with grass growth;
  - mirroring is deterministic per tile;
  - the snow fade applies.

---

## Phase 2: lead verification, docs and commits

1. **Order of landing.** Agents land in any order, but the lead merges and verifies in this order:
   1. DOOR, ANIMAL, PAWN-FLEE, MOVE;
   2. VEG-SIM, VEG-KEYS;
   3. the art agents;
   4. GRASS-DRAW, TREE-DRAW.
2. **After each group:**
   - `go build ./...`, `gofmt -l`, `go vet ./...`, `staticcheck ./...`;
   - `go test -race -count=1 -timeout 40m ./...`;
   - determinism ×3 (`-run 'Golden|Determin|SameSeed|Scale'`).
3. **Art:** `make normals`, `make atlas`, `make art-check` (0 errors), and the menu re-render.
4. **Goldens:** re-pin only with field evidence (VEG-SIM `goldenLegacy`; any animal or movement drift), recorded in the handoffs.
5. **Benchmarks:**
   - `BenchmarkColonyHour` within +10% of 13.8 ms;
   - `TickGrown/500` within +10% of 0.52 ms;
   - the draw benchmarks above;
   - a 2-minute `-fpsprobe` at 500² z0.25 and z1.
6. **Captures by eye:**
   - zoo (XS vs S);
   - grass field (`-capture-ui grass`) before and after;
   - a forest edge with a colonist under a canopy;
   - each biome's new plants;
   - door over wall (ghost, wall down, door up);
   - a wolf charge and an unarmed colonist fleeing;
   - a penned sheep being hit;
   - two colonists crossing in a corridor;
   - the menu diorama.
7. **Handoffs:**
   - ART-015 XS size
   - UI-021 door over wall
   - SIM-035 animal reactions, helpless flight and charges
   - SIM-036 colonist flee
   - SIM-037 walk-through (with a pointer added to SIM-014)
   - SIM-038 new wild plants and grass growth
   - ART-016 vegetation restyle
   - ART-017 trees
   - ENG-002 grass batch and LOD numbers
8. **Checklists and specs:**
   - re-grep the vanilla checklist (animals, wildlife, plants, construction replace, self-defence, movement);
   - the UI/UX checklist (Architect replace, inspector);
   - ART spec §9 and §11;
   - the screens spec (inspector lines);
   - mark each `[x]`, or `[-]` with what's missing.
9. **README:** feature lines for animals, construction, movement, vegetation and trees; refresh the grass and forest media.
10. **Commits:** on `main`, one per workstream (12 or so), never pushed.

## Critical files
- **Sim:** `internal/application/game/{combat,combat_state,occupancy,animals,animal_needs,handling,placement,tick,orders,frame_view,raids,eventlog,grass,plant_growth,weather,water,save}.go`, `internal/domain/{animal,world/plant,world/biome,building/construction}`.
- **Renderer:** `internal/adapters/inbound/ebiten/{draw_art,grass_draw,grass_batch,tree_fade,lod,shadow,environment,biome_art,plant_inspector,architect,hud}.go`.
- **Art:** `assets/source/blender/artlib/families/{animal_roster,plant,trees,overlay}.py`, `assets/palettes/materials_plants.yaml`, `cmd/normalgen`.
- **Docs:** `.cursor/{PARALLEL_WORK.md,skills/parallel-work/SKILL.md,ART_DIRECTION_AND_ASSET_PIPELINE.md}`.

## Verification
- Every test named above is green with `-race`; the full suite passes with `-timeout 40m`.
- Determinism ×3 passes, with goldens re-pinned only with evidence.
- `make art-check` reports 0 errors.
- The draw, tick and colony benchmarks are within budget, and the fps probe 1% low is at least 60 at 500².
- The captures above are reviewed, and a manual play checks each of the fourteen requests.
