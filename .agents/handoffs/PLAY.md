# PLAY handoffs

Families: PLAY (handoffs and the PLAY010–PLAY024 plans). Each section condenses one task's handoff; the full original of `<stem>` is `git show 76d605d04:.agents/handoffs/<stem>.md` (a plan: `git show 76d605d04:.agents/handoffs/_plans/<stem>.md`). New sections go at the end, in the format of `.agents/templates/HANDOFF.md`.

## PLAY-001-path-jobs-names
Pawn names, A* pathing and the job FSM · 2026-09-16 · 54a2df3ca
- Changed: pawns have display names; deterministic 8-connected A* on `grid.Grid`; one job FSM (Goto/Do/Complete/Fail) with entity+tile reservations. Tick phases: commands, needs, AI/jobs, movement, clock. `command.MovePawn` is the first typed gameplay command (right-click on a selected pawn). `PawnView.Name/Job`, `Game.Enqueue/Apply`. Inspector and colonist bar show name and job line.
- Decisions: one sim owner, insertion-order pawns, no goroutine-per-pawn. A* is Chebyshev, diagonal only if both orthogonal neighbours are walkable, tie-break f, g, y, x. Random `stepOnce` walks removed: idle pawns stand until AI assigns work.
- Limits: no region graph, congestion or pawn-pawn occupancy.
- Determinism: pathing and job assignment are seed- and insertion-order deterministic. Save: none.

## PLAY-002-eat-sleep-needs
Eat, sleep and needs · 2026-09-16 · 54a2df3ca
- Changed: ground food and wood stacks, hunger/rest decay on `schedule.DefaultColony`, Eat job, Sleep on the current tile until beds exist (PLAY-003). Critical hunger/rest interrupt Work. Start kit Ada, Bo, Cora plus food. `PawnView.{Hunger,Rest,Ate,SleptInBed}` (percent 0–100), `WorldView` stacks and Food/Wood totals, `Game.SpawnStack/SetNeeds/LoadStartKit`. Inspector food/rest bars, top chrome Food count.
- Decisions: needs are 0–10000 permille. Hunger decays every 12 ticks awake, rest every 4 ticks awake, sleep restores 3/tick. Eat reserves the food entity; no negative quantity. Candidate food is insertion order with distance tie-break; decay is tick-modulo.
- Limits: no cooking, tables, diet or mood (UI/UX §9.1 stays `[-]`). Downed-at-0 is SURV-001.

## PLAY-003-stockpile-haul-architect
First playable: stockpile, haul, build · 2026-09-16 · 54a2df3ca
- Changed: paint a stockpile (food+wood), haul, build wall or bed (wood, frame, Construct). Walls block pathing; beds are claimed for Sleep. Commands `DesignateStockpile/Wall/Bed/Cancel`, `SetWorkEnabled`; `WorldView` lists items, frames, buildings, stockpiles; `PawnView.WorkHaul/Construct/Grow`. `game.NewColony` loads the start kit. Architect overlay and Work enable cells; Escape order: Architect, Work, inspector, pause.
- Decisions: stockpile filter is food+wood only. Frames stay walkable, completed walls `SetWalkable(false)`. Wall wood 2 / 40 work ticks, bed 3 / 60 (original numbers). Work types are enable flags, not 1–4 priorities. Harvest is last in work order so it does not starve haul. Bed `Claimant` is the sleeper. Pinned by `TestFirstPlayableHaulBedEatSleep` (~80k ticks).
- Limits: no grow zones, rooms, roofs, power, stockpile settings UI or priorities; wizard does not change the start kit. UI-2 Architect/Work stay `[-]`.

## PLAY-004-architect-and-pawn-orders
Build menu, drafting, right-click orders · 2026-10-05 · 8c2d59745
- Changed: buildable catalog (wall, door, floor, bed, table, chair, lamp) with costs, work, layers (floor under object), blocking and rotation; one `CanPlace` rule shared by preview and sim; deconstruction with half refund; drafting; prioritized jobs; `OrdersAt` lists valid actions with reasons. Architect panel (categories, icon cards, tooltips, ghosts, line/fill/single/area placement, Q/E rotate), Draft button (R) with badges, right-click order menu, translucent blueprints, doors open when occupied.
- Contracts: commands `DesignateBuild/Deconstruct`, `DraftPawn`, `Prioritize`, `Order` (`Command` gains `Build/Facing/Order/Layer`); `PawnView.Drafted`, `FrameView/BuildingView.Facing/Floor/Deconstruct`, `game.Buildables()`, `Game.CanPlace`, `OrdersAt`; job `Deconstruct`, `Record.Dest/Forced`; `-capture-ui`.
- Decisions: validity lives in the application layer, UI only displays. One floor + one object per tile; hauls carry the exact frame id (`Dest`). Drafted pawns take no automatic jobs (eating/sleeping included). Prioritized jobs yield only to exhaustion. A finished blocking building pushes pawns off its tile to a fixed-order neighbour.
- Limits: 1x1 footprints only, no material choice, research locks, multi-select, queued orders, forbid or frame inspector; table/chair/lamp had no effect yet. Save: none (buildable defs are code).

## PLAY-005-world-resources
Map generation, plants, mining, stone · 2026-10-05 · 351dc10b6
- Changed: seeded map generation (terrain + plants), `terrain` package, plant species with growth and regrowth, Cut plants and Mine orders, stone blocks and stone as wall/floor material. Commands `DesignateCut/Mine`, `Command.Stuff` (0 = wood), `OrderCut/Mine`, `WorkMine/Cook/Hunt`; read model `WorldView.Terrain/TerrainW/Mines/Stone`, `PlantView.Species/Growth/Stage/Cut`, `FrameView/BuildingView.Stuff`, `Game.CanPlaceStuff`. Default map 48x40; `NewColony` generates the map and centres the start kit. Architect Cut/Mine tools, Wood/Stone picker.
- Decisions: generation uses its own RNG stream derived from the seed. Rock tiles are terrain; mining makes stone ground. Trees block building (cut first); bushes/crops are cleared by completed buildings.
- Limits: no water/ore, no terrain movement cost, mining speed has no skill.

## PLAY-006-food-production
Food kinds and growing zones · 2026-10-05 · 43f95b9fb
- Changed: per-kind nutrition and preference (meal > ration > amberroot = berries > meat); berry bushes yield 5 and regrow to 35%; growing zones sown with amberroot (8 per harvest, faster on rich soil; Grow work order harvest > sow > cut), auto harvest and resow. Read model `GrowZones`, `FoodByKind`, `PawnView.LastMeal`. Grow-zone overlay and per-kind item sprites.
- Decisions: four food routes: forage, farm, hunt (PLAY-009), cook (PLAY-007). Amberroot later became potatoes (PLAY-012).

## PLAY-007-benches-and-bills
Multi-tile buildings, benches, bills · 2026-10-05 · eed93725c
- Changed: rotatable multi-tile footprints with interaction spots; 1x2 bed (cot stays 1x1 until PLAY-011), workbench and stove that block movement; recipes (simple meal: 2 raw food to 1 meal; wooden club: 4 wood); bills (do X / forever / until you have X, suspend, remove); Bill job (fetch one ingredient stack, carry to work spot, work, drop product). Building inspector shows bills with -/+/mode/pause/delete and add-recipe buttons, plus Deconstruct. Work panel gains Craft.
- Contracts: construction `Size/Spot/HasSpot/Bench`, `Footprint`, `Covers`; `recipe` package; job `Bill` + `Record.BillIndex`; commands `AddBill/SetBill/RemoveBill`, `WorkCraft`; `BuildingView.W/H/HasSpot/SpotX/SpotY/Bench/Bills`, `Game.Recipes()`, `Game.CanPlaceAt(kind, stuff, facing, x, y)` (replaces ebiten `Simulation.CanPlace`).
- Limits: single-stack ingredients only, products drop on the work spot, no ingredient radius/filters, no bill reordering.

## PLAY-008-rooms-mood-lighting
Rooms, impressiveness, mood, lamp light · 2026-10-05 · 96849e2cb
- Changed: rooms are flood-filled regions bounded by rock, walls and doors (touching the map edge = outdoors), with contents, role (general/bedroom/dining/workshop/barracks), impressiveness score and label (awful..impressive). Pawns keep timed memory thoughts (bed vs ground, outside, awful/nice room, meal vs raw, table) and situational thoughts (hungry, tired); mood = 50 + thoughts, clamped 0..100; below 30 a pawn loses one work tick in four. A lamp lights its whole room (warm tint).
- Contracts: packages `world/room`, `pawn/mood`; `PawnView.Mood/MoodLabel/Thoughts`, `WorldView.RoomGrid/Rooms`, `Game.RoomInfo(x, y)`; ebiten `drawDaylightRooms` replaces the flat veil, `skyOverlay` premultiplied, max alpha 48 to 96; `-capture-ui pawn`.
- Limits: rooms recompute fully when dirty; mood has no smoothing, trend or breaks; light is per room (no falloff, no outdoor radius, no plant effect); wall art can show a lit band on a room's south wall.

## PLAY-009-combat-animals-hunting
Health, animals, weapons, combat, hunting · 2026-10-05 · 84fa85a14
- Changed:
  - Health: injury (100 HP), downed at 70, die at 100, heal over time (faster asleep in bed, slower downed); downed pawns stand at 55.
  - Two original species. Dustbuck: skittish grazer in herds, 12 meat. Ridgewolf: predator hunting dustbucks when hungry, bites colonists within 5 tiles only when hungry with no prey, retaliates, drops a downed colonist for other prey, 6 meat (a wolf kill leaves half to scavenge). Wildlife spawns away from the start; a herd migrates in at dawn when fewer than 4 remain.
  - Weapons: fists, club, rifle, hooves, bite. Instant deterministic rolls, rifle accuracy falls with distance, rock/walls/doors block shots; cooldowns plus short aim warm-up. Drafted pawns attack on right-click and fire at hostiles when idle. "Equip" order; hunters fetch a free rifle.
  - Hunt designation (architect Orders tool, animal inspector toggle, right-click), Hunt work type needs a ranged weapon; kills drop meat.
  - Presentation: animal sprites (`animal` Blender family), hunt crosshairs, health bars, held weapons, tracers, hit puffs, melee arcs, red hostile banner.
- Contracts: packages `animal`, `combat/weapon`, `combat/projectile`; `health.Record.Injury`; command `DesignateHunt`, orders `OrderHunt/Attack/Equip`, jobs `Hunt/Attack/Equip` (Attack does not reserve its target); `PawnView.Health/Weapon`, `WorldView.Animals/Traces`, `Game.SpawnAnimal/Equipped`. `NewColony` spawns wildlife, `New` does not (fixtures stay animal-free); start kit has a rifle; `-capture-ui pawn|combat|melee`.
- Decisions: six-day probes on seeds 1-3: unprovoked wolves never hurt colonists, wolves thin herds, migration refills them; marking all for hunting yields 84 meat on day one.
- Limits: no body parts, bleeding, medicine, rescue (PLAY-010); animals need no rest, no taming/manhunters/corpses (PLAY-010); undrafted colonists neither flee nor fight (PLAY-010); shots resolve instantly (tracers presentation only); one shared hostile banner (PLAY-010 alert list).

## PLAY-010-rescue-butchering-alerts
Rescue, corpses, butchering, alerts · 2026-10-06 · d5ae2f578
- Changed:
  - Doctor work (on by default, Work "Doc" column live). Rescue: a doctor carries an injury-downed colonist to the nearest free bed, the patient follows tile by tile and claims it; an interrupted rescue puts the patient down where the rescuer stands. In bed patients heal 4x faster. Tending: 300 work ticks heal 8 injury at once and healing runs 2x for a day; with no free bed patients are tended in place.
  - Kills leave a carcass item (dustbuck/ridgewolf corpse), hauled into food stockpiles; a predator kill feeds the predator and leaves the carcass. Butcher table (2x1 bench, Production menu): "Butcher dustbuck/ridgewolf, forever" bills; Cook work, 1 carcass to 12 or 6 meat.
  - Self-defence: an undrafted colonist attacked by an animal within 3 tiles fights back if armed, otherwise runs about 7 tiles away, re-planning every 30 ticks.
  - Alert list on the right edge, rebuilt from the read model each frame, replaces the centre banner. Red: animal attacking, needs rescue, starving. Amber: collapsed from exhaustion, low food (< 4 items per colonist), unhappy. Grouped repeats; click cycles the camera.
- Contracts: item `CorpseDustbuck/CorpseRidgewolf`, `IsCorpse()`, `animal.Def.Corpse`; construction `ButcherTable`; recipes `ButcherDustbuck/Ridgewolf`; jobs `Rescue/Tend`; command `WorkDoctor` (`work.DefaultColonist` includes Doctor); `PawnView.WorkDoctor/Lying/InBed/CarriedBy/Tended`.
- Limits: no medicine items, infections or self-tend; downed starving colonists are not fed; carcasses never rot; lying/carried/dead pawns still drawn standing (animation pass next).

## PLAY-011-sleeping-spots
Sleeping spot replaces the cot · 2026-10-06 · 277828723
- Changed: the 1x1 cot is gone. Sleeping spot is a 1x2 floor marking (2x1 rotated, Q/E), free and instant (no frame, no hauling); deconstruct erases it at once. Mood counts it as sleeping on the ground; for rescue, doctoring, claiming and healing it counts as a bed. Sleeper lies along it with the head on the pillow (flatter figure on north-south). The bed is the built 1x2 upgrade. Floating "z"s over sleepers.
- Contracts: construction `Cot` renamed `SleepingSpot` (1x2, Rotatable, Wood 0, Work 0), new `Kind.Instant()` (`placeFrameStuff` completes at once, `markDeconstruct` erases, collected then removed after the loop). Art `overlay/sleeping_spot_<facing>`, `ui_icon/sleeping_spot`; cot sprites retired to `_to_delete/retired_cot`. Architect furniture tool "Cot" is now "Sleeping spot".
- Open: 2x2 double bed with two sleepers (done in PLAY-012).

## PLAY-012-double-bed-stacks-crops
Double bed, big stacks, seven real crops · 2026-10-06 · c32e0f8d2
- Changed:
  - Double bed (2x2): two sleepers share a blanket. Beds have two claim slots (`Building.Claimant2`; `SlotOf/CanClaim/Claim/Release/Owner`), each sleeper in its slot's head cell (`construction.SleepCell`). Colonists prefer the bed they hold; claims share beds so sleep jobs no longer reserve the bed.
  - Stacks: `item.StackLimit` 1000; carry at most `item.CarryLimit` 75 per trip and only what a frame needs. Hauls top up same-kind piles first. Merge overflow spills to the nearest free tile or one with room (no loss). Hover shows counts.
  - Farming: seven real crops replace amberroot (now potatoes). Game days to harvest / yield / soil sensitivity: radish 2/3/40%, lettuce 3.75/4/90%, beans 5/4/60%, potato 7.5/8/30%, corn 8/7/100%, pumpkin 8.5/12/70%, wheat 10/8/80%. Real days at about 12 real days per game day.
  - Plants (wild too) grow only 06:00–20:00. Rate = 100 + (fertility - 100) x sensitivity. Nutrition follows real energy density; raw grain, beans, potatoes are low preference. Colonists eat several portions per sitting (12 max). Balance: about three tiles of potatoes per colonist. Each crop has own sow and harvest work.
  - UI: growing zone crop picker (days on plain/rich/gravel soil, yield, food per item); plant hover shows growth; default zone crop potato.
- Contracts: construction `DoubleBed`, `Kind.Sleepers()`, `SleepCell`, `BuildingView.Claimant2`; item `Amberroot` renamed `Potatoes`, new `Corn/Wheat/Beans/Lettuce/Radishes/Pumpkins`, `Raw()` = any food except rations and meals; plant species (Amberroot removed), `Def.Sens/SowWork/Label`, `Crops()/Rate()/GrowDays()/GrowingHour()`; `Game.Crops() []CropView`; `-capture-ui grow`, `-scenario double`.
- Decisions: 12-day probe, one 2x3 field per crop: harvests radish d3, lettuce d4, beans d6, potatoes d8, corn/pumpkin d9, wheat d11; all colonists alive.
- Limits: no seasons, temperature, irrigation, blight, quality, spoilage; a zone's crop is fixed when painted; stockpiles do not filter by food type.

## PLAY-013-sleep-and-anti-flap
Sleep rules, anti-flap pass, zones over floors · 2026-10-21 · 51d888b3a
- Changed:
  - Sleep rate `needs.SleepGain=2` every `SleepPeriod=5` ticks (20% to 100% in exactly 8 h). Start rules in `phaseAI`: Work hours, a tired colonist keeps working (`needs.SleepAllowed`) until it collapses at 0; Anything/Recreation, sleeps once tired; Sleep block, bed, unless `restedFree` (then free time until rest falls to 20%).
  - Wake rules (`sleepDone`): collapsed stays down until `WakeRest` 80% and hunger does not wake it; hungry with reachable food wakes to eat; Sleep block wakes at 100% and sets `restedFree`; Work hours wakes above 20% (user choice); else wakes at 80%. Collapse: sleeps where it dropped, same tick, log "Collapsed from exhaustion" (`logCollapsed`).
  - Zones now draw over floors (`drawArt`: terrain, ground cover, floors, zones, blueprints; `Renderer.layerTrace`, `TestZoneDrawsOverFloor`).
- Decisions (anti-flap fixes; each new no-flap test fails on the old code):
  - W1/W2 Warm vs Sleep/patient rest interrupts skip `job.Warm`; warmth wakes a sleeper only at `exposure.Recover`. W3 warm interrupt only when a warm spot is found.
  - F1 unreachable food: `noFood` cooldown `foodRetry=500`. B1 unreachable bed falls back to `assignSleepInPlace`. I1 `keepIdling` restores free time after a failed giver. D1 dead `Exhausted()` checks removed.
  - H1 campfire `banked` band: banks at 20 C, relights below 16 C.
  - E1 `Bill.Waiting` latch resumes at `ResumeAt` (10 to 8); `billPending` counts meals in progress.
  - A1/A2 flee release radius `FleeRadius+animal.FleeRelease` (9), finish the flee path. A3 no flee while enraged. C1 unarmed colonist: one "Fled from X" log, `calm` 600 ticks, release radii 8/18, holds position. C2 armed colonist sticky target unless a new threat is 3 tiles closer.
  - Old counts: cold+tired 142 kind changes in 500 ticks, cold patient 300 in 300, unreachable food 1000 failed ticks in 1000.
- Save: `restedFree`, `noFood`, `calm`, `banked` are optional save fields (older saves load); `Bill.Waiting` saved with the bill.
- HUD: no new widget; existing alert plus log line, job line, Rest bar, bill row "(Have enough - resumes at N)".
- Benchmarks: `ColonyHour` 28.2 to 22.3 ms; `ColonyRaid` 30.3 to 23.6 ms; `DrawArt` 5.3 to 3.7–4.5 ms.
- Open: with the default schedule (Sleep 20–05) colonists tire in late Work/Recreation, bed early, wake at 100% around 01–03; a shorter default Sleep block is the user's call. Audited, unchanged: siege re-acquire/breach re-plan, indoor sowing uses live room temperature, prefer beds in warm rooms, predators never give up a chase, tame grazer with failed flee path re-plans every tick, calm window delays bedtime and warmth checks (up to 600 ticks).

## PLAY-022-power-followups
Lamp bulb, solar weather, smooth lamp light · 2026-10-26 · 3e6cc3864
- Changed: standing lamp bulb is a `P.glow` part, lit only while powered (`furniture/lamp_standing_glow`, `drawPowerParts`). Solar output follows weather via `weather.SunFactor(kind, intensity)` in permille: clear 1000, cloudy 700, rain 450, storm 350, snow 400, blizzard 250 at full intensity, scaled by x(0.5 + 0.5*intensity); shared by `power_gen.go` and the renderer's `weatherSun`. Lamp light is smooth: ground quads get per-corner levels (mean of open tiles around the corner, walls excluded, `drawLitGround`); sprites sample continuous distance (`lightGrid.atF`).
- Decisions: no seasonal day length (`lighting.Outdoor` has none); eclipses not modelled. Origin: ENG-005 "left open" items, RimWorld-like behaviour with our own numbers. No golden moved.
- Limits: `BenchmarkDrawArt500` z1 +4%, zmin about +8% (noisy).
- Open: a per-tile light step remains at roof/sky edges; the no-power badge covers the unlit bulb in captures.

## PLAY-023-zones-rimworld-rules
Zone placement rules, dumping stockpile, storage priority · 2026-10-26 · 19905500b
- Changed:
  - `zoneRefusal` with the reason shown in the UI. Every kind: outermost map ring ("Too close to the map edge"), fogged ("Unexplored"), already zoned ("Another zone"), rock wall ("Rock"), wall/impassable building/filled grave ("Building"), blueprint of a blocking building ("Planned building"). Growing: water, bridge, terrain other than soil/rich soil/gravel/marsh (`terrain.GrowSoil`, "Not soil"), built or planned floor ("Floor"). Stockpile and dump: deep water only (shallow now allowed).
  - Under a zone: when a building finishes or terrain changes, refused cells are removed (`RemoveTiles`); a cut zone splits (biggest piece keeps its id, new ids row-major); one message per zone ("Growing zone shrank: floor built"). Saved zones migrate on ground rules only (keep edge and fogged cells).
  - `zone.KindDump` (appended): animal corpses and stone chunks only; `DesignateStockpile` with `Build=1`, "Dumping stockpile" tool (reuses the stockpile icon).
  - Priority Low / Normal (zero) / Preferred / Important / Critical on stockpiles and dumps via `SetZonePriority`, picker and pane line. Haulers search highest first; a stored stack moves only to a strictly higher priority with room, after at least 1 in-game hour (hysteresis, `ZoneMap.StoredAt`).
  - Sowing only between 0 and 50 C growing temperature.
- Decisions: a load dropped in water still floats to dry ground unless a stockpile on that cell takes it. User asked for RimWorld-matching placement rules, original values. No golden moved.
- Open: removing a bridge does not re-check zones. Fixed `TestBridgeZones` (wrong kind string); `TestSetZonePriorityIgnoresBadTargets`, `TestLowOnlyStorageStays` pin priority guarantees.

## PLAY010_PLAN
PLAY-010 play-feedback batch (14 items, up to 15 agents) · 2026-10-23 · 397d16293
- Scope: fourteen play-feedback requests, all deterministic and headless in the sim (sim RNG only), hysteresis or cooldown plus a no-flap test for every threshold behaviour, spatial queries through the indexes (`animal_index.go` `nearestWhere/within`, `plantAt`, `bldgIndex`, `frameIndex`). All names, art and numbers original; the pasted vegetation brief is reference only (no RimWorld plant names or balance; invented plants like cave fungi, the vine, gold-fruit bush, hay left out).
- Requests and user decisions:
  - 1 XS animals bigger but every XS smaller than every S: `sizeLength[SizeXS]=0.44` (hive beetle XS+ to 0.506, below S- 0.51); `TestXSBetweenOldAndS`; read model only, no golden moves.
  - 2 Door/FenceGate over Wall/Fence/Door/FenceGate of a different kind: old piece deconstructed first, then the new built. Same kind stays `ReasonOccupied`; wall over door and floors on these stay refused; a cell with a frame stays `ReasonPlanned`. `Frame.Replaces`, `ReplaceMarked`; `constructTry`/forced build/`OrdersAt` refuse with "The wall must come down first"; cancel works both ways; ghost text "replaces wall" via `ReplacesAt(kind,x,y)`.
  - 3/4 Attacked animals: every animal fights or flees (wild flee classes flee; wild fighters Territorial/Predator/PackPredator/Legendary fight unless badly hurt; tame flee from a colonist, tame fighters fight others). A helpless animal (flee class, tame hit by colonist, badly hurt, attacker unreachable) flees on every hit and re-picks a goal each hit: `FleeGoal` 8 tiles, else BFS (radius 8, 300 cells) maximising distance, else any non-adjacent neighbour, else any neighbour. Slaughter is not an attack. Test: penned sheep hit every 60 ticks for 20 hits moves each time.
  - 5 Predator charge: wild awake Predator/PackPredator, not fleeing/hurt/enraged/tame, no target, only when `predatorsHuntColonists()` (off on Wanderer/Gentle); every 60 ticks staggered by id, `nearestWhere` within 3, 30% roll, then `NextCharge = now+600` (saved); `ChargeSince` saved; ends beyond 10 tiles, after 1200 ticks, or target down. Logs "Charged by a <species>"/"Charged <name>"; action line "Charging <name>".
  - 6 Unarmed undrafted colonists flee from any animal with `Mode==Attack && Target==id` within 12 tiles (16 while calm or fleeing), before queued orders (they resume after `fleeCalm=600`); goal 8 tiles away, then +-45, +-90 degrees, `walkableNear(.,3)` and reachable, prefer inside home; cornered stays. Armed keep 4/8 and fight; raiders 14/18. `PawnView.Fleeing`, Job "Fleeing from <creature>". No-flap test at the 12/16 boundary.
  - 7 Colonists and tame animals walk through each other (`stepGo`, no waiting/sidestep/yield/`makeWay`); wild animals and raiders still block. Combat exception: `inCombat` (drafted, `job.Attack`, or live `g.calm` entry; `combat_state.go`) never ends a move on another colonist's tile; unstack sweep only when one is in combat. Partner layer separate from the one-blocker-per-cell index; `crowdCost` does not price partners. Old blocking tests updated deliberately (new assertion, not weaker): `TestEnterBlockedByStandingPawn`, `TestIdlePawnStepsAside`, `TestHeadOnSwapResolves`, `TestCorridorHeadOnBothArrive`, `TestDoorChokepointQueues`, `TestThreeCycleNoDeadlock`, `TestCrowdToOneCellSpreads`, `TestNoSharedCellInvariant`. Benchmarks `ColonyHour`, `TickGrown/500` within +10%.
  - 8 Grass: composed tufts blended across tiles, never a carpet; copies by growth <25:1, <50:2, <75:4, <90:6, >=90:9 in a jittered 3x3 grid, up to 0.15 tile past the edge; one triangle batch per frame; LOD >=48 px full, 24-48 capped at 4, 16-24 baked into chunk image (chunk version includes `GrassVer`), <16 soft tint. Budgets: `DrawArt500/250` <= 6 ms and 6000 draw calls, `BenchmarkGrassBatch` <= 1.0 ms, fps probe 500^2 z0.25 1% low >= 60. Grass grows: `growth []uint8` lazy (absent = 100), regrow at 15, +per 250 ticks per chunk so 15 to 100 takes 3 days at mild temperature, paused by snow/freezing; grazers prefer fuller tiles, tiles under 30 skipped unless nothing within 30 tiles (hysteresis); `eat` scales `GrazeFood` by growth/100, min 30%; saved additive `GrassGrowth`; `WorldView.GrassGrowth` (copy-on-write) and `GrassVer`; hover "Grass (growing, 40%)".
  - 9 Trees: visible trunk (>=55% of height), canopy overhanging about 0.5 tile above, spill <= 0.2 sideways, ~1.5 tiles tall, no solid core; parts `_trunk`/`_canopy` on the same canvas and origin; canopy alpha eases to 0.4 within 0.2 s when a creature, hovered/selected entity or Architect ghost is on the tree tile, the tile above or the two diagonals above (Reduced Motion snaps); trunk and shadows never fade. `BenchmarkDrawPlants` <= 1.2 ms.
  - 10/11 Vegetation restyle (flat two-band, muted olive, transparent gaps, composable sprites; ART spec §9 rewritten; `assets/palettes/materials_plants.yaml` tokens) and new wild plants appended to `plant.Species`: bramble (walk cost 150, slows), moss, agave, ball_cactus, elephant_ear (110), fire_lily, dandelion; `Def.WalkCost`, `Def.Ground` (never blocks building); per-biome permille placements (rainforest total 155, inside `TestBiomePlants` 140-180); `TestNativePlantsSurviveTheirBiome` must hold; `goldenLegacy` for non-temperate biomes expected to move (re-pin with field diff showing only plants changed); `TestScaleGolden`/`TestGrownGolden` should hold.
  - 12 Parallel limit raised to 15 agents (PARALLEL_WORK, parallel-work skill).
  - 13 Remove the unused 203 MB vendor sprite pack (2,000 tracked files), `pack.go`, vendor `atlas` loaders, `LoadPack`; no-art fallback draws terrain colours and pawn rectangles (`TestRendererWithoutArtDrawsFallback`); history keeps the bytes, no rewrite.
  - 14 Two lasting AGENTS.md rules: remove unused assets in the same change (aliases only while read; `make art-check` fails on orphans via `assets/metadata/orphan_allow.txt`), and grass/plants never have a contour (`outline_width=0`, checked by tooling).
- Decisions: Phase 0 lead contract first (cap, XS size, `inCombat`, species enum, read-model fields, art key contracts, trees builders moved to `families/trees.py`, vendor removal, rules); Phase 1 twelve agents on disjoint files (ART-XS, ART-TREES, ART-GROUND, DOOR, ANIMAL, PAWN-FLEE, MOVE, VEG-SIM, VEG-KEYS, GRASS-DRAW, TREE-DRAW, ART-CHECK) with shared files split by function; Phase 2 lead verification (landing order DOOR/ANIMAL/PAWN-FLEE/MOVE, VEG, art, draw; race tests, determinism x3, art-check, benchmarks, captures, handoffs ART-015/016/017, UI-021, SIM-035..038, ENG-002). `animal.State`, `construction.Frame` and grass save fields are gob-encoded (add fields only); event log kinds append-only; concurrent Blender runs use their own `TMPDIR`.
- Limits: no open checklist row names this plan (its rows are `[x]`: SIM-037 collision, SIM-038 wild plants, rescue), so there is no verbatim Open part.

## PLAY011_PLAN
PLAY-011 auto-oriented doors and gates, in-place material swaps, unmovable fences · 2026-10-24 · d021b7416
- Scope: three requests after play. Doors and fence gates face the right way by themselves; walls/doors/fences/gates cannot be moved and placing the same piece with a different material changes it in place (same material does nothing); fences and gates become unmovable. User decisions: swaps happen in place (a builder converts the piece, the room never opens, nothing walks through); scope is structure + floors + furniture (walls, doors, fences, floors, carpets, sandbags, beds, double and hospital beds, tables, chairs); furniture stays movable.
- Problem found: door art axis was guessed from wall links (wrong between fences, at corners/T-junctions, and in shadows where `shadow.go` always drew `ew=false`); gates stored the Q/E facing; same-kind placement gave `ReasonOccupied`; `construction.Movable` still allowed fences and gates.
- Auto-orientation (`autoFacing(k, x, y, prefer)`, Door and FenceGate only): solid neighbours = Wall/Door/Fence/FenceGate (built or frame) or natural rock. `h` = solid E+W, `v` = solid N+S. `h>v` gives South (spans east-west); `v>h` gives East; tie keeps the current facing (hysteresis, so a piece never flips back and forth). Fixed N,E,S,W read order, no RNG. Runs at placement (Q/E is only the tie-breaker; Door facing is now stored but Door stays non-Rotatable, 1x1) and, via O(1) `reorientAround(x, y)` on the four orthogonal neighbours, from `completeFrame`, `removeBuilding`, frame placement/cancel (`cancelFrame`, `cancelRect`) and `setTerrain`. A door placed before its walls turns to fit.
- Old saves: `saveState.DoorsOriented bool` (additive; new saves true). Loading without it runs `autoFacing(Door, x, y, South)` once per door; gates keep their facing.
- Renderer: door axis from stored facing (South/North = `ew`, East/West = `ns`) for buildings, frames and ghost; `links.mask` guess deleted; shadows use the same axis. New read model `AutoFacingAt(kind, facing, x, y)`; `ghostCell.facing` so each ghost cell previews along the wall run. Hints lose "Q/E rotates" for doors and gates ("faces along the wall or fence; Q/E sets a free-standing one").
- Swap rule: `canPlace(k, stuff, x, y)` / `canPlaceAt(k, stuff, f, x, y)`. `swapTarget` returns the existing building when it has the same Kind and origin (and Facing for rotatable multi-tile kinds; doors/gates compare origin only), every footprint cell is that building, `b.Stuff != stuff`, `StuffOK`, and no frame is planned. Swap cells skip Occupied/Blocked; same material gives neutral `ReasonSame = "already that material"`; partial overlap stays Occupied/Planned. Door over a door of another material is a swap; door over wall/fence stays a PLAY-010 replacement.
- Swap frame: `Frame.Swaps entity.EntityID` (gob additive), no deconstruct mark, `Need` = normal material cost, `CompNeed = 0` (components stay), `WorkLeft = WorkFor(kind, newStuff)`. The frame shares cells via the frame index; walkability untouched; `blockingFrameAt` ignores it; `constructTry` does not wait. `completeFrame` keeps the same building id, sets `b.Stuff = fr.Stuff` and `b.Damage = 0` (HP becomes new `MaxHP`), keeps facing/claimants/ForPrisoners/bills/fuel/items, refunds the old material (`construction.Refund(kind)` of `refundStuff(old)`, no components), `markRoomsDirty()`, `structVer++`, no log. If the target is gone the frame completes as a normal new building.
- Guards: Move refused with `ReasonSwapPending = "Material change in progress"` (`canMoveTo`); deconstructing the target turns the frame into a normal build; cancelling refunds delivered material and leaves the building; a swap also repairs (damage resets) and the repair job (`defense.go`) skips buildings with a pending swap; bed owners keep their claim and sleepers are not ejected; saves round-trip `Swaps`.
- UI: `ReplacesAt(kind, stuff, x, y)` returns "replaces wall", a swap label like "wood wall → steel", or empty; `ReasonSame` cells draw neutral grey and `submitArchitect` skips them (so dragging steel over a half-steel run sends only the wood half); count/cost totals count only swap and new cells. Frame inspector "Changing to steel"; building inspector "Changing to steel (N%)" from `BuildingView.SwapTo/SwapPct`. `construction.Movable` and UI `movableKind` exclude Fence and FenceGate (Move disabled, "Can't be moved"; existing fences just lose the option).
- Execution: lead Phase 0 contract commit (`ReasonSame`, `ReasonSwapPending`, `Frame.Swaps`, `DoorsOriented`, `BuildingView.SwapTo/SwapPct`, interface stubs), then two parallel agents: SIM (opus; placement/tick/frame_view/move/defense/world/save/construction) and RENDER (sonnet; ebiten files), TDD, no commits.
- Tests pinned: orientation cases (horizontal/vertical runs, between fences, T-junction picks longer run, tie keeps facing, door-first-then-walls, removing one wall of a 2/2 cross does not flip, no-flap alternating build/remove changes facing at most once per real majority change, rock cliff, old save); swap cases (wall, door stays working, bed keeps claimant, hospital bed needs no components, floor, carpet colour, table footprint, `ReasonSame` gives no frame or message, gate swap always `ReasonSame`); `TestCannotMoveWallOrGrave` extended to fence/gate; deliberately updated `TestReplaceCanPlaceRules` and `TestCanPlaceReasons`.
- Verification/docs plan: determinism x3, goldens expected to hold (legacy colonies build no doors/gates; re-pin only after a field diff), no new art; handoff was to be `UI-022-orientation-and-material-swap`; checklist rows Blueprint, Deconstruction/Replace, Ghost blueprint, Validity visualization; README construction line.
- Limits: no open checklist row names this plan, so there is no verbatim Open part.

## PLAY012_PLAN
PLAY-012 Plan tool and crisp UI scaling (Auto + presets, HiDPI) · 2026-10-24 · 808ab0452
- Scope: two requests. (1) A Plan tool: purely visual placeholder marks painted on tiles, blocking nothing, that vanish when a finished building stands on the tile (user choice: any building clears it, floors do not). (2) UI that scales with the screen: crisp native scaling (text and chrome redrawn at real size, not stretched), real HiDPI (Retina renders at true resolution), Options offers Auto + presets.
- Plan tool (done; checklist row `[x]` UI-023): mirrors `home.go`. `planState{cells []uint8; ver uint32}` (`planMarks`, lazily allocated); append-only `command.DesignatePlan` (`X,Y,X2,Y2,Enabled`, saved in `Pending`); `paintPlan` normalises, clips, bumps `ver` only on real change. No job giver, reservation, path cost or room effect reads marks. Auto-clear in `completeFrame`'s footprint loop for any non-floor-layer kind (`Layer != LayerFloor`), also on `completeMove` (new footprint); `completeSwap` changes nothing. Read model `WorldView.Plan []uint8`, `PlanVer` via `shareLayer(&g.viewPlan, ...)`. Save: `saveState.Plan []int32` sparse sorted indexes, nil when empty so goldens hold; hashed as saved.
- Plan UI: Architect Orders tools `plan`/`unplan` (`placeArea`, filled rectangle/oval), icons `ui_icon/plan`, `ui_icon/plan_clear` from the overlay family; `plan_draw.go` batch cached by `PlanVer`, drawn after zones under blueprints; palette `plan_fill` #B8D4F0 alpha 0x30 and `plan_edge` #D8E8FF alpha 0x90 in `assets/palettes/ui.yaml` and `widgets.go` (`TestUIPaletteMatchesYAML`); always visible, HUD toggle "Plans" beside Home hides them, plan tools force visible while held; hover readout "Planned".
- UI scaling model (Part B, see Open): `s = dev x ui`; `dev` from `ebiten.Monitor().DeviceScaleFactor()`; Auto `ui = clamp(windowHeight/720, 1.0, 2.0)` rounded to 0.05; presets 75/90/100/115/130/150/175/200 %. UI lays out in logical pixels through a scaling canvas (`ui_canvas.go` helpers `uiFillRect`, `uiStrokeRect`, `uiLine`, `uiImage`, `uiImageOp`, `uiTriangles`); text face created at `size x s`, metrics divided by `s`; map multiplies tile size by `dev` only; captures force `dev = ui = 1` (byte-identical). `Layout` becomes `LayoutF` (physical size, logical `sw/sh = ceil(p/s)`); cursor split (`raw.mx/my` physical for the map, `ui.mx/my = raw/s` for widgets); `colonyViewport` logical plus `colonyViewportPhysical()`. Settings: `Accessibility.UIScale` widened to 75-200, new `UIScaleAuto` (default true; old files load Auto on) and `Graphics.HiDPI` (default true); live apply, no restart. Minimum-window checks use logical size ("Enlarge the window or lower the UI scale"). Options: UI scale stepper (Auto shows current value, focus restored) and a "Sharp on high-density screens" toggle; no separate text slider (YAGNI); presentation-only, never reaches sim or saves.
- Sweep rule: camera-positioned things (world) stay raw physical; screen-layout things use `ui*` helpers with logical coordinates; mixed files split explicitly (architect, minimap, room_stats, devmode, orders, move_mode, labels, selection, globe, landing; backgrounds/splash unscaled). Guard test `TestUIDrawsThroughCanvas` allowlists the files that may draw raw (world renderer, backgrounds, `ui_canvas.go`), so a new raw-drawing UI file fails the build. Existing two-size draw tests rerun at s = 1.0, 1.5, 2.0 with a rect inside-screen and non-overlap check; font_test global mutation (race) removed; `-capture-ui menu|pawns|colony` hash test unchanged at default settings.
- Execution: lead Phase 0 contract commit (plan command/view/save/palette and the whole scaling core, guard test with a temporary allowlist), Phase 1 eight agents (PLAN-SIM, PLAN-UI, HIDPI-MAP (opus), SWEEP-HUD, SWEEP-PANELS, SWEEP-TOOLS, SWEEP-SCREENS, OPTIONS), Phase 2 lead checks including `-fpsprobe 60` at 500x500 z0.25 and z1 with HiDPI on/off (if the 1% low drops under 60 at 2x, HiDPI defaults off for that resolution class; result recorded either way). Handoffs planned `UI-023-plan-tool`, `UI-024-ui-scaling`.
- Open: the UI scale and density row is `[-]` (PLAY-012 UI-024, plus PLAY-017/018); still missing per the checklist: spacing and row heights are per-file constants, not a central token set. The scaling design (Part B) is kept verbatim below.

### Open (verbatim)
#### Part B: crisp UI scaling

### B.1 Model: two scales, one rule
- **`dev`:** the device pixel density, from `ebiten.Monitor().DeviceScaleFactor()` (1 on normal screens, 2 on Retina). It is used so the game renders at the screen's true resolution.
- **`ui`:** how big the interface looks. The total UI scale is `s = dev × ui`.
  - **Auto (default):** `ui = clamp(windowHeightInPoints / 720, 1.0, 2.0)`, rounded to the nearest 0.05. A 720p-tall window gives 100% and a 1080p window 150%. The UI keeps about the same proportion of the screen as the baseline layout.
  - **Presets:** 75, 90, 100, 115, 130, 150, 175 and 200 %.
- **Logical UI size:** `lw = physicalW / s`, `lh = physicalH / s`. The UI lays itself out in **logical pixels** exactly as today (every existing constant and test stays valid at s = 1) and is drawn at physical resolution through a scaling canvas.
- **The map** renders at physical resolution and multiplies its tile size by `dev` only. The world keeps its apparent size on Retina but gets sharper, and the UI scale never zooms the map.
- **Captures** (`cfg.Capture != nil`) force `dev = 1` and `ui = 1`, so captures stay byte-identical.

### B.2 Core (lead, Phase 0, new `ui_scale.go` + `font.go` + `app.go`)
- **`Layout` becomes `LayoutF`:**
  - `func (a *App) LayoutF(outsideW, outsideH float64) (float64, float64)` returns `outside × dev` (physical pixels) and records:
    - `a.pw, a.ph`: the physical size;
    - `a.s`: the UI scale;
    - `a.sw, a.sh`: the **logical** size, `ceil(pw/s)`, `ceil(ph/s)`.
  - Everything that reads `a.sw/a.sh` (layout, hit lists, tests) keeps working in logical pixels.
  - Ebiten's `Layout` stays implemented for older call paths and returns the same rounded values.
- **Scale state:** the per-App value `uiScaleState{dev, ui, s float64}` is mirrored into one package value `curUIScale` (read by the draw helpers). It is set once per frame on the main goroutine before drawing. Tests that need a scale use a helper that sets and restores it (not parallel). The existing global `uiScale` in `font.go` is removed and replaced by `curUIScale.s`.
- **Scaled drawing helpers** (`ui_canvas.go`). Each takes logical coordinates and draws at physical resolution:
  - `uiFillRect(dst, x, y, w, h, clr)`, `uiStrokeRect(dst, x, y, w, h, width, clr)`, `uiLine(dst, x0, y0, x1, y1, width, clr)`, `uiFillCircle` / `uiStrokeCircle` if used;
  - `uiImage(dst, img, x, y, w, h, opts)`, which draws an image into a logical rectangle and fixes the GeoM scale;
  - `uiImageOp(op *ebiten.DrawImageOptions)`, which appends `GeoM.Scale(s, s)` for code that builds its own GeoM;
  - `uiTriangles(dst, vs, is, img, opts)`, which scales vertex positions (sparklines, progress bars, the overlay batches drawn in UI space).
- **Text:**
  - `drawText`, `measure`, `lineH`, `wrapPx` and `ellipsize` keep logical inputs and outputs.
  - The face is created at `size × s` (cached per (size, bold, s)). It is drawn at `(x×s, y×s)` and its metrics are divided by `s`, so wrap and ellipsis decisions are the same at every scale and text is always rasterised at the real pixel size.
  - `debugPrintAt` and `drawLabel` already forward to `drawText`.
- **Panels and buttons:** `drawPanel`, `drawButton`, `drawLabel` and the widget-kit primitives (`ui_*.go`) call the helpers internally, so most screens become scaled through them.
- **Cursor:**
  - `pollInput` keeps `raw.mx/my` in physical pixels for the map (`screenToWorld`, globe dragging and picking, map selection, minimap clicks become UI space, see B.4).
  - It also sets `ui.mx/my = raw / s`, which is what the widget kit, `layoutHits`, `hitButton`, `currentCursor` and every HUD/menu hit test read.
  - `a.cursor` (captures/tests) is given in logical pixels.
- **Viewport:** `colonyViewport(sw, sh)` keeps returning the map area in **logical** pixels for UI layout. A new `colonyViewportPhysical()` (logical × s) feeds the renderer's culling, camera clamp and `screenToWorld`. The HUD bar heights then occupy `hudBarH × s` physical pixels, so the map's visible area matches the scaled chrome.
- **Settings** (`internal/platform/config/settings.go`):
  - `Accessibility.UIScale` keeps its meaning (percent) but widens to 75–200.
  - New `Accessibility.UIScaleAuto bool` (default true) and `Graphics.HiDPI bool` (default true: render at the screen's true resolution).
  - `Normalize`: a 0 scale means 100, and values clamp and snap to the presets.
  - Old settings files (`UIScale = 100`, no Auto field) load as Auto on, since the zero value means "unset" and is treated as true via a `*bool` or an explicit migration in `Normalize`.
  - `applySettings` (`settings.go`) recomputes `ui`/`dev` live, with no restart.
- **Minimum window:** `tooSmall` / `drawTooSmall` and `minSetupW/H` (1280×720) compare against the **logical** size. A large preset on a small window shows the "Enlarge the window or lower the UI scale" note instead of overlapping, and the text names both fixes.

### B.3 Options (Accessibility tab, `screen_options.go`)
Replace the "planned" note with:
- **UI scale:** a stepper `Auto · 75% · 90% · 100% · 115% · 130% · 150% · 175% · 200%`. Auto shows its current value, e.g. "Auto (150%)". Changes apply immediately. A changed scale keeps the Options screen open at the same tab and row (focus restored), so the stepper stays under the cursor's logical position.
- **Sharp on high-density screens** (on/off, Graphics tab): HiDPI rendering. The tooltip notes it costs GPU time on very large maps.
- **Text size stays tied to UI scale;** there is no separate slider (YAGNI).
- **Persistence:** `settings.json` via the existing `setSettings`. Both are presentation-only and never reach the sim or saves.

### B.4 The sweep: every UI draw goes through the canvas
The map stays raw. The rule: anything positioned by the **camera** (world space) keeps raw `vector`/`DrawImage` in physical pixels. Anything positioned by **screen layout** (panels, buttons, text, icons, HUD, menus, tooltips, the minimap frame) uses the `ui*` helpers with logical coordinates.

Files that mix both get the split explicitly:

| File | Split |
|---|---|
| `architect.go` | ghosts are world, the palette and info box are UI |
| `minimap.go` | the box is UI; the map image draws into the scaled box; clicks convert logical → map |
| `room_stats.go` | the hover panel is UI |
| `devmode.go` | the debug overlays: tile markers are world, panels are UI |
| `orders.go` | the context menu is UI |
| `move_mode.go` | the ghost is world, hints are UI |
| `labels.go` | name labels are world-anchored, but text should scale with the UI: positioned in world space and drawn with `drawText` at UI scale (anchor converted physical → logical) |
| `selection.go` | brackets are world |
| `globe.go` | the globe is drawn at physical size inside a logical panel rect converted with `× s` |
| `screen_landing.go` | panels are UI, the globe as above |
| `setup_backdrop.go`, `menu_diorama.go`, `splash.go` | full-screen backgrounds: physical size, unscaled |

- **Guard test:** `TestUIDrawsThroughCanvas` lists the files that may call `vector.*`, `ebitenutil.*` or `.DrawImage(` directly. It covers the world renderer, the backgrounds and `ui_canvas.go` itself, with a reason per file. It scans the package source, and the build fails if a new UI file draws raw.
- **Hit rectangles** need no change: widgets keep registering logical rects, and input is logical.

### B.5 Tests and checks (core + sweep)
- **`ui_scale_test.go`:**
  - the auto formula (720 → 1.0, 1080 → 1.5, 1440 → 2.0, 600 → 1.0);
  - presets snap;
  - captures force 1.0;
  - `LayoutF` with dev = 2 returns physical size and logical `sw/sh = physical / s`;
  - text measured at s = 1 and s = 2 gives the same logical width ±1 px;
  - a button drawn at s = 1.5 is hit at its logical centre through the scaled cursor;
  - the map viewport physical = logical × s;
  - `screenToWorld` uses raw coordinates.
- **Re-run the existing two-size draw tests** (`newgame_test.go:873`, `screen_pawns_test.go:419`, `screen_worldgen_test.go:333`, `app_test.go:381`, `screen_landing_test.go:409`, `editor_test.go:526`) at s = 1.0 and also at s = 1.5 and s = 2.0 (logical 1280×720 kept). Nothing overlaps or leaves the screen (a widget-rect inside-screen and non-overlap check added to the kit's debug hit list).
- **`TestSetupMinWindow` / `TestNewGameScreensNeedMinWindow`** compare in logical pixels.
- **Font race:** removing the `font_test.go` global mutation fixes the race.
- **Captures:** `-capture-ui menu`, `pawns` and `colony` give byte-identical output before and after the change at default settings (a hash test over one capture state).


## PLAY013_PLAN
MAPGEN-2 field-based local map generator, debug views, fog of war (file is PLAY013_PLAN) · 2026-10-24 · c91b56439
- Scope: natural-looking colony maps from ordered passes (large coherent shapes, medium patches, small irregular detail), each pass on its own random stream, with debug views of every field. User decisions: the legacy generator stays frozen (`Generate`/`GenerateBiome` serve tests, `-scenario` set pieces and old captures, hash pins intact; every new-game landing-site map uses the new pipeline); debug views are both an in-game developer overlay and a `cmd/mapgen` PNG tool (seed grids, hilliness sweeps); scope covers the pipeline, vegetation clusters and wildlife by habitat, natural mountain roofs (thin/thick), surface ruins, geothermal vents, stone-chunk debris, roads following world roads, start-spot scoring, validation/repair, and fog of war; no sealed "ancient danger" rooms this round. Originality (ENG-003 not invoked): the pasted spec quotes reference constants; we use generic techniques with our own numbers and names, tuned by eye with the debug views.
- Before: `GenerateSite` ran baseTerrain, caves, coast, river, lakes, wetGround, road, safety, plants on one shared RNG stream (changing any pass moved every plant); `pkg/noise` is size-dependent value noise; no fog, no power system, no map-image tool; start is the map centre.
- Architecture: new pure package `internal/domain/world/mapgen` with `Config{Seed, Width, Height, Site, Version}`, `Fields` (float32 grids: Elevation, MountainBias, Fertility, Wetness, VegDensity, ChunkDensity, RiverDist, CoastDist, RockDepth, StartScore, Stone, masks), `Result` (Terrain, Roof, Stones, Ores, Plants, Grass, Chunks, Ruins, Vents, Roads, Start, Fields when Debug, Metrics), `Step`/`Ctx` with `Rand(step)` = one independent stream per step (adding a boulder pass never changes plants; no map iteration; slices sorted before random choice), `Generate` and a `Builder` for the loading screen. `Version`/`GenVersion` (=2) stored in Setup so saved colonies never regenerate differently. New `pkg/noise2`: hash-based 2-D gradient noise (`Gradient`, `FBm`, `Ridged`, `Warp`), RNG-free, order- and size-independent, outputs in [0,1]. Additive `site.Site` fields `MountainDir`, `RainClass`, `CoastWidth` (legacy path never sees them).
- Pipeline (19 steps): elevation, fertility, wetness, material, rockmask, caves, coast, river, lakes, rocks, terrain, ores, chunks, roads, start, ruins, vents, vegetation, validate. Start chosen after landforms and water so features keep distance and vegetation leaves its pad usable. Wildlife stays in the game layer.
- Landforms (original numbers): domain-warped FBm at three scales (weights 0.55/0.30/0.15, scaled by sqrt(W*H)/250); mountain bias weights Flat 0, Small 0.08, Large 0.18, Mountainous 0.38; rock share by quantile (Flat ~4%, Small ~10%, Large ~22%, Mountainous ~42%, each +-25%) plus rough-stone and gravel bands. Caves (site.Caves only): components >= 450 cells, tapering noise-steered tunnels with branching (width 3.5-5.5 down to 1.3, 7% branch chance after 12 steps, 3 levels). Coast bands (deep, shallow 4-7, beach 2-8). River: smoothed displaced centreline, width by size, bank bands plus fertile floodplain, canyon through rock for large rivers in Large/Mountainous, crossing guarantee (shallow stretch at least every 60-90 cells). Material: one low-frequency FBm per permitted stone, argmax regions (1-3 broad regions). Rock cleanup (drop <= 5-cell components except Flat outcrops, fill <= 3 holes, trim spikes); `RockDepth`; roofs: thin `roof.Natural` at depth >= 2, new `roof.NaturalThick` at depth >= 7 (not removable by roof tools, "Thick rock overhead"; never targeted by drop pods; never collapses; no sky light). Terrain precedence: water, rock, cave floor, water edges, elevation bands, biome patches (data-driven with MinSize), fertility quantile ground, soil; patch cleanup minimums (sand 6, marsh 8, rich soil 6, gravel 4, mud 5).
- Features: ore lumps (depth thresholds steel 2, silver/machinery 3, gold 5; spacing 6; irregular lumps by frontier trimming); stone chunks (density x 1.6 near rock, p = 0.004 x density, formations of 1-4; ordinary items); roads (A* cost grid, Chaikin smoothing x3, 2-wide Track with soft edges, junction near start); small surface ruins 0-3 (broken wall run, roofless shed, loot table; >= 20 from start; `Building.Ruin` bool excluded from wealth/home/room scores, deconstruct refund x0.5); geothermal vents 1-3 (2x2 `SteamVent` kind, Natural, walkable, blocks building; presentation steam bursts; heat +1.5 C into its room or +6 C in 5x5 outdoor cells via `heat.go`; no power system yet, records `PowerReady` for a future geothermal generator, checklist `[-]`).
- Vegetation: hierarchical clustered placement (VegDensity x suitability x clustering field, 8x8 saturation buckets, species balance x0.5/x1.5, minimum spacing, growth 10-100%, trees 25-100%); grass mask from Fertility x Wetness x VegDensity for Setup games (PLAY-010 grass growth starts full). Wildlife for Setup games spawns by habitat (reachable dry land, >= 25 from start, weighted to grass for grazers, domestic/pet excluded, cap D15 unchanged).
- Start and validation: start score on a coarse grid (open buildable land within 12 >= 70%, reachability, >= 25 from edges, trees within 20, stone within 40, fertile soil within 20, no deep water/rock/vent within 4, mild centre preference); local repair if none passes; structural validators with repairs; metrics (not hard failures); profile tests over 30 seeds (Mountainous largest rock component >= 55% of rock, Flat rock <= 8% and open >= 70%, forest >= 4 tree clusters on 250^2). `Start` replaces the map centre everywhere a Setup game used it (arrival, `startHome`, camera, `startTarget`, wildlife distance, clear areas).
- Fog of war: `fog []uint64` bitset (1 hidden), saved sparsely as hidden-run pairs; legacy colonies and old saves have no fog; Setup games start fogged. Initial reveal floods from the start over walkable cells (doors pass) revealing reached cells plus 8-neighbours; open caves connected to the surface revealed, sealed caves hidden. Mining/deconstruction/door opening flood-reveals the adjacent hidden region (O(region), message "A cave opened up."). `World()`/`Snapshot()` filter hidden things while AI sees truth; placement refused with `ReasonFogged = "unexplored"` except mine designations on hidden rock; hover "Unexplored". `WorldView.Fog`/`FogVer`; renderer draws #0B0B0E with a 1-tile soft edge cached per chunk, black on the minimap.
- Debug: `cmd/mapgen` (flags for seed, biome, hills, size, river, coast, caves, stones, road, `-grid`, `-sweep hills`, `-tile`) writes one PNG per field plus `metrics.json`; pure Go. In-game dev menu "Map fields" cycles layers (fields regenerated on demand from the saved Setup via `game.MapFields(name)`, never saved; alpha 0.7 world overlay with legend; read-only, dev mode only).
- Integration/saves: builder phase per pipeline step with loading-screen names; `applySite` uses `Result` (new `geology.FromResult`); saves gain `Setup.GenVersion`, fog, `Building.Ruin`, `roof.NaturalThick`, all gob-additive. Scale and grown goldens use the legacy path and must hold; new mapgen golden pins per biome x hilliness (12) plus profile tests. Budgets: `BenchmarkGenerate250` <= 0.6 s, `BenchmarkGenerate500` <= 2.0 s, fog reveal <= 1 ms per mining event.
- Execution: Phase 0 lead contract (noise2, mapgen skeleton, additive types, read-model fields, palette tokens); Phase 1 ten agents (LANDFORM, WATER, GROUND, RESOURCES, ROADS, FEATURES, VEG, START, MAPGEN-TOOL, ART); Phase 2 INTEGRATE, WILDLIFE, FOG-SIM, FOG-RENDER, DEBUG-OVERLAY; Phase 3 lead verification (12-seed contact sheets per biome). Handoffs planned `SIM-039-mapgen-pipeline`, `SIM-040-fog-of-war`, `ART-019-vents-ruins`.
- Open: UI/UX row "Debug overlays" is `[-]` (MAPGEN-2); it lacks path/region-cost and temperature overlays, which this plan does not describe. The in-game overlay section is kept verbatim below in case it is wanted.

### Open (verbatim)
### 6.2 In-game developer overlay
- **Dev menu "Map fields":** cycles off / elevation / fertility / wetness / rock types / caves / river distance / terrain classes / vegetation density / start score / roofs / fog / regions.
- **Data:** the game regenerates the debug fields on demand from the saved Setup (`game.MapFields(name)`, run on a worker, then cached). The fields are never saved.
- **Rendering:** a world-space image over the map (alpha 0.7) with a small legend in the HUD.
- Read-only; dev mode only.

## PLAY014_PLAN
MAPGEN-3 cave plants, road re-routing, bridges, sealed vaults (file is PLAY014_PLAN) · 2026-10-24 · ab8b6704d
- Scope: four gaps left by MAPGEN-2: cave vegetation, road re-routing in validation, bridges, sealed ancient rooms; three systems sharing one map. User choices: sealed rooms hold a hive threat plus sleeper caskets; bridges are built by players and generated on roads; bridge load is light only (wooden wall fine; stone and steel walls refused; flooring refused). Originality: follow the spec's behaviour and architecture with our own names, numbers, text and sprites (Bryolux is "duskmoss", ancient danger is "sealed vault", cryptosleep casket is "sleeper casket", warning letter "Ominous walls"); no reference number copied; screenshots are feel references only, never traced.
- Generation order: world tile, geography, environment, structural features (road bridges in `roads`; vault footprints in a new `vaults` step right after `start`, before ruins), populate (vegetation incl. cave plants never on bridges/vault walls/caskets; vault contents in mapgen; defenders and occupants in the game), finalize (validate, then fog, rooms, vault triggers in the game).
- Cave vegetation: `lighting.Cell(outdoor, roofed, artificial)`; new `game/light.go` artificial light grid (lamps radius 6 at 0.7, campfires radius 5 at 0.6, duskmoss glow 0.1 radius 1), cached and rebuilt only on `structVer` or lamp/campfire/duskmoss change; `cellLight = max(outdoor if unroofed, artificial)`; renderer sprite lighting unchanged. `plant.Def` gains `MinLight` (0 = no rule, existing plants unchanged), `DiesInLight`, `Glow`, `Cave`. Ordinary moss `MinLight` 0.35 (no growth in dark caves). New `Duskmoss`: slate-blue cave moss, `MinLight` 0, `DiesInLight` 0.45, `Glow` 0.1, GrowTicks about 40 days, `WalkCost` 125, Browse, Ground, Cave, ColdKill -40 C, HeatKill +45 C, fertility 40 on rough/smoothed stone for Cave species. Light damage: 8% growth per in-game hour while `cellLight >= DiesInLight`, withers and is removed with "Duskmoss withered in the light" once per area; hysteresis: harm stops only below 0.35 (no-flap test). mapgen cave pass: eligible dark cave floor (Stone/Gravel, not Blocked, naturally roofed), clusters of 1-4 touching cells seeded on about 7% of eligible cells, 15-25% of dark floor covered, clusters never touch; lit cave mouths may get ordinary moss, never duskmoss; growth 30-100%; no grass in caves. Renderer: faint cool point light at each duskmoss; inspector "Needs darkness"/"Needs light"/"Withering in the light".
- Road connectivity: `Fields.RoadEnds`; export `roads.Route` (A*) and `roads.Lay`; A* failure retries with rock cut at 25 everywhere. `validate.roadsConnected` BFS from each edge point to its junction, re-routes from the farthest connected cell and logs "road N re-routed", reverts dangling pieces if impossible. Test: every road connects over 30 seeds x profiles; connected roads untouched.
- Bridges (foundation layer): cell layers terrain / foundation / structure / items, effective support is the higher of terrain and foundation; removal restores terrain limits. `construction.Support` (None/Light/Medium/Heavy); `terrain.Support` (water None, land Heavy so land rules unchanged); `Requires`: stone-block/steel walls Heavy, wooden walls, doors, fences, gates, furniture Light, floors and carpets Heavy (refused on bridges). `terrain.Bridgeable`: ShallowWater, DeepWater, Marsh, Mud; deep ocean excluded (`Result.Ocean` from the coast mask; saved ocean bitset for new colonies; legacy maps treat all deep water as bridgeable river). `construction.Bridge` (wood only, `LayerFoundation`, Wood 6, Work 110, Salvage 3, flammable, Light support); `research.Bridges` unlocks `build:bridge`; reasons `ReasonNotBridgeable`, `ReasonOpenSea`, `ReasonNoSupport`, `ReasonBridgeInUse`. One helper `g.cellWalkable` at every walkable-bit write site (setTerrain, move, removal, completion, placeGenerated, burial). Bridge moves at 100% (`treadFloor`), no soaking, preferred to wading; frames over deep water worked from adjacent walkable/bridged cells; deconstruction refused while a structure stands on it; destroyed bridge destroys what it supports (no refund, items on deep water "sank", pawns relocated). Stockpile zones allowed on bridges, growing zones not. Foundation is a gob building.
- Generated road bridges: where a road crosses river/lake water (never ocean) it crosses on a straight 2-wide contiguous wooden bridge bank to bank (water routing cost 12, straightened to the river's local normal from `F.RiverLine`, else the chord); bridge cells in `Result.Bridges`/`F.Road`, dry cells Track; bridge cells count walkable for reachability/connectivity/I6.
- Art/render: flat low timber deck, 16 connection variants by open-side mask, construction frame, `ui_icon/bridge`; foundation drawn after terrain and before floors/objects, water shadow on open edges; Architect "Bridge" tool in Structure (placeFill, research-locked); inspector "Wooden bridge" with support "light: no stone walls or floors"; hover "Bridge over deep water"/"over marsh".
- Sealed vaults: mapgen `features.Vaults` after `start`: one per 250x250 area (1-3, attempts may fail, none on SeaIce); casket hall of k=1-4 bays (3 wide each, one 1x2 casket), loot/defender strip, 1-cell intact wall ring (local stone 70%, steel 30%); interior (3k+1) x 7 or 10, with ring 6-15 x 9-12; embedded >= 70% in rock, >= 30 from landing, sealed; floors of local stone blocks; natural roof kept, `Blocked` set. Occupants: grateful sleeper 30%, hostile ancient 25%, sleeper + hive crawler 15%, two hive beetles 10%, empty 10%, long-dead remains with gear 10%. Loot 3-6 stacks (components 4-12, IndustrialMedicine 3-8, silver 150-450, gold 20-70, rifle 35%, prosthetic/GlassEye/SparkTablet 15%).
- Vault game layer (`vault.go`, `casket.go`): saved state machine Generated, Warned, Breached, CasketsOpened, Cleared (`saveState.Vaults`); `Building.Ancient` (not wealth, not Home anchor, not repaired). Defenders: dormant hive insects `n = clamp(round(threatPoints/50), 2, 6)` per vault, brutes <= n/3; none when ThreatScale is 0/lowest preset or `incidentDisabled("hive_swarm")`; `animal.Animal.Dormant` skips thinking, inspector "Dormant". Warning every 60 ticks when a colonist is within bounds+2: letter "Ominous walls" once, never after the interior is revealed. Breach when `fogOpen` reveals an interior cell: defenders wake (`Manhunter = hiveForever`), letter "The vault is open. Hive insects stir inside!", History entry; breach does not open caskets. `construction.SleeperCasket` (1x2, blocks movement, Ancient, `Building.Sleeper`, `CasketGroup`): command `OpenCasket{X,Y}` (250 work ticks) or any damage opens every casket in its group; outcomes grateful sleeper via `pawngen` joins ("A sleeper wakes and asks to stay"), hostile ancient as one-person raider group, sleeper+crawler, beetles, empty, remains; empty casket deconstructs for steel (salvage 20), no bed reuse; Cleared when every defender/hostile occupant is dead or gone. Fog keeps the interior hidden. Art `structure/sleeper_casket_closed/_open`, `ui_icon/open_casket`; sound `sfx.casket_open`; `cmd/mapgen` draws vaults and bridges.
- Execution: Phase 0 lead contract, Phase 1 seven agents (CAVE-PLANTS, ROADS, VAULT-GEN, BRIDGE-GAME, VAULT-GAME, ART, RENDER), Phase 2 lead verification (mapgen goldens re-pinned once with reason, `BenchmarkGenerate500` <= 2 s, fps probe on a grown colony). Checklists planned: "Ancient ruins/debris" `[x]`, bridges note, duskmoss and light rules, lighting item `[-]` (simulation cell light for plants).
- Open: vanilla row "Roof/indoor darkness" is `[-]` (MAPGEN-3: roofed cells are dark to plants unless lit; no work or mood effects); the plan's simulation cell-light design, which owns it, is kept verbatim below.

### Open (verbatim)
#### 1. Cave vegetation (duskmoss, ordinary moss needs light)
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

## PLAY015_PLAN
PLAY-015 areas, pens, Home, snow clearing and fire · 2026-10-24 · cb6a3e0e6
- Scope: a pasted spec of RimWorld-style areas, zones, colonist and animal restrictions, pens and Home. Before: SIM-046 minimal (10-area bitset with paint/rename, manager only from Assign tab, hard path wall plus job-start target check, no Home choice, no animals, haul destination and sleep ignored areas); pens were painted zones (no marker, flood fill, settings, roping, no livestock/pet split, gates let all tame animals through); Home only auto-expanded around buildings and repair ignored it; fire did not exist (domain stub, lightning only visual); snow was one map-wide number. User decisions: cumulative scope (areas, pens, Home, plus per-tile snow with a snow-clearing area, plus a full fire system; "tell me if you meant to drop snow clearing"); painted pens auto-convert to marker pens on load; allowed areas soft and spec-faithful (only destinations must be inside, paths prefer the area but may cross); unpenned livestock roams, raises an alert and may wander off the map. Originality: behaviour and architecture followed, names/numbers/text/art ours (10 areas instead of 8, our marker cost and fire rates, "Snow clearing area" not "Snow and sand removal"). Goldens moved by design (gates hold livestock, soft areas, per-tile snow, lightning fires): pen scenario, grown, rain/storm; each re-pin justified field by field; `TestTemperateGenerationUnchanged` and map generation must not move.
- Areas: `Area{ID uint16 (stable, never reused), Name, Color (10-colour palette), Bits}`, `MaxAreas` 10, `Delete`, `Invert(id,w,h)` (padding bits untouched), `ByID`, `Clear`; old positional areas migrate to ids 1..n. `AreaRef`: 0 Unrestricted, special value Home (live mask), else id; deleting resets users to Unrestricted ("N pawns are now unrestricted"). Commands `CreateArea`, `DeleteArea`, `InvertArea`, existing `PaintArea/RenameArea/AssignArea` (now animals and Home). Soft traversal: path cost outside the area x4, reachability no longer area-limited. Destination eligibility (inside area or Home) for work targets, haul destination (`DestX/Y`), hunting (prey outside 600 ticks abandons), food, bed and sleep spot (Sleep removed from exempt list; no bed inside means floor inside the area), recreation, wander. Overrides: drafted, mental break, fleeing, combat Attack response, rescue, tend, surgery, animals following a drafted master. Cleaning, repair and firefighting need the target inside Home AND the pawn's area (Unrestricted = Home only). `AreaView{ID, Name, Color, Cells, Ver}` copy-on-write. UI: Architect Zone "Expand/Clear allowed area" submenus with area rows plus "Manage allowed areas..."; tinted overlay baked per chunk by area version; modal manager (swatch, rename, invert, red delete with confirm, "New area" disabled at 10); selectors in Schedule tab (Allowed area column), Animals tab (Area / Pen), pawn inspector line "Area: Indoors"; Assign tab Area column removed.
- Pens: `animal.Roster.Penned` explicit per species (farm species penned; pets and trainable animals use areas), reviewed against 61 species, mirrored in the creatures art mirror. `construction.PenMarker` 1x1, 10 wood, Architect Misc. Enclosure flood fill from the marker over cells a penned animal can stand on (barriers: fences, closed gates/doors, walls, rock, deep water, blocking buildings); not enclosed if it reaches the map edge or exceeds 2,500 cells ("Not enclosed", holds nothing; alert "Pen 2 is open", penned animals start roaming); recomputed lazily on structure version, cached per marker. Gates: penned animals pass only while led by a handler; area animals pass as today; wild neither. Marker inspector: Animals (per species allow + male/female, All/None), Food (grazing per day, herd demand, trough stock, verdict "Enough grazing" / "Short by N per day: add hay or expand"), Auto-cut (plant checklist default trees, cacti, brambles; every 2 hours designates matching plants). Assignment: closest reachable enclosed accepting pen, recomputed on settings/structure change, tame or birth. New Handling job `Rope` (60 work; animal follows one cell behind; releases inside the target pen; triggers: outside any accepting pen or in one that no longer accepts it; handler's area applies; refused and postponed when unreachable). Unpenned livestock: roam (radius 30), alert "Unpenned livestock: N"; after 3 in-game days unpenned, each day an animal 40+ cells from Home has a 12% chance to wander to the nearest edge and leave ("Bessie wandered off"); roped or penned resets the count (hysteresis), a handler roping cancels leaving. Animals tab columns Master (`SetAnimalMaster`), Follow drafted, Area / Pen (read-only pen name, "Roaming", "Being roped"). Migration: painted pen zone becomes a `PenMarker` at the free walkable cell nearest the centroid with defaults; unfenced zones become "Not enclosed"; scenarios build markers.
- Home: `homeAuto` (default on, saved, `SetHomeAuto`, house-icon toggle in lower-right controls; off keeps existing Home); triggers also on new stockpile/growing zones and pen markers (2-cell margin); repair requires building in Home (and pawn's area); Architect Zone "Expand/Clear home area"; roof tools move to Zone; new persistent `roof.AreaBuild` bit keeps roofs rebuilt after collapse until cleared, remove area keeps `NoAuto`.
- Snow and fire: per-tile snow layer and snow-clearing area (job `ClearSnow`, command `DesignateSnowClear`), full fire system (`world/fire`, `Fire{X,Y,Size 1..100,Age}`, growth +4 per 30 ticks, -6 without fuel, spread at size >= 35 with chance flammability x size/100 x 0.06, x1.5 downwind, x0.2 into roofed from outdoors, outdoor rain/snow intensity >= 0.3 loses 12 per step; flammability table grass 0.5, wild plants 0.6 (trees 0.7, duskmoss 0.9), crops 0.6, wooden buildings and bridges 0.7, stone/steel 0, wood/cloth/leather/straw/hay/corpses 0.7-0.9, food 0.3, metal and stone items 0, terrain never burns). Fire game layer: per-chunk active lists (saved); damage per step to buildings (bridge collapse cascade), plants (ash stain), grass, flammable items, pawns (`health.Cause` Burn, `body.Burn` wounds; animals Injury); room fires heat the room, outdoor fires warm 5x5; fire cells cost x20 to path; idle pawns and animals step out; animals flee within 4; colonists flee fire only when caught in it. Firefighting: `work.Firefight` on by default, job `FightFire` (-10 size per 30 ticks, skill-free), eligible inside Home AND area, drafted "Extinguish" order ignores both. Alert "Fire in the colony!" while any fire burns inside Home; `WorldView.Fires []FireView{X,Y,Size}`. Art `effect/fire_{small,medium,large}_0..3`, `overlay/ash_0..2`, icons (extinguish, start_fire dev, pen_marker, snow_clear, home_auto, roof_area), `structure/pen_marker`; sound `amb.fire`, `sfx.lightning_strike`. Fire tests include a 500-cell blaze ticking <= 2 ms and a no-flap test for stepping out of fire.
- Execution: Phase 0 lead contract (commands, domain types, read models, save fields, hook stubs, asset ids); Phase 1 eleven agents (AREA-CORE, AREA-UI, PEN-SIM, PEN-UI, HOME, SNOW, FIRE-SIM, FIRE-JOBS, FIRE-RENDER, ART, ROSTER); Phase 2 lead verification incl. fps probe on a grown colony with a 300-cell fire and deep snow.
- Open: vanilla rows "Weather effects" (`[-]`, PLAY-015 SIM-057 delivered per-tile snow and snow clearing; partial) and "Ignition" (`[-]`, missing weapon-fire sources such as incendiary shots). The snow and ignition parts of the plan follow verbatim.

### Open (verbatim)
#### 4. Per-tile snow and the snow clearing area
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
**Active set:** fires on cells, in per-chunk lists (no global scans); saved.

**Ignition:**
- **Lightning:** during Storm blocks, a seeded sim strike every 1–3 in-game hours on a random outdoor cell. On fuel it ignites; it is shown by the existing flash and thunder (which are now driven by the sim event).
- **Exploding animals:** the existing `burst()` "bursts into flames" hook.
- **Developer action:** "Start fire".

## PLAY016_PLAN
PLAY-016 crops redone in the grass and bush style, plus visible cold death · 2026-10-25 · 738832e1a
- Scope: crops looked like older art (saturated toon palette, dark soil mound under every plant, scaled 1.3-1.6x, one big centred sprite per tile with no jitter, mirroring or growth scaling), unlike PLAY-010 grass and bushes (Blender `plant_base.py` primitives `_limb/_leaf/_ball/_cluster`, muted two-band `pl_*` palette, no outline or shadow, flat normals for small plants, jittered mirrored tufts scaled by growth in `grass_batch.go`). User choices: several small plants per tile composed like grass; mound removed and tilled furrows shown under planted crops; scope = all 11 crops at every stage, berry bush, wild healroot, harvested produce items, and a cold look with the user's rule: below a per-crop temperature the crop is destroyed over a few hours, visibly disappearing. Originality: own shapes, palette, numbers; nothing traced.
- Art direction (new ART spec §9.3 "Crops", same family as §9.1): `pl_crop_*` entries in `materials_plants.yaml` (leaf greens slightly yellower than wild foliage; accents desaturated: corn muted gold, wheat/brewgrain straw and bronze, pumpkin burnt orange, radish dusty rose-red, cotton warm cream, healroot pale blue-white, ashleaf silver-sage, potato ochre, beans sage, lettuce pale jade, berries deep plum-red; none more saturated than the dandelion or fire lily accents). Each sprite is one small plant; flat normals for low crops (lettuce, radish, potato, beans, ashleaf, healroot, sprouts); no outline, no baked shadow. Silhouettes per crop (corn tall stalk with tassel and cob; wheat fine stems with bowing heads; brewgrain heavier bronze heads; cotton shrub with cream bolls; potato leafy mound with tubers peeking; beans twining sprawl with pods; lettuce rosette and dense head; radish leaf fountain with red shoulder; pumpkin sprawling vine with 1-2 fruits; healroot low herb; ashleaf silvery rosette). Stages with unchanged keys: sown (furrow decal and seed specks only), sprout (2-3 tiny leaves), young (half size), mature (no produce), harvestable (produce visible), wilted (drooping, browned, for dying); variants v0..3. `overlay/tilled_0..3` decals: soft furrows, tileable along rows, drawn under crop cells and sown-but-unsprouted cells. Berry bush keeps its body with smaller plum-red clusters; wild healroot in ground-plant style. Produce items restyled to the muted palette with softer rounder forms (items keep the item outline per the art spec); UI icons follow via `overlay.py` ICONS.
- Renderer (`crop_batch.go`): units per tile (corn 2; wheat and brewgrain 4; cotton, potato, beans, healroot, ashleaf 3; lettuce and radish 4; pumpkin 1 sprawl plus fruit at harvest), deterministic from a tile hash, random mirroring, slight row alignment along the zone's long axis, unit scale from growth within the stage (no pop between stages) with a 0.9-1.1 trim, per-unit sway (tall crops more), one batch through a runtime atlas like `grass_batch.go`; LOD: far zoom one unit per tile, very far a tinted mark. Budget: a 500-crop field <= 1 ms of draw at default zoom. Inspector and hover unchanged (one plant entity per cell).
- Cold/heat damage over hours: `Plant.Damage uint8` (0..100, additive save); below the kill temperature (crops `FrostKill`, wild `ColdKill`, heat `HeatKill`) damage grows 25% per in-game hour at the threshold plus 5% per degree beyond, capped at 60%/h (a crop just below its threshold dies in about 4 hours, a hard freeze in under 2). Hysteresis: damage stops only when air is 2 C back on the safe side; below 60% damage heals at 10%/h; from 60% the plant keeps dying even if it warms; a no-flap test covers temperature hovering at the threshold. Game: the staggered per-chunk frost pass in `plant_growth.go` adds damage instead of deleting; at 100% the plant is removed with the batched notice ("The cold killed 14 corn plants"); a once-per-day message when frost first hurts a species ("Frost is damaging the corn"); harvesting a damaged crop yields `(100 - Damage)%`. Read model `PlantView.Damage`, `PlantView.DamageKind` (cold or heat). Visual: 0-49% normal sprite with growing rime tint (cold) or scorch tint (heat); 50-100% wilted sprite, units shrinking toward 60% and fading over the last 30%, units dropping out one by one; hover/inspector "Freezing: 40% damaged" / "Scorching". Applies to wild plants and trees (ColdKill/HeatKill) and the berry bush, not to grass (snow and frost pause logic unchanged).
- Execution: Phase 0 lead contract (§9.3 text, palette entries, `Plant.Damage` and view/save fields, asset ids, `crop_batch.go` stub, `plant.py` crop section split into `crops_grain.py` / `crops_leafy.py`); Phase 1 five agents (ART-GRAINS, ART-LEAFY (opus), ART-ITEMS, RENDER-CROPS, PLANT-DAMAGE (opus)); Phase 2 lead visual review, `make art-check` (no outlines on plants, no orphans: removed mound and legacy sprites deleted in the same change), determinism, fps probe on a grown colony. Goldens: winter scenario goldens may move (plants die over hours instead of instantly), re-pin with a field-by-field note; `TestTemperateGenerationUnchanged` and map goldens do not move.
- Limits: no open checklist row names this plan, so there is no verbatim Open part.

## PLAY017_PLAN
PLAY-017 plan: sowing, hauling, outfits, animals, plants and light, chase warnings, UI-scale fixes, visual tests · 2026-10-25 · c7d739ac6
- Changed (twelve playtest requests, each with a cause found in code):
  - Growers clear a wrong plant in a zone cell before sowing (`assignZoneClear`, existing `job.Cut`); ripe wrong crops are harvested, not cut; the right crop is never cut.
  - Zone labels pinned to the zone and scaled with it (clamp(0.45 x min zone side, 11, 40) logical px, centred on the zone cell nearest the box centre).
  - `Gameplay.RunInBackground` (default off): off pauses on focus loss ("Paused (window in background)") and keeps a manual pause on refocus; on uses `SetRunnableOnUnfocused` plus vsync off and a ~20 fps cap while unfocused.
  - Haulers carry only the colony's goods: `item.Stack.Unclaimed` and `HaulMarked`, commands `DesignateHaul` and `SetHaulMark`, `SaveVersion` 3; "Haul things" tool, `overlay/designation_haul`, item-inspector mark button.
  - Portraits and world labels draw in place through `uiDrawImage`, `worldText` and `worldIcon` (bar, inspector, tame names, chat icon, fight marker, drafted icon).
  - Colonists dress at once into the best Body x Head outfit for the day's mean +-10 C; a "Wear X" order marks the slot forced.
  - Tame animals eat the nearest allowed food (plants, loose or stockpiled raw food, carcasses); tame predators hunt the nearest wild huntable.
  - Double-click a colonist in the bar centres the camera (18-frame window).
  - Plants grow for max(sunTicks, lampTicks) per chunk window (lamp light >= `plant.StrongGrowLight` 0.5, whole window); roofed unlit plants and grass stop; ripe zone crops are auto-marked (`wantsHarvest = Harvestable && Designated`); plant panel shows `GrowState` and `RipeIn`.
  - `noteChase` posts a Danger message with Focus when an animal starts a chase (hungry predator, revenge, charge, fight-back, woken vault sleeper); red message, alert names the colonist, chase line from `AnimalView.TargetID`, camera centres when off-screen; pause-on-threat reuses `watchThreats`.
  - Visual regression suite: every draw helper traces (kind, text, physical rect, pass); `TestUIDrawsThroughCanvas` becomes a per-function `go/ast` guard (exemptions keyed `file:func` with a reason, stale ones fail); frame-invariance harness at UI scale 1/1.5/2, device scale 1/2 (pan, scale, anchors, panels).
- Decisions:
  - User decision: outputs of work the player ordered are colony items; only Haul work moves items; things the colony never touched wait for a Haul mark. Zero value of the flags is colony, so old saves and test spawns are unchanged.
  - Hysteresis: outfit change needs >= 1 C gain and a 2 h cooldown (skipped while Freezing or Sweltering); chase re-warn needs a new target or the chase ended plus 1 in-game hour (`ChaseWarnedAt`/`ChaseWarnedTarget` saved); animal feeding keeps the 4000/9500 thresholds, `NoFoodRetry`/`HuntRetry`.
  - Unroofed outdoor crops grow exactly as before (no balance change); grass outdoors regrows at 24/14 of the old rate in the sun window so the day total is unchanged; campfires light only while burning.
  - Needs, forced orders and drafted colonists are never interrupted by dressing; pets and penned/colony animals are never hunted.
  - Agents never re-pin goldens; the lead re-pins once (expected moves: wild corpses not hauled, grass timing and roofed plants, earlier dressing).
  - Visual tests: under ~20 s with `-race`, no pixel or window tests.
- Limits: spacing and row heights remain per-file constants (no central token set).
- Open: the UI-scale row stays `[-]`; the haul and plant-panel rows below still own open parts.
- Execution: lead contract first, then 11 parallel agents (GROW, PLANTS-LIGHT, VISUAL-FIX, APP-SHELL, HAUL-SIM, HAUL-UI, HAUL-ART, APPAREL, ANIMALS (opus), CHASE-UI, VISUAL-TESTS) with disjoint files.

### Open (verbatim)
Haul loose item to storage (vanilla checklist) and the plant panel line of Selected entity header (UI checklist) are the rows still open for this plan; the owning plan parts follow.

#### 5. Haulers carry only the colony's things (game + UI + art)
**Contract:**
- `item.Stack.Unclaimed` and `HaulMarked`. The zero value is colony, so old saves and the 176 test spawns are unchanged.
- `CanMerge` requires both to be equal.
- New commands `DesignateHaul` (rect) and `SetHaulMark` (one stack).
- `ItemView` gains both flags.
- `SaveVersion` 3.

**Rule:** only `haulToStockpile` skips `Unclaimed && !HaulMarked`. Every other job still uses unclaimed items; forbidden still blocks everything.

**Spawned unclaimed** (`spawnUnclaimed`, new `haul_claim.go`):
- mapgen ruin loot and chunks;
- vault loot and casket weapons;
- raider and visitor gear;
- animal corpses not killed by the colony (a predator, raider, fire, old age, or a wild animal's starvation);
- eggs from wild mothers.

**Everything else is colony:** start kit, pods, cargo, bills, drying, harvests, cut wood, mining, hunting, tame animal products, refunds, colonists' gear.

**Claiming:**
- Anything a colonist picks up becomes colony (`splitLoad`, the `doHaul` pickup, bill and refuel carries), so leftovers get stored.
- Storing clears both flags.
- `splitLoad`, `dropStackOf` and `clearItemsFrom` keep the flags; `cancelRect` clears marks.

**Old saves:** chunks and animal corpses lying outside stockpiles become unclaimed.

**On click (plant inspector):**
- the growth bar caption gives the state: "growing", "resting (night)", "too dark: needs sun or a lamp nearby", "too cold" or "ripe";
- a line "Ripe in about N days", estimated from the current conditions;
- `PlantView` gains `GrowState` and `RipeIn`, recomputed on every build like `Wilted`.

## PLAY018_PLAN
PLAY-018 plan: colony HUD re-layout, command buttons, charcoal palette, letters, Learning Helper · 2026-10-25 · b70a1a584
- Changed: the colony HUD follows a familiar spatial arrangement on our own grid (map full screen, HUD floats over it; `computeHUDLayout(sw, sh, hudState)` is a pure function computed once per Update):
  - bottom: 11 tabs (Architect, Work, Schedule, Assign, Animals, Wildlife, Research with progress fill, World, History, Factions, Menu); lower-left inspect pane with tabs opening upward; 64x64 command-button grid; windows bottom-left on top of the tabs.
  - right column: overlay toggles, 4 speed buttons, date/time/season, weather/wind, hovered-cell climate, condition lines, minimap, letter stack (max 8), Learning Helper, alerts by severity.
  - top: colonist bar (top-centre, 52x64 slots, scale steps 1/0.75/0.5), resource list top-left (stored goods only, categorised and collapsible), messages, hint and toasts.
  - nothing selected: the pane shows the hovered cell (terrain, fertility, roof, temperature, light, room, objects).
  - every action is a `cmdButton` (legacy ids kept so click-by-id tests hold); multi-select merges by key; hotkeys follow declaration order (R draft and F forbid stay); options via left/right click and float menu.
  - Learning Helper with ten original lessons; letters stack with kinds threat/caution/info/good, saved (cap 40), typed `command.DismissLetter`.
  - new read models: `WorldView.Stored []StoredResourceView`, `WorldView.Letters []LetterView`, `MessageView.Kind`, `Simulation.TemperatureAt`; new `item.Category`.
  - settings v4: `ShowLearningHelper`, `LearnCollapsed`, `LearnedLessons`, `CategorizedResources`, `ShowColonistBar`.
- Decisions (user, 2026-10-25): arrangement only, no RimWorld decompiled numbers or colours; new original cool charcoal palette for every screen, original warm brown/gold for the Learning Helper; scope is HUD, inspect pane plus command grid, Architect, windows, letters and alerts; add a Learning Helper. MAIN_UI §9/§10 redrawn and the decision recorded there; ART §17/§33 palette note.
- Decisions: art agents run on opus and load the art-assets skill (user, 2026-10-25; recorded in PARALLEL_WORK §6); same 32 palette token names with new values, plus `ui.letter_*`, `ui.alert_*_bg`, `ui.tutor_*`, `ui.mood_*`.
- Bugs fixed on the way: `hud.plan` and `hud.rooms` shared a rect; `colonyChromeHit` missed animal/item/plant panels, chips and hover panel; gizmo hotkeys followed screen position.
- Tests: pinned numbers became rects from `a.hud()` with no assertion dropped; no-overlap tests at 1280x720, 1920x1080, 1113x626, UI scales 1/1.5/2, 6 and 30 colonists, 13 and 27 commands; Escape order, disabled reasons and canvas guard stay green unchanged.
- Risks: input precedence of the new HUD right-click, hotkey letters change, tooltips must flip above the grid, full-screen viewport affects camera clamping and fps (~17% more rows), letters save growth, leftover warm colours, captures and README media stale.
- Execution: lead contract first (`hud_layout.go`, split of `hud.go` into owned files, command-button and pane types, hooks, read models, tokens, settings), then 14 parallel agents (PALETTE, ICONS opus; GAME-RM, CORE-PANE, PAWN-INSPECT, THING-INSPECT, BUILD-INSPECT, ARCHITECT, HUD-TOP, HUD-RIGHT, GLOBAL, WINDOWS, LEARN, VISUAL-TESTS); lead keeps `ui.yaml`, `cmd/**`, specs, checklists.
- Verification target: fps vs 119 avg / 94 1%-low, `BenchmarkColonyHUD`, captures at UI scale 1/1.5/2.
- Limits: no `hud.go` frozen-dispatcher growth (it keeps only `drawColonyHUD`, `colonyButtons`, `colonyChromeHit`); `inspectorRect`, `hudTopH`, `hudBarY` and `gizmo.go` were to be deleted once unused.
- Open: 17 UI/vanilla rows remain `[-]` (UI scale, semantic colors, toast, game state indicator, selected entity header, location, context actions, multi-select, status indicators, build option cards, overlay registry, letter window, expandable details, lesson cards, lesson history, event letters, alerts); the spec below is still live.

### Open (verbatim)
The whole layout and contract still own open rows (checklist text for each gap is in the UI and vanilla checklists).

#### User decisions
**User decisions (2026-10-25):**
1. **Arrangement only.** Copy RimWorld's spatial arrangement and interaction behaviour, but size everything on our own grid with our own fonts and icons. No RimWorld decompiled numbers or colours; project rules (AGENTS.md, MASTER_SPEC L10, ART §17/§33) stay intact.
2. **Palette.** A new original cool charcoal palette for every screen. The Learning Helper gets an original warm brown/gold.
3. **Scope.** All of: the HUD arrangement, the inspect pane plus command-button grid, the Architect re-layout, windows, letters and alerts.
4. **Learning Helper.** Add one.

#### Target layout (logical px, our sizes, all multiples of 4; margin 8, gap 4)
The map fills the whole screen (`colonyViewport` = full screen) and the HUD floats over it. `computeHUDLayout(sw, sh, hudState) hudLayout` is a pure function, recomputed once per Update (`a.hud()`).

**Bottom of the screen**

| Region | Rule |
|---|---|
| Tabs | Full width, 28 tall at `sh-28`. 11 equal tabs: Architect, Work, Schedule, Assign, Animals, Wildlife (new; was F5), Research (shows a progress fill), World, History, Factions, Menu. |
| Pane (inspect) | `{0, Tabs.Y-188, 400, 188}` |
| PaneTabs | A 24 px strip above the Pane; tab widths are measured. |
| Expanded | Opens upward from PaneTabs. Width `clamp(want, 400, 560)`; height capped below the colonist bar. |
| Command grid | 64×64 cells on a stride of 68. Starts at `Pane.right+8` (or `ArchCats.right+8`) and ends at `Global.X-8` (9 columns at 1280). Fills the bottom row left to right, then wraps upward, up to 4 rows. Only occupied cells block the map. |

**Right column** (x = `sw-240`, 232 wide)

| Region | Rule |
|---|---|
| Global controls | Built bottom-up from `Tabs.Y-8`: overlay toggle icon rows (24 px, 8 per row); 4 speed buttons (52×24: pause, normal, fast, superfast); date, then time + season; weather + wind; "Indoors/Outdoors N °C" for the hovered cell; one line per active condition (replaces the status chips). |
| Minimap | 180 square, just above Global, only on big maps. |
| Letters | 28 px rows stacked upward from the minimap or Global; at most 8, then "+N older". |
| Learning Helper | Top-right: `{X, 8, 232, 28}` collapsed, up to 200 tall expanded. |
| Alerts | 24 px rows from below the Learning Helper down to the letters, ordered by severity, then "+N more". |

**Top of the screen**

| Region | Rule |
|---|---|
| Colonist bar | Top-centre at y 8. Width cap `sw − 2·248`, so it never reaches the resources list or the right column. 52×64 slots. Scale steps 1 / 0.75 / 0.5 allow 1 / 2 / 3 rows, then a "+N" slot; names are hidden below scale 1 (tooltip instead). Six colonists fit one row; 30 colonists wrap to 18+12 at 0.75. |
| Resources | Top-left `{8, 8, 168, …}`, 20 px rows of icon + count. Counts stored goods only (stockpile cells). Categorised mode is collapsible. The list scrolls above the highest lower-left element and hides when less than 40 px is left. |
| Messages | Right-aligned to the right column, under the bar; at most 5 rows of 22 px, each at most 480 wide. |
| Hint and toasts | Just above the grid or pane (replaces `drawBottomHint` and the toast position). |

**Architect**
- ArchCats `{0, Tabs.Y-140, 212, 140}`: 2 columns of 100×28.
- ArchDesc above it, 320 wide: tool name, cost, description, stock, crop picker or material choice.
- The tools are command buttons in the grid.

**Windows**
- Work, Schedule, Research, Animals, Wildlife, Assign, History, Factions and World sit bottom-left on top of the tabs: `{0, Tabs.Y-h, min(wantW, Right.X-8), min(wantH, Tabs.Y-Bar.bottom-16)}`.
- They never cover the bar, alerts or global controls.
- Menu stays the pause overlay.

**Nothing selected:** the Pane shows the hovered cell's info (terrain, fertility, roof, temperature, light, room, objects), replacing the bottom-right hover panel.

#### Contract (lead, alone, first)
1. **`hud_layout.go`.** `hudLayout` holds every rect above plus `BarSlots`/`GridCells []rect`, `BarScale` and `GridCols`. `hudState` holds the counts and flags the layout needs. `colonyChromeHit` becomes "the point is inside a region the HUD drew" (the minimap keeps its own click path). `colonyViewport` returns the full screen. `inspectorRect`, `hudTopH` and `hudBarY` stay as deprecated aliases until the last caller is gone, then get deleted.
2. **Split `hud.go` into owned files**, moving code verbatim and stubbing the rest: `hud_resources.go`, `hud_colonistbar.go`, `hud_right.go`, `hud_global.go`, `hud_cellinfo.go`, `hud_inspect.go`, `hud_commands.go`, `hud_tabs.go`, `hud_windows.go`, `learn_ui.go`, `cmds_pawn.go`, `cmds_thing.go`, `cmds_build.go`, `cmds_arch.go`. `hud.go` keeps only the frozen dispatchers: `drawColonyHUD`, `colonyButtons`, `colonyChromeHit`. Lead-frozen files: `hud.go`, `app.go`, `colony_ui.go`, `renderer.go`, `app_test.go`, `command.go`, config.
3. **Command buttons (`hud_commands.go`).**
   - Types:
     - `cmdButton{id, key, label, icon, desc, reason; hotkey; disabled, toggle, on, mixed; run func(*App); cmds []command.Command; options func(*App) []cmdOption}`
     - `cmdOption{label, reason, disabled, cmd, run}`
   - **Ids keep their legacy names** (`pawn.draft`, `item.forbid`, `bldg.move`, …) and go into `colonyButtons`. Every click-by-id test keeps working. On click: `run`, else enqueue `cmds`, else `applyClick(id)`.
   - **Producers:** `selectionCommands()` dispatches to per-kind producers: `pawnCommands`, `animalCommands`, `itemCommands`, `plantCommands`, `buildingCommands`, `frameCommands`, `archToolCommands`.
   - **Multi-select merge** by `key`, in the order keys were first declared:
     - disabled only if all members are, with the reason "(k of n)";
     - `on` means all members are on; `mixed` means some are;
     - a click on a mixed toggle switches everyone on.
   - **Hotkeys** follow declaration order (`input.Gizmos[i]`, up to 12); dedicated keys stay (R draft, F forbid). `gizmo.go` is deleted at the end.
   - **Options:** left-click runs the first enabled option; right-click or the corner arrow opens the float menu (`orders.go`, plus `pick func(i)`). Options cover:
     - the per-colonist "now" chips;
     - Equip/Wear/Eat/Haul;
     - Equip nearby;
     - bed owner and animal master;
     - build material.
   - **Tooltip** (kit `uiTip`): label, description, key, and the disabled reason in danger colour.
4. **Inspect pane (`hud_inspect.go`).**
   - Types:
     - `paneSpec{title, subtitle; bars []paneBar; chips []paneChip (e.g. a clickable allowed-area chip); lines []string; tabs []paneTab; close}`
     - `paneTab{id, label, reason, disabled, wantW, wantH, draw(img, rect) /*legacy*/, declare(rect) /*kit*/}`
   - `selectionPane()` dispatches to per-kind producers.
   - `a.paneRect()` and `a.expandedRect()` replace `inspectorRect`, `itemRect`, `plantRect`, `frameRect`, `buildingRect`, `animalPanel` and `groupRect`.
   - Tab ids stay `inspector.<tab>`. Clicking the open tab again collapses it.
5. **Hooks.**
   - `colony_ui.go`/`app.go`: `resourcesUI`, `colonistBarUI`, `rightColumnUI`, `globalUI`, `paneUI`, `commandGridUI`, `learnUI` and `mainWindowUI`.
   - Click prefixes `letter.`, `speed.`, `learn.`, `res.`, `cmd.`, `win.`, `tab.wildlife`.
   - `hudRightClick(mx, my)` runs before map right-click and only consumes clicks over the HUD.
   - `Renderer.onEnqueue` (Learning Helper hook).
   - `ui_modal.go` gets `anchor`/`modeless`.
   - The AST guard's `hudFiles` gains `hud_*.go`, `cmds_*.go` and `learn_ui.go`.
6. **Read models (game, additive).**
   - **`WorldView.Stored []StoredResourceView{Kind, Label, Category, Qty}`.** Counts uncarried, unfogged stacks on stockpile cells, inside `World()`'s existing item loop (`zoneOf` lookup, O(1)). Categories come from a new `item.Category`. `w.Food/Wood/…` keep counting all loose stacks, so alerts and Architect stock lines are unchanged.
   - **`WorldView.Letters []LetterView{ID, Tick, Kind (threat/caution/info/good), Title, Text, X, Y, Focus, Intro}`.**
     - Saved additively and capped at 40. Old saves' letter messages are migrated on load.
     - `MessageView` gains `Kind`; post sites set it.
     - Dismissal goes through a new typed `command.DismissLetter{Target}` (letters live in the save); "read" is UI-only.
     - Pause-on-threat keeps reading Danger messages.
   - **`Simulation.TemperatureAt(x, y)`** joins the UI interface (and `stubSim`).
   - Alert explanations stay UI-side.
7. **Palette tokens.**
   - Same 32 names in `ui.yaml`/`widgets.go`, with new values from PALETTE (lead writes `ui.yaml`).
   - New tokens:
     - `ui.letter_threat|caution|info|good`
     - `ui.alert_danger_bg|alert_warn_bg`
     - `ui.tutor_panel|border|heading|text`
     - `ui.mood_good|mid|bad`
8. **Settings** (`config`).
   - `Gameplay.ShowLearningHelper` (default true), `LearnCollapsed`, `LearnedLessons []string` (deduped and capped in `Normalize`), `CategorizedResources`, `ShowColonistBar` (default true).
   - `SettingsVersion` 4; older files decode over the defaults.
9. **Spec.**
   - MAIN_UI §9/§10 rewritten: new diagram, commands, letters, Learning Helper, and the recorded decision.
   - ART §17/§33 palette note.


## PLAY019_PLAN
PLAY-019 plan: RAM-aware parallel work, hardware-aware graphics defaults, crops on their own tile · 2026-10-25 · 02824e5e4
- Changed:
  - `internal/platform/sysinfo` (no cgo; macOS sysctl, Linux sysinfo + `/proc/meminfo`, Windows `GlobalMemoryStatusEx`) returns total/available RAM and cores; `golang.org/x/sys` becomes a direct dependency.
  - `cmd/rambudget` / `make agent-budget`: budget = available - 4 GB reserve; max heavy agents = budget / measured heavy peak. Rule in PARALLEL_WORK §1/§5 and skill `parallel-work` step 0: run it before every wave, stagger heavy agents, `GOFLAGS=-p=2` on test runs.
  - Memory diet: lazy art decoding with background prewarm, CPU pixel copies freed after GPU upload (alpha boxes kept), LRU-bound tint and lit-pair caches.
  - Hardware tiers at first launch only (High >= 12 GB and >= 6 cores = today's look; Medium >= 8 GB; Low below), settings v5: `Graphics.Quality` (high/medium/low/custom), `Shadows`, `Lighting`, `Water`, `GrassDetail`, `Detected`, `Chosen []string`; v4 -> v5 migration keeps values that differ from old defaults (the player's choice). Options > Graphics: Quality row, per-knob rows with cost tooltips, "Detect again".
  - Low-RAM notice on the main menu and boot screen (total below game peak x 1.5 + 2 GB), dismissible and remembered for that RAM amount; map sizes whose estimate exceeds 70% of total RAM warn in the size picker and site panel (replaces the fixed "needs a capable computer" label at 350+ and the 400 "Large map" warning).
  - Crops stay on their tile: anchor `fy := (Origin[1] + fh*artTilePx/2)/kpx - box.Min.Y` (root at tile centre), containment moves a unit south (root <= 0.95 of tile) then scales it down; art unchanged.
  - `scripts/package_release.sh` now ships `assets/generated/atlas` (release builds could not load the pawn atlas).
- Decisions (user): low RAM warns only, never blocks; detection at first launch only, no runtime quality changes; crops stay on their own tile.
- Limits: new graphics knobs read live through `syncRenderer`, one test per knob.
- Open: none beyond the checklist rows (no open rows for PLAY-019).

## PLAY020_PLAN
PLAY-020 plan: bubbles, zone fill, lamp-lit rooms, sowing, bed rest, medical beds, zone management · 2026-10-25 · a8adb8002
- Changed:
  - Chat bubbles lose the white box: only the badge icon via `worldIcon` (fallback `uiPanel` disc with three dots); `TestChatBubbleHasNoWhiteBox`.
  - Growing and tree zone fill skips cells holding the zone's own plants (`zonePlantCells`, `r.zonePlants`), at every zoom.
  - Lamp-lit rooms: menu scene uses a roofed room with a Blender point light; in game, lamps are point lights with falloff (`(1-(d/r)^2)^2`, radius from `light_radius`, default 6), direction (sprites shade toward the dominant lamp), clipped to their room or, outdoors, to line of sight; creature lamp shadows follow the tile's light, not global daylight. Fps within 1 ms p99.
  - Sowing race fixed: `startJob` claims the tile for `job.Sow` (`g.reserve.TryTile`); forced `OrderSow` on a held tile refuses with `ReasonBusyTarget`.
  - Bed rest: injured, sick, bleeding or surgery-waiting colonists rest by default (per-pawn `SetPawnBedRest`, "Rest when hurt"); medical bed is a per-bed flag (`SetBedMedical`, never spreads; new hospital beds start medical, one-time migration); rescue ranks medical beds first.
  - Zone management from the map: zone selection kind, pane (name, size, plant, growing state, plants sown/ripe/average growth, stockpile filters), commands Plant, Allow sowing, Allow cutting, Expand, Shrink, Delete, Hide, Rename (`SetZoneCrop/Sow/Cut/Hidden`, `RenameZone`, `ExpandZone`, `RemoveZoneCells`, `DeleteZone`; fields `NoSow`, `NoCut`, `Hidden`, `Name`, so old saves and `StateHash` hold); zones stay contiguous and never overlap.
- Decisions (user, 2026-10-25): lighting "All, plus outdoor lamps"; injured or in-pain colonists stay in bed by default; medical is per-bed and never spreads; zone management "the way RimWorld does it" (behaviour from the wiki, own text and art).
- Hysteresis: bed rest starts at Injury >= 40 (pain), untended wound, sick or surgery wait; stops at Injury <= 15 with no untended wound; 2 h minimum rest, 6 h retry cooldown with no free bed; hungry patient eats then returns; no-flap: at most 3 job-kind changes in 300 ticks.
- Zone rules: NoSow stops only sowing (in-flight sows cancelled, tree zones stop replanting); NoCut stops wrong-plant clearing and zone tree chopping, harvest of the zone's own ripe crop continues; Shrink splits into contiguous pieces, largest keeps the id; names trimmed to 24 chars.
- Lead-frozen UI files (`app.go`, `hud.go`, `hud_inspect.go`, `hud_commands.go`) got stub hooks in `zone_select.go` (`handleZoneTool`, `applyZoneClick`, `escapeZone`, `drawZoneSelection`).
- Limits: double-click selecting all similar zones in view left for later; goldens move only where injured pawns now rest.
- Open: patient/work-type/crop-selector/day-night-light rows below.

### Open (verbatim)
Parts owning open rows: Crop selector (Plant list on a zone), Bed/patient reservation, Work types (Rest when hurt), Day/night light (lamps).

#### Lamp-lit rooms, in-game point lamps (LIGHT)
##### 3b. In-game point lamps (LIGHT)
- **Light grid** (`lightGrid.base`, `light.go:421`): per-tile lamp light is the strongest lamp falloff, radius from metadata `light_radius` (default 6), with a smooth curve such as `(1-(d/r)^2)^2`.
  - A room lamp lights only its room; walls keep the brightest-neighbour rule.
  - An outdoor lamp lights what it can see in its radius: not through walls, closed doors or rock, and not into enclosed rooms.
  - Always on. Rebuilt lazily on structure or lamp change, with no per-frame global scan. `Room.Lit` is unchanged.
- **Direction:** sprites shade toward the dominant lamp.
  - The encoding is LIGHT's design (the four custom vertex floats are taken).
  - Builders that only pass `lightTile` values keep working unchanged.
  - Flat directly under a lamp; half-Lambert or a kept ambient share.
  - `litFactor` and `flatLitScale` stay the shader's Go twins; Lighting off gives falloff without direction.
- **Shadows:**
  - Creature lamp shadows depend on the creature's tile (roofed or lamp-lit), not the global daylight (`shadow_cast.go:217`).
  - Their reach follows the grid.
  - Lit outdoor lamps join `pointLights` at night.
- **Performance:** fps probe before/after (250 map, grown 500); draw p99 within 1 ms.
- **Tests first:**
  - near is brighter than far in a room;
  - no light through a wall;
  - an outdoor lamp lights open ground but not behind a wall;
  - west-facing normals east of a lamp are brighter;
  - a roofed room at noon has no sun term and keeps lamp shadows;
  - an outdoor lamp casts creature shadows at night;
  - the Go twin matches the shader.
  - Re-express (never weaken) `TestIndoorAmbientWhenRoofed`, `TestLampAddsWarmth`, `TestLitRoomWallCornersGlow` and `TestPointLightsFromCampfiresAndLitLamps`.

#### 4. Bed rest for injured colonists (MEDSIM; widen `patientRest`, `infection.go:69`)
- **Start** when any of these holds and `RestWhenHurt` is on (default):
  - in pain (`Injury >= painInjury` 40);
  - an untended bleeding or open wound;
  - sick (`Infection >= Sick`);
  - waiting for surgery.

  It no longer needs another doctor: self-tend happens in bed.
- **Stop** at Injury ≤ 15, no untended wound, and not sick. A 2 h minimum rest; with no free bed, a 6 h retry cooldown (the `surgeryGaveUp` pattern).
- **Interruptions:**
  - A hungry patient ends rest, eats and returns (this fixes the loop at `tick.go:470`).
  - Fire, flight, mental breaks, social duty, warmth, drafting and queued or forced orders keep their `phaseAI` precedence. Rest resumes after a forced job.
- **Bed choice:**
  - the nearest free medical bed the pawn may use (the prisoner flag must match), then its own bed, then the nearest free non-medical bed;
  - medical claims are temporary;
  - `freeBedFor` (rescue) ranks medical beds first, then the hospital kind, then distance.
- **Per pawn:** `SetPawnBedRest` turns "Rest when hurt" off or on (saved; default on).
- **State:** new maps are saved as additive `saveState` fields and cleaned in `world_stash.go`, with deterministic iteration. `PawnView.BedRest` and `RestWhenHurt` are filled, and `pv.Job` reads "Resting (injured)" (the `fillFleeView` pattern).
- **Tests:**
  - no-flap: at most 3 job-kind changes in 300 ticks around the thresholds, and the state survives save/load (`sleep_test.go` helpers);
  - goes to bed and stays until healed;
  - a hungry patient eats then returns;
  - no bed puts the cooldown in effect;
  - toggled off, the pawn keeps working;
  - drafting overrides;
  - prefers and then releases a medical bed.
  - Goldens move only where injured pawns now rest, re-pinned with a reason.

#### 5. Medical beds (MEDSIM sim, MEDUI UI)
- **Sim:**
  - `SetBedMedical` sets `Medical` on that one sleeping bed only, never on its neighbours.
  - Turning it on releases its owners. It is independent of "For prisoners".
  - New `HospitalBed`s start medical, with a one-time migration for older saves.
  - `g.isMedicalBed(b)` replaces the "for patients" kind checks at `medical.go:77`, `jobindex.go:265` and `area_dest.go:41-45`. Quality bonuses stay kind-based (`items_ng1.go:86-93`, `surgery.go:375`/`:549`).
  - `assignSleep` (`tick.go:755`) and `partnerBed` (`beds.go:74`) skip medical beds for anyone not resting.
  - `cmdAssignBedOwner` and `OrderSleep` refuse them for healthy pawns, with a reason.
  - `BuildingView.Medical` is filled.
- **UI (MEDUI):**
  - a "Medical" toggle beside "Prisoners" on any sleeping bed: icon `ui_icon/hospital_bed`, desc "Only the sick and injured rest here.";
  - the bed line "Medical bed";
  - the Owner button disabled with "Medical beds have no owner";
  - the pawn toggle "Rest when hurt" (`cmd.pawn.bedrest`, icon `bed_rest`) and the status line "Resting (injured)".

- **Pane** (`plantPane` is the model):
  - Title: the zone's name, or a default "Growing zone N" / "Stockpile N" / "Pen N" from its id.
  - Lines:
    - size in tiles;
    - for grow zones: "Plant: Potato"; growing now or too cold or too hot, with the zone's temperature (`TemperatureAt`) against the crop's range; plants: sown of total, ripe count, average growth;
    - "Sowing off" or "Cutting off" when set;
    - stockpiles: their filters.
- **Commands** (`cmd.zone.*`, with `Run`, in RimWorld's order):
  1. **Plant: Potato:** grow zones; opens the crop list (`openPick`, the architect crop picker's entries and research locks) and enqueues `SetZoneCrop`.
  2. **Allow sowing:** grow zones; Toggle, `On = !NoSow`; `SetZoneSow`.
  3. **Allow cutting:** grow zones; Toggle; `SetZoneCut`.
  4. **Expand:** a drag-rectangle tool for this zone, with the zone-painting ghost (green or red per cell, `drawZoneGhost`, `CanZoneAt`); enqueues `ExpandZone`.
  5. **Shrink:** a drag-rectangle tool that tints in red only this zone's cells under the drag; a single click removes one cell; enqueues `RemoveZoneCells` per rect.
  6. **Delete:** immediate; `DeleteZone`.
  7. **Hide:** Toggle; `SetZoneHidden`.
  8. **Rename:** a text-entry modal with the existing field widgets; `RenameZone`.

## PLAY021_PLAN
PLAY-021 plan: electricity, RimWorld-style · 2026-10-25 · 3337ac537
- Changed (behaviour from the wiki, numbers our own):
  - Power grid: conduits (new `LayerConduit`, under walls and benches), hidden conduits, switch (flicked by a colonist, `job.Flick`), devices join the nearest transmitter within 3 tiles (`ConnectRange`; ties by distance, y, x).
  - Generators: wood-fired 800 W, solar up to 900 W (daylight x unroofed share), wind 250 W per wind unit capped 1100, geothermal 1800 W on a steam vent (vent moves to `LayerFoundation`); battery 1200 Wh, 75% efficiency, 8 W self-discharge.
  - Consumers: lamp 50 W (dark without power), free torch lamp (burns 8 logs, rain can put it out), heater 180/20 W, in-wall cooler 220/20 W with thermostat (target 21 C, heats below target-1, idles above +0.5), electric stove/tailoring/research benches 250/150/200 W, 40% faster, "No power" when dark.
  - `phasePower` (before `phaseClimate`, every 50 ticks or when dirty) integrates exact time, writes nothing without power devices; brownout sheds by class (lamps, benches, coolers, heaters), newest first, off at least 1 h, back with 10% headroom.
  - Events (own `streamPower`, `incidentFits` false without power): solar flare (3-8 h, 12-day cooldown, from day 12) and short circuit (8-day cooldown, from day 10, needs >= 100 Wh, drains batteries, removes a conduit, starts a fire).
  - Research: Electricity 3000 first, then Batteries 2500, Solar 4000, Wind 3500, Geothermal 5000.
  - Saves: `Power *powerSave` (nil when empty); `SaveVersion` 5 migration turns existing lamps into torch lamps. Nets and outputs derived, not saved.
  - UI: Power category and Temperature category in the Architect, `Power grid` overlay toggle (auto-shown while placing; V = power toggle), placement link ghost, building-pane power lines/bars/commands (`SetPowerWanted`, `SetThermostat`, `OrderFlick`), alerts Brownout/Not connected/Out of fuel, letters and a chip for flare and short circuit. Sound is code-made via `cmd/audiogen`, no paid generation.
- Decisions (user, 2026-10-25): generators wood-fired, solar, wind, geothermal; lamps need power plus a wood torch lamp before electricity; extras switches, toggles, solar flare, short circuit, heater/cooler, electric benches; RimWorld-like research tree; numbers our own.
- Invariants: units integer W, stored energy int64 W-ticks (2500 = 1 Wh); light sources come from one helper `lightSources(w)` gated on `Glow`/`GlowRadius`; fuel table moved to `fuel.go` in the same iteration order so goldens hold; no-flap tests on shedding (+-5% ripple for two days, every off span >= minimum) and thermostats.
- Limits: grown golden (lamps) needs a re-pin with a note; per-kind lists cached per structVer to avoid full-list scans; sequencing: starts after PLAY-020.
- Execution: lead contract first, 12 agents in 3 waves (POWERDOM, FUELGEN, INFRA, ART; POWERSIM, CONSUMERS, EVENTS; POWERUI, POWERPANE, RENDER, SOUND).
- Open: power overlay does not draw production vs consumption; semantic colours for health/hostile; heat-source panes (vent, passive cooler, TEMP-5); solar weather dimming belongs to PLAY-022.

### Open (verbatim)
Parts owning open rows: Network production vs consumption (inspector and overlay), Heat sources (heater, cooler, thermostat), Solar day/weather output, Semantic colors (power tokens).

#### Model table (solar, heater, cooler rows)
#### Model (our numbers; integer W; stored energy in W·ticks, shown as Wh)
| Thing | Rule |

| Solar panel | Up to 900 W, following daylight (`lighting.Outdoor`). Scaled by the share of its footprint not roofed; 0 at night. |

| Heater | 180 W while heating, 20 W idle. Thermostat: target 21 °C, heats below target − 1 °C, idles above target + 0.5 °C. |
| Cooler | Sits in a wall gap like a door, facing its cold side. 220 W cooling, 20 W idle. Cools the cold-side room and warms the hot side (outdoors: nothing). Same thermostat hysteresis. |

- **Power overlay:**
  - A global toggle `Power grid` (`hud_global.go`; a second row when there are more than 8 toggles).
  - Auto-shown while placing a power tool (`syncPowerForce` beside `syncRoofForce`, `hud_global.go:518`).
  - It shows:
    - each net's conduits in their own colour, hidden conduits included;
    - the link lines;
    - producers and consumers marked;
    - an unpowered marker on devices without power;
    - battery charge bars;
    - disconnected devices.
- **Inspector** (`buildingPane`, `cmds_build.go:100`):
  - power lines: "Uses 50 W" / "Produces 800 W", "Grid: +350 W, stored 900 of 1200 Wh", "No power: <reason>" (not connected / brownout / switched off / solar flare);
  - bars: battery charge, generator fuel (the existing Fuel bar), the wind or solar share.

  - solar `900 × (Outdoor−0.12)/0.88 ×` unroofed share;
  - wind `250 W` per unit of wind speed, capped at 1100, `×` clear share. The band is 5 cells along both long sides; it is blocked by walls, rock, roofs and trees, and cached per (structVer, roofVer, game hour). `roofVer` is a new counter bumped at `roof.go:87`.

  - Thermostat: `thermoRoomHeat` next to `heat.go:142`, ±220 per interval. Target 21 °C; active below target −1, idle above +0.5; the active flag is saved.

## PLAY022_PLAN
PLAY-022 plan: bigger animals, natural animal motion, puddles that blend · 2026-10-27 · d75c262df
- Changed:
  - Sizes: user decision, size means the AREA a body covers (XS ~1 tile, S 1.5, M 2, L 4, XL 6; sim footprint stays 1 tile). Lengths revised after the full render: XS 1.45, S 1.65, M 1.8, L 2.2, XL 3.0 (Go `sizeLength` and Python `SIZE_LENGTH` change together, a Go test checks they match; species trims +-15% stay). All animals re-rendered with a per-facing canvas and metadata `ppt` (px per tile, 128 unless long side > 384 px; every animal and corpse file), total animal pixels <= 1.2x the 291.3 M baseline.
  - Renderer uses each sprite's own `ppt` for origin, pose, shadows, culling and alpha boxes; picking: a click or box select on any tile the drawn body covers selects the animal (a pawn, item or building on the exact tile still wins); size thresholds (`largeSize`, marker/bar lift, snore offset) recalibrated.
  - Motion: metadata `gait` (tiles per 8-frame walk cycle, default 1.6) so planted feet do not skate; walk frames advance by distance / (gait x `Scale`); starts begin at the contact frame, stops finish to a neutral frame; one bob (baked); graze bouts last seconds and only on cells with something to eat; gait rate capped at display rate (no 6x aliasing).
  - Sim: `AnimalView.Gait` (`amble` wandering/grazing/penned, `walk` errands, `run` fleeing/hunting/charging/manhunter), step ticks walk = today's, run = flee step (60%), amble ~ 2x walk; slow species clearly slower. Goldens moved (animals wander slower), re-pinned with reasons. `Action == "graze"` only on an edible cell.
  - Puddles: per-tile stickers replaced by a chunk-baked smooth puddle veil (`puddle_veil.go`: coherent noise thresholded by `Wetness`, soft rimless edges, tinted terrain colour lit flat, drawn under grass tufts, fades under snow, kept at far zoom); never on water, ice, sand, mud, marsh, floors, roads, furrows or roofs; ripples only inside puddles; old puddle sprites, normals, metadata, `weather.py` builder and `puddleShape` removed (remove-unused-assets rule).
- Decisions: re-render every animal with capped density so animal memory stays about where it is (user); a gait holds for at least its current step (no-flap); path smoothing (Bezier corners, eased starts/stops) kept.
- Open: none specific; the Map viewport row stays `[-]` for other reasons, and Solar day/weather output (weather dims solar) is shared with PLAY-021.

### Open (verbatim)
The only plan parts that touch the still-open rows (Map viewport animal drawing and picking; Solar day/weather output has no PLAY-022 plan part):

   - **Picking:** a click or box select on any tile the animal's drawn body covers selects it (from the alpha box); a pawn or item on the exact tile still wins (`selection.go`, `multiselect.go:207`).

**C7. Picking (RENDER).** A click or box select on a tile covered by an animal's drawn body (its standing frame's alpha box at its drawn position) selects it. A pawn, item or building on the exact tile still wins.

## PLAY024_PLAN
PLAY-024 plan: sentry turret, trade, radio console · 2026-10-26 · d6f64a943
- Changed (original names and tuning; the ENG-003 wiki exception does not cover this feature):
  - Phase 0 contract (lead, done): construction kinds `SentryTurret`, `TradeBeacon`, `RadioConsole` (55 kinds); `power.ClassDefense` (shed last); `weapon.SentryGun`; research `Sentries` and `Radio`; jobs `Trade`, `UseComms`; orders `OrderTrade`, `OrderUseComms`; commands `ProposeTrade`, `CloseTrade`, `CommsCallTrader`, `CommsGift`, `CommsRequestCaravan`, `CommsRequestAid`, `CloseComms`; incidents `TraderCaravan`, `OrbitalTrader`; streams `streamTrade`, `streamDefense`; save blocks `Trade`, `Turrets` (nil when absent, no `SaveVersion` bump); offer codec `EncodeTradeOffer`/`DecodeTradeOffer`; read models (`AimDir`, `ShotAge`, `TradeView`, `CommsView`, `OrbitalView`); domain `economy/trade` and `faction/diplomacy.go`.
  - Sentry turret: 90 steel + 3 components, 140 HP, cover 30%, 120 W firing / 40 W idle, range 10, damage 9, cooldown 60, accuracy 75% (3%/tile falloff), search every 20 ticks staggered by id, hostile raiders/animals only, locks within 10 tiles and lets go past 12 or after 20 ticks out of sight, 30-tick aim, explodes when destroyed; raiders switch to a powered turret only when closer by `switchMargin` (no flap). Research Sentries 4500 (Gunsmithing + Electricity).
  - Trade: kinds bulk hauler, quartermaster (caravan), curio merchant (orbital); buy markup 45% - 1.2 pts/Social level - 0.06/goodwill (floor 6%); sell cut 50% - 1.2/level - 0.05/goodwill (floor 10%); sell price never above 90% of buy (no arbitrage); silver at par; +10% for a trader's specialty; hostile factions don't trade. Caravan: weight 14, cooldown 9 d, from day 4, stays 0.75 d; orbital: weight 12, cooldown 7 d, from day 6, needs a powered console, stays 1.5 d, goods within 6 tiles of a trade beacon, delivery by drop pod after 60 ticks. Killing or capturing a trader costs 40 goodwill. Trade beacon 40 steel.
  - Radio console: 80 steel + 3 components, 2x1, 150 W worked / 15 W idle; research Radio 3000 (needs Electricity).
  - Diplomacy: gift 1 goodwill per 40 silver, max 15; caravan request needs goodwill >= 0, costs 12, arrives in 6-12 h, 4-day cooldown; military aid friendly only, costs 20, 3-day cooldown (Phase 4, ships separately).
- Decisions: `Settle` revalidates against live counts because the sim runs while a dialog is open; new randomness only on the new streams; goldens of set-up colonies may move once traders join the incident pool (re-pin with a note); turret search is cadenced and indexed (`BenchmarkTurretsRaid`).
- Named guarantees kept: `TestTurretTargetNoFlap`, `TestRaiderTurretSwitchNoFlap`, `TestNoArbitrageAllKinds`, `TestTradeNilBlockKeepsHash`, `TestTurretShedLast`.
- Limits: embrasures and pack animals deferred.
- Open: military aid (allies fight raiders) and quests; see the Goodwill changes row.

### Open (verbatim)
Part owning the open Goodwill changes row:

#### Radio console and diplomacy
##### Radio console and diplomacy

| Thing | Value |
|---|---|
| Console | 80 steel + 3 components, 300 work, 2x1, rotatable, interaction spot south of the desk (facing south), blocks movement |
| Console power | 150 W while worked, 15 W idle (`ClassBench`) |
| Research | `Radio` "Radio relay", 3000, needs Electricity; unlocks the console and the beacon |
| Gift | 1 goodwill per 40 silver, at most 15 per gift |
| Military aid | friendly factions only; costs 20 goodwill; 3-day cooldown (Phase 4) |
| Caravan request | needs goodwill ≥ 0; costs 12 goodwill; arrives in 6–12 h; 4-day cooldown |
| Orbital call | needs a trade beacon (disabled with a reason otherwise) |

| 4 | aid agent | opus | military aid (allies fight raiders); `raids.go` exclusive; ships separately |
