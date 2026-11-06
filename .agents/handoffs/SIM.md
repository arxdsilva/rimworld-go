# SIM handoffs

Families: SIM. Each section condenses one task's handoff; the full original of `<stem>` is `git show 76d605d04:.agents/handoffs/<stem>.md` (a plan: `git show 76d605d04:.agents/handoffs/_plans/<stem>.md`). New sections go at the end, in the format of `.agents/templates/HANDOFF.md`.

## SIM-001-climate-and-spoilage
Climate, seasons, spoilage and frost · 2026-10-06 · c0092057b
- Changed: `domain/world/temperature` in integer tenths of C. Outdoor = daily mean + ±6C swing (coldest 05:00, warmest 15:00); DailyMean is a yearly triangle 9C ±15C (peak day 15, trough day 45; start mild spring; winter nights about -12C); Indoor = mean + 3C for any enclosed room. Seasons: spring days 53-7, summer 8-22, fall 23-37, winter 38-52.
- Changed: `Stack.Rot` and `Kind.ShelfTicks()` (at 10C or warmer): meat 2 d, corpses 2.5 d, meal 4, lettuce 6, berries 10, radishes 14, potatoes 30, corn 40, pumpkins 50, wheat/beans/rations never. `RotRate` permille: 0 at <=0C, linear to 10C, 1000 above. Merges weight rot by quantity; rotten and fresh corpses never merge.
- Changed: `phaseClimate` every 250 ticks. `rotItems` deletes spoiled food ("N meat rotted away"); a rotten corpse stays (not butcherable) and decays after twice its shelf life; carried or reserved stacks rot but are removed only when free. `frostCrops` kills below FrostKill: corn/beans/pumpkin -1C, potato -2, lettuce -3, radish -5, wheat -8; wild plants never die of frost. No sowing when the day mean at the cell is <=3C (`growingSeason`). Growth x `plant.TempFactor` (0 at <=0C, full 10-35C, 0 by 45C).
- UI: top bar season and outdoor temp; hover tile temp (indoors marked) and item "spoils in N days"/chilled/frozen/rotten; messages top-left ~10s (`WorldView.Messages`, last 12); alert "Food spoiling soon (N)" (<1 day left); crop picker shows frost-kill and shelf life. Read model: WorldView OutdoorTemp/IndoorTemp/Season; ItemView FreshDays/Rotten/RotRate; CropView FrostKill/ShelfDays.
- Decisions: the no-sow rule exists because crops sown into winter froze at once. Seed-7 year probe (`RIMWORLD_PROBE=1 go test ./internal/application/game -run YearProbe -v`, ~15 min): 3 colonists alive, ~900 food left; frost ~day 38.
- Open: cooler/freezer room, heaters/campfires, pawn comfort, weather (all later done in SIM-003/016).

## SIM-002-drying-rack
Drying rack: dried food that never rots · 2026-10-06 · 80ddf0380
- Changed: `construction.DryingRack` 2x1 rotatable, 6 wood, bench with default forever "Dry food" bill. `recipe.DryFood` takes up to `RackCapacity` (25) of one stack of any undried, unrotten perishable (incl. meals; corpses are butchered; wheat/beans already dry); 90 ticks to lay out; `game.racks[bid]` holds a `dryBatch`; a loaded rack takes no new bill until it drops.
- Changed: `dryRacks` runs with `phaseClimate`. Full-speed drying: lettuce 0.75 d, meat/meals 1, radishes/corn 1.5, berries/potatoes 2, pumpkins 2.5. `item.DryRate(temp)`: full from 20C, slower when cool, 25% freeze-dry below 0C. Food doesn't rot on the rack. Finished batch drops on the work spot as a dried stack ("25 dried meat ready").
- Decisions: `Stack.Dried` => `Shelf()` 0, never rots; dried and fresh never merge, splits keep the flag; nutrition unchanged. Tearing down a loaded rack drops the batch undried with its rot.
- UI: BuildingView DryKind/DryQty/DryPct, ItemView Dried; bill status "drying a batch"; inspector "drying 25 meat, 40%"; rack draws the batch browning; dried stacks browned, rotten corpses sickly green; Architect Production > Drying rack. `-scenario drying -capture-ui drying`.
- Fix: `centerOn` marks the camera placed (first alert click/capture was undone by first-frame centring).

## SIM-003-heat-and-exposure
Room heat, campfires and exposure · 2026-10-06 · e0d72c301
- Changed: per-room temperature `g.roomTemps` (`heat.go`). On rebuild (`remapRoomTemps`) a new room starts at the average of its cells' old values; first build at Indoor(). Every 250 ticks `heatRooms` moves each room toward outdoor +3C by `roomLeak` = 120/sqrt(cells) permille clamped 10-80, and adds `campfireHeat`/cells (160 tenths*cells; holds a 20-cell room ~30C above outside). A campfire banks (no heat, no fuel) once local air hits 16C. `RoomView.Temp`; a room with a fuelled fire counts as lit (a fire isn't a lamp for impressiveness).
- Changed: `construction.Campfire` 1x1, 5 wood, blocks movement, Architect > Furniture. Fuel `g.fuel[bid]` ticks, capacity 20 wood, `TicksPerWood` 6000 (~10 logs/day). Haulers refuel fires at half or below (`job.Refuel`, 30 ticks, beside the fire). BuildingView Fuel/FuelPct/Burning; alert "Campfire out of wood"; flame, ember and smoke particles.
- Changed: `woodLow()` (wood on ground < max(20, two days of burning per fire)) makes Grow colonists cut designated trees before harvesting or sowing.
- Changed: `domain/pawn/exposure`: comfortable 8-28C; thoughts cold -4 (<8), freezing -10 (<-5), hot -4 (>28), sweltering -10 (>34). Hypothermia builds <=0C (faster colder), heatstroke mirror >38C. Severity permille: 200 seeks a bearable room, 600 collapses, 1000 dies, <400 a collapsed colonist stands up. At -10C collapse ~2.4 h, death ~4 h. Health causes CauseHypothermia/CauseHeatstroke: doctors rescue to bed but don't tend. Mild exposure: drop everything but eating, `job.Warm` to nearest bearable room, wait up to 2500 ticks; retry every 1000 if none (`g.noComfort`). PawnView Hypothermia/Heatstroke/Temp; alerts per colonist.
- Fix: `assignCut`/`assignHarvest` use `nearestPlant` (closest reachable unreserved, 16 candidates); walled-in designated trees no longer loop "cut (unreachable)".
- Probe (seed 7): hut ~16C at -8C outside, ~11 logs/day; all 3 survived.
- Dev: `Game.SkipTo`, `FuelCampfire`, `ClearPlants`; `-scenario winter`.
- Open: apparel (done SIM-005), heaters/coolers with power, doors leaking heat, heat flowing between rooms.

## SIM-004-free-time-and-bedtime
Free time, bedtime priority, stack counts · 2026-10-06 · 8cf51a8a1
- Changed: idle colonists in work or recreation hours start `job.Wander` (random tile within 5, 9 for wanderers), `Chat` (beside nearest free colonist within 16; speech bubble; ends if partner leaves), `Relax` (nearest free chair, reserved) or `FireSide` (beside a burning campfire); linger 240-720 ticks, failed pick falls back to wander. Temperament from a name hash (stable) weights Wander/Chat/Relax/Fireside: Wanderer 6/2/1/1, Social 2/6/1/2, Homebody 1/2/4/4; `PawnView.Temperament`.
- Decisions: free-time jobs never block work: re-check every `idleRecheck` 30 ticks, immediately when hungry or tired. Bedtime beats everything including player priority (eats first if hungry); drafted colonists and seeking warmth are exempt.
- UI: stacks >1 show a count tag bottom-right of the tile, hidden when tiles <36 px.
- Limits: two older tests assuming colonists stand still were loosened.

## SIM-005-clothing
Clothing · 2026-10-06 · dad647000
- Changed: items Leather, Parka, Tuque, Duster; butchering gives leather via recipe `Extra`/`ExtraQty` (5 per dustbuck, 4 per ridgewolf). Tailoring bench (`construction.TailoringBench` 2x1, 7 wood, Production): parka 14 leather/900 work, tuque 4/300, duster 10/700 (Craft work). Stockpiles always accept goods.
- Changed: `g.apparel[id]` worn by `item.Slot` Body/Head, over everyday clothes. Insulation (cold/heat): parka +20C/-3C, tuque +6/0, duster +4/+8. `exposure.Bands` replaces fixed constants; `bandsOf(id)` = Everyday 8-28C widened by gear; exposure, thoughts and warmth-seeking use each colonist's own band.
- Changed: right-click "Wear X" (`OrderEquip`, queueable; replaces the slot). Autonomy `assignWear`: wear a free garment fitting the season better by >=3C (day mean ±10C), remove one that makes it worse (parka in summer).
- UI: PawnView Apparel/ComfortMin/ComfortMax; inspector Gear tab enabled (weapon, garments, comfort range, hint); worn-layer art for parka, duster, tuque. `-capture-ui dressed`.

## SIM-006-cooking-and-dining
Campfire cooking and eating at tables · 2026-10-06 · 12c54d1c3
- Changed: campfire is also a bench (spot south). `recipe.CampfireMeal`: 2 raw food -> 1 simple meal, 180 Cook ticks (stove 120). Fire takes bills only with wood (status "fire needs wood", `BillNoFuel`). New campfires get default "until 10 meals" bill; `BuildNow` fixtures get none.
- Changed: `dining.go` `carryToTable`: seat beside a table within 30 tiles of the food (chair worth 6 extra tiles, else any free floor beside the table), splits `portionsFor`, eats there; no table in reach or already at one -> eat on the spot (`j.Count`=2). "Ate at a table" now needs a table adjacent (diagonals included).
- Open: fine meal (meat + veg) needs a multi-ingredient haul first.

## SIM-007-recreation-and-mental-breaks
Recreation need and mental breaks · 2026-10-06 · 00154c58f
- Changed: `domain/pawn/recreation` 0-10000 ("fun"), start 7000, drains 1 per 14 awake ticks (~4300/day, none asleep). Gain per 10 ticks: wander 4, relax 6, fireside 6, chat 8, horseshoes 13. Thoughts: <1500 starved -12, <3500 bored -5, >=8500 entertained +4.
- Changed: `construction.HorseshoePin` 1x1, 3 wood (Furniture); un-entertained colonists pick it within 30 tiles (wanderers half as often), throw from 3 tiles (`job.Horseshoes`).
- Changed: mental breaks (`domain/pawn/mentalbreak`, `mental.go`): hourly (staggered), mood <20 gives 2% per point under 20 (40% at 0). Kinds: sad wander 8 h, food binge 4 h (>=3 portions, `bingeEat`), hiding in room 10 h (`job.Hide`, own bed else nearest room). During a break MovePawn/Prioritize/DraftPawn are dropped, undrafted, queue cleared; still eats starving, sleeps exhausted. End: catharsis +30 for a day and a message.
- UI: inspector "MENTAL BREAK: sad wander", danger alert, storm cloud over colonist; PawnView Recreation/MentalBreak. Event messages centred under top bar; inspector room label right-aligned. Dev `Game.ForceBreak`, `-scenario fun`.

## SIM-008-threats
Threats: raids and winter wolf packs · 2026-10-06 · 8870c099b
- Changed: `raids.go` raiders as pawns apart from the colony (`raiderIDs`, `raidGroups`; modes arrive, assault, flee). `thinkRaider` targets nearest standing colonist within 18, rifle or club. Hurt raiders or a group that lost half flee to the edge. Downed raiders bleed out after 4 h and drop their weapon; corpses linger half an hour.
- Changed: storyteller `scheduleThreats`/`phaseThreats`/`fireThreat`: first threat day 5, then every 5-9 days; winter 50% chance of a 2-3 wolf pack; raid size `min(5, 1+day/10)`. Colonists defend (14 tiles), auto-engage, accept drafted attack orders. Save carries raiders, groups, next threat tick and the on/off switch.
- UI: hostiles rendered but excluded from selection, bar and colonist alerts; red chevron (orange fleeing); reserved `raider` cloth tint; "Raid: N raiders" danger alert. `-scenario raid`.
- Open: pause-on-threat, damage numbers (later done); raid strategies and wealth points (SIM-013); manhunters (SIM-012).

## SIM-009-defences
Defences: doors, sandbags, spike traps · 2026-10-06 · 47680c76c
- Changed: buildings have `MaxHP` (doubled for stone) and saved `Damage`; `CoverPct` per kind; new `sandbags` (walkable, 55% cover, stone OK) and `spike trap`.
- Changed: `defense.go`: `hostileMap` (colony doors shut to raiders; animals always use it); `startBreach`/`doBreach` (unreachable target: plan as if walls/doors gone, bash first one: club damage, 7 rifle butt, every 60 ticks; 0 HP removes); `coverFor` cuts ranged hit chance by best cover beside the target facing the shooter; traps spring on raiders and predators 75% (32 injury) and are re-laid as a blueprint, colonists never spring them; `assignRepair`/`doRepair` (`job.Repair`, Construction work, only when no raiders stand). Raiders attack from where they stand if in reach.
- UI: Security architect category, health bar under damaged buildings, art `sandbags`/`spike_trap`, palette material `burlap`. `-scenario defense`.

## SIM-010-infection-and-medicine
Infection, herbal medicine, healroot · 2026-10-06 · 6d47b4218
- Changed: `domain/pawn/infection`: care levels untreated/self/tended/medicine, `Chance` 40/20/12/3%, hourly `Step` +4/-5/-9, Onset 15, Fatal 100, Sick 20, DelayHours 8. `item.HerbalMedicine`; `plant.Healroot` (6.5 days, 2 medicine/plant, soil sensitive). `health.CauseInfection`; moods Sick -8, InPain -6 (injury >=40).
- Changed: `infection.go`: saved `g.wounds`; hourly `phaseInfection` (wound untended 8 h may fester; a tend lasts a day; kills at 100). `patientRest`: sick (>=20) or hurt (>=40) colonists with another doctor lie down (up to 6 h past wake-up). Doctors tend anyone lying down, fetching one herbal dose first; a lone colonist self-tends (weaker). Tend: injury healed 8 (medicine 16, self 4).
- UI: PawnView Infection/Wounded; status "infection N%", "wound untended", "(tended)"; healroot art; `-scenario medic`.

## SIM-011-research
Research · 2026-10-06 · 37b698054
- Changed: `domain/research` projects FoodPreservation, Herbalism, Tailoring, Traps, Gunsmithing (needs Traps) with cost, prereqs and unlock keys ("build:", "crop:", "recipe:"). `construction.ResearchBench`, `recipe.MakeRifle` (12 wood), `job.Research`, `command.SetResearch`/`WorkResearch` (Research on by default, last in `work.Order`).
- Decisions: a point per working tick, two-hour sittings; gates on DesignateBuild/DesignateGrowZone/AddBill/`CanPlaceAt` ("needs research: X"). `New` is a sandbox with everything researched; `NewColony` starts with nothing; `BuildNow` bypasses gates. Progress saved.
- UI: Research tab (`research_ui.go`); locked tools/crops/bills greyed with reason on hover; WorldView Research/Locked, PawnView WorkResearch. `-scenario research`.

## SIM-012-animal-handling
Animal handling: taming, pens, guards, manhunters · 2026-10-06 · 12bd840e5
- Changed: `domain/animal` TameChance/Revenge per species, `TameWork`, `ManhunterTicks` (a day), `TameRetryTicks`. `handling.go`: `DesignateTame` (+cancel), `assignTame` under Hunt work; success names the animal; failure sets a retry delay and may enrage it (danger message pauses). `provoked`: wild animals hurt by a colonist may go manhunter (`thinkManhunter` hunts the nearest standing colonist until rage ends).
- Changed: tame animals (`thinkTame`): wolves guard (raiders/hostile/enraged within 10), grazers flee only wild predators; all keep to nearest pen or within ~12 tiles of first bed; real grid (doors open), never spring traps, not hostile. `DesignatePen` rectangles (`g.pens`, saved, WorldView Pens), `ReleaseAnimal`. Storyteller: outside winter a quarter of threats from day 8 are manhunter dustbuck packs. `chase` shared close-in-and-bite helper.
- UI: Orders > Tame, Zones > Animal pen, tame/manhunter overlays, names, animal inspector Tame/Release/Slaughter, Animals tab. `-scenario animals`.

## SIM-013-raid-scaling-sieges-sappers
Wealth-scaled raids, sieges and sappers · 2026-10-06 · ba8b74898
- Changed: `Game.Wealth()` = goods (`item.Value`) + buildings (2x wood + work/20, stone x1.5) + tame animals. `raidPoints(day)` = wealth/40 + 30 per standing colonist + 2/day; `raidKit` spends on raiders (club 60, rifle 90 at 55%), 1 to 8. Storyteller `scaledRaid`: sappers (day >=10, colony `enclosed()` from edge, 50%), siege (day >=12, 30%), else assault. Dev hooks `Raid`, `Siege`, `Sappers`. `raidGroup` gains Plan, CampX/Y, Camped, Stormed, Started, Until (saved).
- Changed: siege (`thinkSiege`): camp 18 tiles from `home()`, dig in after arriving or 2 h (sandbags in front, danger message), hold and shoot in range, never chase, storm after 5 h. Sappers: `sapperMap` (doors shut, traps avoided), `startBreach` through walls/doors/rock (`sapperBreaks`), `doDig` rock 90 HP (`rockDamage`, saved).
- Fixed via year probe: smashed buildings leave `Rebuild` blueprints (`rebuildFirst`); breached rooms keep a saved `residual` temperature (`residualLeak` 6%/interval) instead of jumping to outdoor; failed-path back-off `pathRetry`/`raiderRetry`; `edgeTile` only picks edges the colony can be reached from; raiders go home `raiderGiveUp` (a day) after arriving. Probe colony survived the year (8 threats).
- UI: PawnView RaidPlan; alert "Raid / Siege / Sappers: N raiders". `-scenario siege|sappers`.

## SIM-014-movement-timing-and-blocking
Movement timing, diagonal cost and creature blocking · 2026-10-19 · 3176ecff3
- Changed: per-creature `motionRec` (`motion_store.go`: NextMoveAt, StepTick, StepTicks, BlockedTicks, BlockedBy) replaces the global `(now+id)%15` slot. Step 15 ticks straight (`TicksPerCell`), 21 diagonal; animals `Step()` or round(Step()*1.414) (`animal.StepCost`). `phaseMovement` moves colonists, raiders and animals in ascending ID, applying at once. PawnView/AnimalView expose StepTick, StepTicks, NextX/Y, HasNext, BlockedBy.
- Decisions: occupancy index (`occupancy.go`; downed/sleeping/carried don't block; lower ID keeps a shared cell). `enter()` verdict order: free go; my attack target wait; ring of walkers (head-on = 2-ring) highest ID yields every 15 ticks (sidestep, repath, back off, wait); walking occupant wait; idle occupant and walker blocked >=30 ticks gets a make-way job (raiders/drafted/breaking never yield); stationary occupant on last cell: exact-cell jobs wait, move orders and animals take nearest free, others arrive adjacent; else repath at >=45 blocked ticks, every 45 (`pathAround` +40 occupied, +20 beside stationary). After 600 blocked ticks the job fails as `job.Blocked`. Haulers that detour keep their drop-off.
- Changed: `domain/ai/pathing` octile 10/14 costs and heuristic, no diagonal past wall corners, optional `Coster`, pooled buffers; tie-break (f, g, y, x) unchanged.
- Determinism/saves: deterministic but seeds diverge from earlier builds; `motionRec` saved, older saves load "due now"; `SaveVersion` stays 1; occupancy transient. Guarantees: `TestOldSaveWithoutMotionLoads`, `TestNoSharedCellInvariant` (seeds 1-5 x 20k ticks), `TestBlockingSameSeedSameRun`.
- Bench: Hour 16.5 -> 17.3 ms (+5%), Raid 18.6 -> 19.5 ms; allocs 5.6 MB -> 0.31 MB and 12 -> 0.54 MB per op; movement phase ~0.2 -> ~1.6 ms/hour.
- UI: ART-007 eases blocked creatures to a stop; inspector "waiting for <name>". Dev `-scenario corridor|crowd`, `-capture-speed`.
- Open: simultaneous move orders to one cell fail Reserved (needs formation order); no movement speed stat or terrain cost; no region graph (full rate-limited A*). Addendum: colonists and tame animals later walk through each other (SIM-037, PLAY-010).

## SIM-015-action-read-model-knife-spear
Action read model, knife and spear · 2026-10-19 · dc16a8353
- Changed: `PawnView.Action` canonical per job (`action_view.go`): aim/melee, build, deconstruct, chop, mine, sow, harvest, eat, horseshoes, research, tame, repair, breach, tend (not while fetching medicine), refuel and bills only while carrying input (stove/campfire/rack cook; tailoring bench tailor; butcher table butcher; others craft), haul/rescue while carrying; animals flee, bite/kick, graze. `ActionProgress` = 1 - WorkLeft/total, -1 when unknown (haul, rescue, attack, breach, repair). `AttackCycle` capped 0.99, 1 right after the strike. `ActTarget`, `Carrying` ("pawn" while rescuing). `HitTick`, `HitFromX/Y`, `HitDamage` stamped on victims in `strike()`. `TraceView.Attacker/Target/Weapon`.
- Changed: knife (3 stone, 200 work; dmg 8, cooldown 70, accuracy 90, raid points 50, value 10) and spear (5 wood, 320 work; 13, 125, 85, 70, value 20), range 1. `kitOf` rolls rifle 50%, club 25%, knife 12.5%, spear 12.5%. Recipes appended so saved IDs hold.
- Determinism/saves: strike/hit stamps saved additively (`saveState.Strikes`, `Hits`; needed for identical save/load continuation); raid composition diverges.
- Notes: a trace first appears with `Age` 1, so strike frames key on `Age <= 1`; `HitTick` = `Tick()-1`. Bench: Hour 16.6 -> 16.9 ms, Raid 18.9 -> 20.1 ms.
- UI: knife/spear in bill list and wielded; presentation by ART-008.

## SIM-016-weather
Weather · 2026-10-19 · d09965b21
- Changed: `domain/world/weather` (pure): clear, cloudy, rain, storm, snow, blizzard in 6-hour blocks from a seasonal Markov table (`Row()`), splitmix-hashed from (world seed, block), no global RNG. Spring/Fall rain and storm x1.3, Summer x0.7. Day mean <=0C turns rain to snow and storm to blizzard; no snow above +2C. Intensity eases over an hour (`State.Goal`). Wind drifts. `SnowDepth` builds in snow, melts >0C, rain halves it once per block; `Wetness` dries twice as fast in summer. Weather seed hashed from saved terrain.
- Changed: `weather.go` `phaseWeather`. Outdoor steps cost more (`weatherSlow`): rain +15%, storm +25%, snow depth >0.3 +20%, blizzard +40%. Outdoor campfire doused in rain or snow at intensity >=0.3 (`heatRooms` goes through `burning()`). Outdoors -2C in snow, -5C blizzard. Thoughts "soaked" -3 (6 h), "snow day" +2.
- UI/read model: WorldView Weather/WeatherIntensity/WindX/WindY/SnowDepth/Wetness; PawnView Soaked; PlantView Wilted. Dev `SetWeather`, `-scenario rain|snow` (just past a block boundary, holds ~5 h).
- Determinism/saves: seeds diverge; `weather.State` saved additively, older saves start clear. Bench Hour +4%, Raid +3% (incl. SIM-017).
- Limits: no fog kind; storms only darken; `Wilted` has no per-plant stall timer; no weather controller UI (`Set weather` unchecked).

## SIM-017-burial-cremation
Corpses, graves, funeral pyres and mourning · 2026-10-19 · d09965b21
- Changed: on death (`die()`) a colonist or raider drops the weapon and leaves `item.CorpseHuman` (`Stack.Of`, `Stack.DiedAt`; `CanMerge` needs same `Of`). The pawn stays in `Snapshot()` dead and `Embodied`; raiders no longer vanish after `corpseLinger`. Human corpses rot into never-spoiling `item.Bones`, are not butcherable and never stockpiled.
- Changed: `construction.Grave` 1x2 rotatable, dug (`Def.Dug`: work 400, no materials). Empty grave walkable; filled one blocks and cannot be deconstructed. `graveRec{Occupant, Name, Cause, DiedDay, BuriedDay, Headstone}` (headstone = occupant ID hash mod 4).
- Changed: `job.Bury` (haul work, ahead of hauling): nearest free body (lower ID wins ties), nearest reachable empty grave, fill 200 work; "<Name> was laid to rest" and every living colonist gets `LaidToRest` +3. Vanished target fails safely.
- Changed: `construction.Pyre` (6 wood, 2x1 bench), default bill `recipe.Cremate` burns one body + 2 wood (wood taken from nearest free stacks at completion; bill waits for 2 free). Message "<Name> was cremated" + `LaidToRest`.
- Decisions: thoughts `ColonistDied` -6 for 3 days (colonists only); `UnburiedCorpse` -2 state thought while a human corpse is within 8 tiles in the same room or both outdoors (scans the item array, not the map).
- Determinism/saves: ties by ID; seeds diverge (raider bodies persist). Grave records saved additively; on load a dead pawn with no body and no record gets a corpse, idempotent. Bench within 5%.
- UI: Architect Furniture Grave/Funeral pyre; grave inspector "Here lies <name>"/"Empty grave"; ItemView Of/OfName, BuildingView Occupant/Epitaph/Headstone; actions dig/bury/cremate; art ART-009. `-scenario burial`.
- Limits/Open: no player grave assignment, sarcophagi or funerals; pyre wood is drawn from stock, not carried.

## SIM-018-roofs
Roofs · 2026-10-20 · 9d64cde88
- Changed: `domain/world/roof` cell flags Built, Natural, PlanBuild, PlanRemove, Supported, NoAuto; `SupportRange` 6 (Chebyshev), `AutoMaxCells` 150, `BuildWork` 30, `RemoveWork` 15; `SupportField` multi-source BFS. Support = Rock or completed Wall/Door, recomputed on every room rebuild.
- Changed: auto-roof plans every unroofed supported cell of a closed room of <=150 cells unless the player removed it (NoAuto). Commands `DesignateRoof` (clears NoAuto/PlanRemove), `DesignateRemoveRoof` (sets NoAuto; natural roofs untouched), `DesignateCancel`. `job.Roof`/`Unroof` under Construct (nearest planned tile, lower row-major tie; tries 3 nearest reachable then waits).
- Changed: unsupported built roofs collapse in `phaseClimate` ("A roof collapsed near (X, Y) — no support within 6 tiles.", one per batch, `RoofCollapses` stamp 1 h, not saved). Mining/sapper digging leaves a Natural roof that never collapses. All writes via `setRoof` keep a sorted plan index and O(1) per-room roofed permille.
- Changed: heat with roofed share f: room drifts to `Outdoor + IndoorBonus*f` and leaks `roomLeak*(1+4(1-f))`; fully roofed unchanged. `outdoorsAt` is now `underSky` (no roof; walls/doors count as covered) for soaked, snow day, `weatherSlow`, `weatherTempDelta`, `doused`, Soaked, Wilted; "Slept outside" needs open sky. Fix: `RoomView.Lit` needs a burning campfire.
- Saves/determinism: `saveState.Roofs` additive; old saves auto-roof closed rooms on load. Seeds diverge where rooms exist. Bench flat (17.5 / 20.5 ms).
- UI: WorldView RoofCells/RoofCollapses/Hour/DayOfYear, RoomView.Roofed; overlay, tools, lighting in UI-010/ART-010. Dev `RoofRooms()`, `HoldWeather()`, `-scenario roofs|sunny`.
- Limits: no collapse injuries, no thin vs overhead-mountain roofs; plants under roofs still grow.

## SIM-019-wildlife
Wildlife: 61 species · 2026-10-21 · e21011daa
- Changed: `domain/animal` roster of 61 species (key, archetype, size XS-XL, behaviour, habitat, natural weapon, herd size, speed class, flags dimorphic/explosive/small-prey). Enum values append-only (deer, TimberWolf took the dustbuck/ridgewolf slots; old names still parse). Ten originals (RimWorld-invented, so original designs): emberbuck, cinder_rat, woolhorn, glimmerstag, timbergnaw, hive_beetle, hive_brute, hive_crawler, ground_sloth, dire_wolf.
- Decisions: numbers are our own formulas (`buildDef`) from size and behaviour tables: HP XS 12, S 28, M 55, L 100, XL 170; step 16/14/16/18/22 (fast -4, slow +6, flee x0.6); meat 2/6/14/30/55; damage 2/4/7/11/16; cooldown 100-140; accuracy 80-70. Tame/revenge: domestic 75/5, pet 70/3, grazer 40/10, skittish 30/0, docile 50/5, territorial 15/50, predators 15/45, hive 0, legendary 5/100; legendary HP x2.5, tortoise x1.5.
- Changed: predators hunt prey no bigger than themselves (small-prey only XS/S); hive insects hostile from spawn, untameable, never flee; explosive animals burst on death (10-20 damage within 1.5 tiles, `WorldView.Bursts`, no fire spread); legendary enrages its kind when hurt.
- Changed: spawning (`wildlife.go`, `g.rng` only) weighted by season and habitat (cold x3 winter x0.3 summer, hot reverse, domestic/pet x0.15, no legendary/hive). Threat roll out of 8: winter predator pack 4, hive swarm 1 (from day 12), raid 3; else manhunter pack 2 (from day 8), hive 1, raid 5 (raids were 1/2 winter, 3/4 otherwise). Hives burst from rock nearest the colony; glimmerstag visit from day 20, 2%/day, once a year.
- Changed: one `CorpseAnimal` kind with `Stack.Animal` (no cross-species merge), one butcher recipe by size; legacy corpses and bills convert on load; `splitLoad` keeps species. Views AnimalView Label/Attack/Female/Size, ItemView.Animal. `-scenario zoo`.
- Determinism: seeds diverge (start wildlife changed). Bench Hour +4.5%, Raid +0.5%; animals with no flee radius skip the threat scan. UI: ART-011/012.
- Limits: no milk/wool/eggs/juveniles (later SIM-026), no training; explosions start no fires; female sets share male corpse sprites.

## SIM-020-blood-stains
Blood and ichor: stains on floors and bodies · 2026-10-21 · 87c437c34
- Changed: `domain/world/stain` Blood/Ichor, `Stain{X,Y,Kind,Amount,Born,Variant,Rem}`; Full 1000; a full stain fades in exactly 7 days (`FadeTicks` 420000); rain washes in 2 h (`RainWash`), snow 3 h (`SnowWash`) off open ground at intensity 1; cap 2000, faintest dropped first; saved fixed-point `Rem` makes 250-tick steps exact. Under a roof stains only fade.
- Changed: `stains.go` amounts: hit 40+25*dmg on the target tile (rifle hit 14 = splat, two = pool); heavy hit (>=6) also 40% beyond the target away from the attacker; death pool 700; bleeding drip 80 every third tile (colonists with open untended wound and Injury >=15, raiders >=30, animals <50% HP); downed untended 60/h up to 600. Hive insects bleed ichor. Decay in `phaseWeather` each climate step.
- Determinism/saves: no RNG (variant from tile hash); `Stains` additive; `WorldView.Stains` sorted row then column.
- UI: `blood_draw.go` floor decals (drips <0.2, splats <0.6, pools; dry red to dark brown over a day; ichor yellow-green; alpha amount*0.85); body blotches via `lit.kage` `Custom3 = 32*ichor + 2*wetStep + coverage` (hair, held items, carried patients stay clean); sprays 4+dmg/2 particles (max 16). Bench: StainDecay(2000) 0.074 ms, DrawStains(500) 0.017 ms, DrawArt flat.
- Limits: stains under later-built walls stay until faded; corpses keep no body coverage. Cleaning came in SIM-021.

## SIM-021-dirt-cleaning-home-carpets
Tracked dirt, cleaning, Home area, cotton, carpets · 2026-10-22 · c1e571b9e
- Changed: `domain/world/filth`. Boots pick up 6 dirt/step on outdoor soil or gravel (grass halves, full-wet rain triples); field work (sow, harvest, cut, mine, dig grave) adds 12 per 30 ticks; stone/built floors outdoors shed 3%/step; indoor steps drop 8% of carried dirt (min 1, only while carrying >=10). Carpet wipes cloth 70%, leather 55%, straw 45%, wood 30%, and 15% of the wiped stays on the tile. Indoor dirt never fades; rain/snow wash outdoor dirt at stain rate. Colonists and raiders only. Every write via `addDirt`/`stainMoved` -> `filthMoved`; dense cells + sorted `nz`, `tread` flags rebuilt with rooms.
- Changed: cleaning (`clean.go`): `cleanQ` holds Home tiles with dirt+stain >=120 and a tile leaves only at 0 or when no longer Home (hysteresis: half-swept stays queued). `assignClean`: nearest by (distance, index), max 16 reachability probes; 20 removed per work tick, dirt first. Clean work type on by default, last in tie-break; old saves switch it on once (`CleanWired`).
- Changed: Home area (`home.go`): `DesignateHome`; auto-grows 2 tiles around every completed building except floors/graves; new colony starts with a 13x13 Home; old saves get Home around buildings. WorldView Home/HomeVer.
- Changed: room cleanliness (`rooms.go`): up to -10 impressiveness (`filth.Penalty`, 200 filth/cell = full); `afterSleep` follows; RoomView Filth/Cleanliness/FilthPenalty; `roomSum` incremental and exact (skipped during rebuilds).
- Changed: cotton (9 days, 5 cloth), wheat drops 4 straw; carpets crafted at workbench (straw x10, wood x6) or tailoring bench (cloth x8, leather x6), a walkable 1x1 building whose material is the carpet item (deconstruct returns it); Build swaps unusable materials for the default.
- UI: Zones > Home/Clear home, Furniture > Carpet with material picker and catch %, Work tab Clean column, "Dirty boots N%", room labels ("bedroom, dirty (-3)"), Home overlay, dirt prints (`dirt_draw.go`), broom pose + `sfx.sweep`.
- Verified: a 3000-op seeded property test checks incremental sums against a full recount. Bench: DirtStep 19 ns, AssignClean(4000) 0.012 ms, ColonyHour +-0%.
- Limits: dirt subtle (many trips to reach 120); no vomit/trash filth; floors have no cleanliness/beauty of their own; no switch to stop Home auto-expansion; no firefighting/repair in Home; cloth carpet untinted.

## SIM-023-biomes
Biomes: twelve biomes, new ground and wild plants · 2026-10-22 · 04d46d084
- Changed: `domain/world/biome` has 12 biomes (temperate forest, boreal forest, tundra, cold bog, ice sheet, sea ice, temperate swamp, tropical swamp, tropical rainforest, arid shrubland, desert, extreme desert), each with key, name, blurb, difficulty label, `temperature.Climate`, wetness, ground mix, wild plant densities, grass density/regrowth, wildlife density. New terrain sand, marsh, ice (walk cost marsh x2, sand x1.2, ice x1.3); plants pine, jungle tree, cactus, shrub, reeds (die outside their climate band).
- Decisions: numbers are our own tuning; biome choice is direct (no world map). Temperate forest keeps the original generator, locked by `biome_golden_test.go`. Biome saved with the world; old saves load as temperate forest.
- Entry points: `game.Biomes()` (`BiomeInfo`), dev `-biome`, `AppConfig.DefaultBiome`.
- Limits: no tile picker (wizard, UI-014); no per-biome disease, forage, topology or hilliness; biomes are Go data, not data packs.

## SIM-024-animal-ecology
Animal ecology: needs, states, grazing, troughs · 2026-10-22 · 04d46d084
- Changed: `domain/animal/ecology.go` rows (diet, core/marginal biomes, egg kind, spawn weights; domestic never spawned wild; sex/age rolled). Every animal has Food and Rest; states Wander, Feed, Rest, Hunt (shown Attack), Flee, Leave with hysteresis. Feeding: grazing, browsing, crop raiding (alert), carcasses; timbergnaw fells trees; starvation kills. Herds bed down together; predators hunger-driven and give up; badly hurt fighters break off.
- Changed: feeding troughs in pens (plants/meat/any filter), haulers fill them (`troughs.go`, `trough_feed.go`).
- Decisions: `animal_index.go` bucket index; animals decide fully every 60 ticks, no per-tick global scans. Animals-only bench 0.205 s (was 1.46 s). ColonyHour/Raid rose to ~30/32.8 ms (from 22.3/24.9) because start population is now sized to carrying capacity (~15 vs 8 on the 48x40 map; 18.7/24.6/29.5 ms for 0/8/15 animals).
- Limits: trough feed doesn't rot and is lost on deconstruct; hurt predators can starve; wild animals eat loose start-kit food; no taming skill, wildness, food requirement, training.

## SIM-025-wildlife-population-events
Wildlife population and nature events · 2026-10-22 · 04d46d084
- Changed: `population.go` per-biome carrying capacity; arrivals at edges below 70%, herds leave above 130%, predator cap; start population sized to capacity. Incident framework (`domain/incident`, `incidents.go`): weights, cooldowns, biome limits; a nature event considered every 2-5 days: herd crossing, stray joins, farm animals wander in, taken a liking (self-tame), timbergnaw gnawing. A mad animal (single manhunter) joins the storyteller pool in `raids.go`. Messages carry Focus jump links.
- Verified: 10-day stability test keeps populations in the band.
- Limits: raids still use the older storyteller schedule (incident framework is nature events only; no queue, point budget, category); no population policy beyond hunting/slaughter; incidents are Go data.

## SIM-026-animal-breeding-products
Animal breeding, life stages and products · 2026-10-22 · 04d46d084
- Changed: `domain/animal/life.go`, `animal_life.go`: ageing, juveniles (smaller, half portions, follow mother), old-age death, daily mating check 35%, pregnancy/litters, tame young named. Chickens/ducks/geese lay eggs; fertile eggs hatch at 0-45C, frozen ones die. `animal_products.go`: milk, wool, ember oil with a product bar; new Handle work type runs Milk and Shear jobs. Items: milk, wool, egg, fertile egg, ember oil (campfire fuel), wool parka, wool carpet.
- Decisions: taming moved from Hunt to Handle work; old saves migrate.
- Limits: young give no reduced (50%) meat; no pack animals/caravans/training; taming has no skill/wildness/food requirement; job line reads "milk do" for ember oil; wool parka reuses the leather parka layer, tinted.

## SIM-027-postpone-blocked-work
Postponing blocked and unreachable work · 2026-10-22 · fc49cab16
- Changed: `postpone.go`: blocked work (15 s) or unreachable work (1 h; cleared when walkability changes via `grid.Version`) is put off per pawn+job+target. Givers try up to 8 nearest candidates and skip postponed. A pawn stuck behind a non-yielding blocker gives up after 2 s (120 ticks) instead of 10 s; working pawns with the job on an adjacent cell shuffle aside. Player orders: blocked retried 3 times then dropped with message; unreachable dropped with message.
- UI: inspector "put off: ..."; postponement state saved.

## SIM-028-floors-under-furniture-fences-forbid
Floors under furniture, fences and gates, forbid · 2026-10-22 · fc49cab16
- Changed: floors go under any furniture, not on rock, walls, fences/gates or dug graves; builders work from an adjacent tile. Wood and stone fences (autotiled) and fence gates (4 facings): animals can't cross fences, tame animals pass gates, wild can't; colonists and raiders climb fences at x3 step cost; animals never pick food behind a fence; fences cast no shadow.
- Changed: `Stack.Forbidden`: haul, eat, equip, bills, refuel, troughs and burial skip forbidden stacks; raid weapons and bodies drop forbidden. UI: Forbid/Allow area tools, item panel toggle, red map marker, F key (UI-016).
- Limits: pathing doesn't prefer gates over climbing; forced Prioritize on a forbidden item allowed; stacks forbidden mid-haul not interrupted; no stone gate art.

## SIM-029-stone-types-ore-smoothing-stonecutting
Stone types, ore veins, smoothing and stonecutting · 2026-10-22 · f7bce7011
- Changed: `domain/world/geology`: per-cell stone (sandstone, granite, limestone, slate, marble) and ore (steel, silver, gold); `Generate(seed, terrain)` picks 2-3 stones per map in noise regions and grows veins from cells >=2 deep. RNG `randx.New(seed^Salt)`, never the sim RNG. Value noise moved to `pkg/noise`; generation output unchanged (hash-verified).
- Changed: terrain `RockSmooth`, `StoneSmooth`; five chunk kinds, five block kinds, Steel/Silver/Gold; walls and doors can be steel. `StuffStats` per-material HP/work (granite keeps old numbers; others original). `StonecutterTable` with `Cut<stone>` recipes: 1 chunk -> 10 blocks. Mining drops a chunk of its stone; ore cell drops Steel 20, Silver 25, Gold 8 at 1.5x work. `DesignateSmooth` (rock without ore, bare rough stone), builders smooth under Construction after roofs; smooth floors count as floored for rooms.
- Saves: adds `Geo`, `Smooths`, `StructVer`; old saves regenerate geology from a terrain hash; legacy `item.Stone` becomes `GraniteBlocks` everywhere.
- UI/read model: WorldView StoneType/Ore (shared slices rebuilt on `StructVer`), Smooths, Steel/Silver/Gold; `game.SoilLabel`; `SoilView.Smoothable`. Guarantees: `TestSmoothNoFlap`, `TestLegacyStoneMigrates`, `TestWorldGeologyDeterministic`.
- Limits: no fog of war (ore visible, later SIM-051), no mountain roof, no per-kind stack limits, silver/gold value only (no trade), no beauty stat, silver/gold rare on 48x40 maps.

## SIM-030-planet-factions
Planet generator, landing-site parameters and factions (NG-1) · 2026-10-22 · f33df2d86
- Changed: `domain/world/planet`: Goldberg hex globe 10f^2+2 tiles (23,042 full, 12 pentagons), coverage 30/50/100% around a seed-chosen pole. `NewBuilder(Params).Step(units)` in budgeted units across seven phases (shape, elevation, climate, biomes, rivers, factions, roads), each with its own `randx` salt; identical for any step size (`TestStepBudgetDeterministic`); full globe ~35 ms. `TestPlanetGoldenHash` pins seed "amber-hollow" at 30%. Land ~38% (`TestLandShareInRange` 31.5-44.5% over 100 seeds). Queries in `query.go` (`TileAt`, `Site`, `ValidLanding(i) (ok, reason)`, `RandomLanding`, ...).
- Decisions (numbers changed from plan): latitude falloff 28 - 52*|sin lat|^3.0 (`tempLatExp`; plan 1.25), -6.5C/km, setting offsets -12/-6/0/+6/+12; rain knots 0/12/25/38/50/90 deg = 2400/2100/300/800/900/200 mm; moisture decay up to 3x longer within 20 deg of equator, +-25% rain noise; rain shadow x0.65 with a peak within 3 tiles upwind; tropical biomes from 18C (not 20) so no temperature gap; river sources rain >500 and elevation >200 m, 6%/tile. Ocean colder than -8C is Sea Ice land. 12 biomes classified in table order (`Classify`, `TestBiomeClassificationTable`); caves 30% of mountainous, 12% of large-hill tiles; 2-3 of 5 stones from smooth noise. Rivers follow steepest descent (creek, river 4, large 12); roads are an MST plus loops with A* (`TestRoadsConnectSettlements`).
- Changed: `world/site.Site` (biome, climate, hills, river size and edges, coast, lakes 0-3, caves, stones, road edges, latitude); amplitude 6C equator to 22C at 60 deg + up to 4C continentality; day swing 12C dry to 5C wet; peak day 15 north, 45 south. Zero `Site` = legacy map.
- Changed: `domain/faction`: Settlers (industrial, up to 4), Clans (neolithic, 4), Drifters (industrial, 3), Freebooters (industrial, 3, raid points x1.2), Swarm (hidden insect); all original; 660 settlement names, names unique per world. Goodwill: Freebooters/Swarm fixed -100; `Adjust` clamps +-100, keeps last five events; `DailyDrift` one point toward baseline, 60 days of history. Hysteresis: hostile <= -50, calm again >= -10; friendly >= +50, neutral again < +30 (`TestFactionHostilityHysteresis`). Settlements: Poisson disc spacing 8/6/4 (sparse/normal/crowded), capital first, rest within 18 tiles; never on ice sheet/sea ice/extreme desert; no landing within one tile of a settlement.
- Changed (in play, `factions.go`, `story.go`): setup colonies' raids come from a hostile human faction (`faction.PickRaider`, weighted by settlements); none hostile -> pack, manhunters or hive. Letter names the faction, raiders get generated names, Neolithic factions field no rifles. A repelled raid costs 5 goodwill (`raidRepelled`). `Simulation.Factions()` feeds the Factions window. Legacy colonies have no factions ("Raiders").
- Open (flag): arid shrubland is ~40% of land over 100 seeds (lead's figure): `Classify` falls through to it for >=18C with <1900 mm rain and for cool tiles with 250-550 mm. Candidates: lower the 1900 mm rainforest line, add a warm seasonal-forest rule for 550-1900 mm. Not changed.
- Limits: movement difficulty informational; goodwill changes only by raid outcomes and drift (no gifts, quests, trade, aid); no territory, settlement attack maps, mechanoid/ancient factions; trade, caravans, world travel, quests out of scope (plan §0.5). `BenchmarkPlanetBuild100` checks the 1.5 s budget.

## SIM-031-storyteller-difficulty
Storytellers, difficulty and threat points (NG-1) · 2026-10-22 · f33df2d86
- Changed: `domain/storyteller` `Plan(profile, state, now, rng)` -> `Decision` (when, category, points percent, optional second threat), deterministic via `randx`. Orla Vane `steady` (Cycle: first threat day 6, gap 3-5 d, 8 days pressure/5 quiet, points +4%/cycle up to +40%); Bram Holloway `hearth` (Calm: day 10, gap 9-15; after a death the pending threat slips 6 days and shrinks to 70%, once per threat via `OnLoss`/`Eased`); Kestrel Quay `gambler` (Dice: day 4, daily roll 07:00, 12.5% +2.5 per quiet day, 5% double threats, points 60-150%); hidden Legacy (day 5, gap 5-9) makes exactly the pre-NG-1 draws (`TestLegacyPlanMatchesConstants`). `storyteller.Spark` for the sparkline; portraits `ui_portrait/<key>`.
- Changed: `domain/difficulty`: Wanderer, Gentle, Settled, Seasoned (neutral, all 100), Grim, Merciless, Custom; threat scale 5/35/65/100/145/200, peaceful days 0/12/8/6/4/3, adaptation off for Wanderer then strength 60/80/100/100/100. 19 settings with slider limits; `Match`, `Encode`/`Decode` (`key=value,...`, payload of `SetDifficulty`). `ThreatPoints`: 25/standing colonist + wealth curve (wealth/50 to 20,000, slower after, slower again past 100,000) x threat scale x (100+adaptation)/100; time escalation x(100+rate*years)/100; "ignore wealth" = 80 points/year; floor 35.
- Decisions: adaptation (`Adapter.Day`): +1/day without a loss (cap 120), -25 on a loss day (floor -40); target `days/2-10` clamped -30..+40 scaled by strength; live value moves <=2/day and only when >=3 from target (dead band, `TestAdaptationNoFlap`).
- Changed (in play, `story.go`, `weather_ng1.go`, `setup.go`): a Setup colony uses the storyteller `Plan`; nothing before max(first threat day, peaceful days). No-Setup colonies keep the old `raids.go` scheduler bit for bit (goldens unmoved). Threat die filtered: ruled-out threat or raid with no hostile faction becomes pack/manhunters/hive. Raid points = ThreatPoints x plan% x faction multiplier; size cap 8 + map area/40,000, max 24. Difficulty hooks: yields, research speed (`diffScale`), mood offset, pain tolerance (stats rebuild on change), infection chance, animal revenge, predators hunt colonists, extreme weather.
- Decisions: extreme weather: each in-season day (rolled at midnight) 6% chance of heat wave (+12C summer) or cold snap (-14C winter) lasting 2-4 days; no new one for 3 days after (min duration + cooldown); letter and chip.
- Changed: `command.SetStoryteller` (takes effect at next scheduling decision) and `SetDifficulty` (preset key or `custom:`+encoded; invalid ignored), both ignored without a Setup; Options > Story drives both. `Simulation.Story()` read model.
- Open: `PositiveBias` (20/45/30) unread (storytellers differ only in threat pacing); Disease frequency slider has no effect (no disease events; tooltip says so); player faction naming; death/downing difficulty option; storytellers schedule only raids, packs, manhunters, hives.

## SIM-032-pawn-identity-skills
Colonist identity, skills in play and character creation (NG-1) · 2026-10-22 · f33df2d86
- Changed: `pawn/skill`: 11 skills, levels 0-20, Neutral 7, `SpeedPermil = 1000 + 60*(level-7)` (level 20 = 1780, level 0 = 580); XP to next `20000 + 6000*level`; passions learn at 35%/100%/150%; above level 10 decay `500*(level-9)`/day, dropping a level only past half the previous cost and landing at 3/4 (`TestSkillXPAndDecayNoFlap`). `pawn/stat`: 14 stats in fixed layer order base, body, age, conditions, traits, difficulty.
- Changed: `pawn/trait` 19 original traits in conflict groups (Gentle cannot do violence, +3 Social); `pawn/backstory` 59 original backstories with skill changes, incapabilities and tech tags (Go tables, not data files); `pawn/look` (sex, 3 builds, hair, beard; `look.Legacy(id)` reproduces the old renderer look, `TestLegacyLookMatchesRenderer`); `pawngen` deterministic from (seed, slot, roll): ages 14-68 (mode 30; 8% teens with childhood backstory only), nickname 35%, 10% clan backstories, 30% major passions, 12% condition, beard 45% of men.
- Changed (in play, `skills.go`, `bio_build.go`, `bio_view.go`): every colonist holds a bio, live skills and stat sheet (rebuilt on birthday or difficulty change). Job progress follows skill, WorkSpeed and trait factors; XP saturates at 4000/skill/day; shooting/striking grants 120, a finished chat 40 Social; yields scale with skill; harvesting below Plants 4 may fail. Fussy doubles bad-room thoughts, Roughneck quarters them; passion work gives joy, major passion "loves their work".
- Decisions: incapable work is never assigned: priority forced 0, `SetWorkPriority` ignored with toast "Incapable: <reason>", pacifists refuse to strike (`TestIncapablePriorityIgnoredAndOrderRefused`, `TestViolenceIncapableNeverStrikes`). Legacy/named pawns are neutral (all skills 7, no traits, `TestNeutralPawnTimingsUnchanged`).
- Changed: `CandidatePool` (`candidates.go`) exists before any Game; each candidate keeps its generation slot so rerolls stay deterministic; reroll (keeps edited names), randomize, move, swap, rename (nickname required, unique, <=16 chars), undo; pool = People + 5 by default; `Team()` warnings (nobody can fight, no doctor/cook/grower/builder/researcher, all very young/old). `Simulation.Bio(id)` feeds the Bio tab.
- Saves: bios and skills in save version 2 (`TestImportBiosMigratesLegacy`). Golden hashes re-pinned once (`TestScaleGolden`) for the new saved state only; other fields checked identical against `4d68b9aa`.
- Limits: male/female builds read weakly at 1x and beards vanish facing north (art follow-up); no Artistic skill, craft quality, relationships, family, per-pawn faction; traits/backstories not moddable; stat layers stop at traits+difficulty; age only by birthdays (60-day year), no chronic conditions.

## SIM-033-water-sites
Landing-site maps and water (NG-1) · 2026-10-22 · f33df2d86
- Changed: terrain appended (saved codes keep meaning): shallow water (wadeable, walk 300%, wetness 1000, unbuildable), deep water (impassable), mud (140%, wetness 800, fertility 70), dirt track (90%, not farmable); `terrain.IsWater`.
- Decisions: `GenerateSite(seed,w,h,biome,site.Site)`; the zero `Site` equals `GenerateBiome` (`TestZeroSiteMatchesLegacy`) so goldens don't move; each pass has its own `randx` salt. Hills thresholds per level; caves in large hills/mountains 9 tiles from centre; coast 1-2 edges (~18% deep band, 5 shallow tiles, beaches); rivers creek 3 / river 9 (core 3) / large 15 (core 7) wide with fords every ~45±10 tiles; up to 3 lakes radius 6-14; wet ground within 4 tiles of water (marsh 48% falling to 12%, else rich silt 40% within 3); 2-wide dirt track toward centre.
- Decisions: reachability pass: water cleared within 7 tiles (`WaterGuard`), then the start must reach >=70% of walkable land (`ReachShare` 700 permille) by carving 3-wide (8 tries) then 5-wide crossings (32), so the player can't be walled in. 500x500 site ~65 ms (`BenchmarkGenerateSite500`).
- Changed (in play, `water.go`, `placement.go`, `filth.go`): "Can't build on water" (`ReasonWater`); `dryWalkable` for spawns/items (animals and raiders may cross shallow, never spawn there); water/mud/marsh/sand/ice slow steps by walk cost unless floored; wading washes half the carried dirt and gives Soaked; mud picks up dirt faster. `groundExtra` adds (walk cost-100)/10 only through `crowdCost`, i.e. crowd repaths; ordinary paths ignore mud/water cost on purpose (performance), so colonists may wade.
- UI: `water_draw.go`/`water.kage` ripple and caustics on the presentation clock (four baked frames without the shader; Reduced Motion freezes), shore foam, 16-mask mud/track edges; pod fall/dust effects; ambience loops `amb.water_river`/`amb.water_shore` by on-screen share (damped indoors); baked into 16 px/4 px chunk tiers (ENG-001).
- Limits: no bridges, fishing, water as resource; deep water impassable only (no swimming/drowning); rivers run edge to edge, not the world tile's course.

## SIM-034-components-medicine
Components, industrial medicine, tool cabinet, hospital bed, pod shells (NG-1) · 2026-10-22 · f33df2d86
- Changed: `item.Components`: workbench recipe (12 steel -> 1, 1500 work, Crafting, min Crafting 6, behind Fabrication) or buried machinery ore (`geology.Machinery`, site maps only, 2 per tile). `item.IndustrialMedicine`: 2 herbal -> 1, 800 work, doctor job, min Medicine 4, behind Pharmacology. Recipes now name the trained skill (`Learn`) and optional `MinSkill` (below it, pawns skip the bill). Research Fabrication (5000, needs Gunsmithing) and Pharmacology (4000, needs Herbalism); research saved as growable list, `research.V1` loads old saves.
- Changed: frames may need components, delivered by haulers (`supplyKind`) after the material; cancel returns them, deconstruct returns half rounded down. Tool cabinet (steel only, 30 steel + 1 component): each within 6 tiles of a bench adds 6% bill work speed, max two. Hospital bed (45 wood or steel + 2 components, 1x2): patient gets 115% of tend healing and half the wound infection chance; rescuers prefer it. Beds, double beds, tables, chairs can be steel. `construction.PodShell` per drop pod, deconstructs to 6 steel.
- Decisions: care tiers untreated, self, tended, herbal, industrial. Doctor uses industrial for injury severity >=50 or any infection, herbal otherwise, each falling back to the other (`TestMedicineChoiceNoFlap`). Industrial heals a tend 2.5x (herbal 2x, self half), infected 1% (herbal 3%), -12 infection severity/hour (herbal 9).
- UI: architect cards show components in tooltips; scenarios can forbid buildings (disabled with reason).
- Limits: silver not currency (no trade; D13 has no effect); no per-patient medical policy; cabinet speeds only workbench bills; no machining bench; no power.

## SIM-035-animal-reactions
Animal reactions to being hit and predator charges (PLAY-010) · 2026-10-23 · 7c042c6e3
- Changed: every animal hit reacts at any size (`hurt` -> `animalHurt`): fighters (territorial, predator, pack predator, legendary) fight back, everything else flees. An animal that can't fight back (flee type, tame and hit by its colonist, badly hurt, attacker out of reach) flees on every hit to the farthest reachable spot (straight away -> farthest within 8 tiles -> neighbour); penned tame animals flee within the pen (`animal_flee.go`). Pets (dogs, cats) are flee class.
- Changed: wild predators may charge colonists/raiders within 3 tiles: roll every 60 ticks, 30%, then 600-tick cooldown; never while asleep, fleeing, hurt, tame or enraged; off on difficulties without "predators hunt colonists" (Wanderer, Gentle). Tame animals flee threats too; a predator that can't reach its target over fence/water is ignored; no revenge on an unreachable foe. Domain: `ChargeSince`, `NextCharge`, `Fighter`, `Charger`.
- UI: log "Charged by a <species>"/"Charged <name>"; inspector action "Charging <name>".
- Save/golden: goldens re-pinned (six golden colonies: no colonist dies, two take a charge hit). `TestPredatorBlockedByFence` now designates the pen.

## SIM-036-colonist-flee
Unarmed colonists flee animals (PLAY-010) · 2026-10-23 · 7c042c6e3
- Changed: an undrafted colonist with no weapon (or unable to do violence) flees any animal coming for it (hunting, charging, manhunting, retaliating) at <=12 tiles and keeps fleeing until 16 (hysteresis; 13 tests incl. a no-flap at the 12/16 boundary). Runs right after mental breaks and before queued orders, so it interrupts orders, work and sleep; an interrupted forced order returns to the queue front. Drafted colonists stay under player control; armed colonists keep the old 4/8-tile reach and fight back.
- Decisions: `fleeGoal` tries straight away, then +-45, then +-90 degrees, each reachability-checked and preferring the home area.
- UI: `PawnView.Fleeing`, job line "Fleeing from <creature>".

## SIM-037-walk-through
Colonists and tame animals walk through each other (PLAY-010) · 2026-10-23 · 7c042c6e3
- Changed: a step into a cell held only by colonist/tame "partners" just goes (own partner layer in `occupancy.go`); wild animals and raiders still block. In combat (`inCombat`: drafted, attacking, fleeing) a colonist never ends a walk on another colonist's tile; outside combat colonists may share a tile and the unstack sweep splits pairs only when one is in combat. `crowdCost` doesn't price partners for a partner mover.
- Decisions: supersedes the colonist part of SIM-014's "every standing creature owns its cell". Blocking tests moved to wild animals.

## SIM-038-wild-plants-grass-growth
Seven wild ground plants and growing grass (PLAY-010) · 2026-10-23 · 7c042c6e3
- Changed: bramble thicket (walk 150%), ground moss, desert rosette, ball cactus, elephant ear (110%), fire lily, dandelions (our names/numbers); per-biome, browsable by grazers (not cacti), never block building, cleared by it. Grass regrows: a grazed or cut tile returns as shoots (15%) and fills to 100% in ~3 days at 15C; snow/frost pauses it; grazers prefer fuller grass, meal scaled by growth (min 30%), tiles under 30% left alone unless nothing else near. Rainforest jungle trees 95‰ and berries 25‰ keep the total in 140-180‰.
- Save/read model: `game/grass.go` lazy per-chunk growth layer; `WorldView.GrassGrowth`/`GrassVer` (content fingerprint); saved as `GrassGrowth`.
- Limits: the first path plan has no per-cell cost (adding it would move every golden), so routes avoid brambles only on replans. Five `goldenLegacy` site hashes re-pinned (plants only); `TestTemperateGenerationUnchanged` holds.

## SIM-039-body-surgery
Body parts, wounds, prosthetics and surgery (ENG-003 M1) · 2026-10-24 · 141eccc86
- Changed: humanlike body-part tree (brain, eyes, ears, nose, jaw, torso, heart, lungs, stomach, arms, hands, legs, feet); per-part wounds cut/bruise/bite/scratch/burn/surgical with tended flag and pain; 10% permanent scar on cut/bite/scratch/burn; non-vital parts can be destroyed, vital damage spills so downed (70) and death (100) thresholds are unchanged. Capacities (consciousness, moving, manipulation, sight, hearing, talking, breathing) feed MoveSpeed, WorkSpeed, ShootAcc; consciousness 0 downs. Face scars/missing face parts feed opinion.
- Changed: prosthetics (peg leg, prosthetic leg/arm, wooden hand, denture, hearing trumpet) crafted at the workbench. Operation bills (install/remove implant, amputate): success from skill, bed, medicine; minor and catastrophic failure; anaesthetic 1 h; botched operation leaves an opinion memory. Old saves migrate.
- Decisions: numbers from the published RimWorld wiki (ENG-003 user decision).
- UI: Health tab with capacities, pain, parts, operation queue add/remove (UI-022).
- Open: bleeding/blood loss, disease framework, medical care policy, operation reorder; no damage types/armor, organ transplant, bionics; infection still one record per pawn.

## SIM-040-drugs
Drugs, tolerance, addiction and policies (ENG-003 M2) · 2026-10-24 · 141eccc86
- Changed: beer, ashleaf tea, spark tablet; alcohol bands warm, tipsy, drunk, hammered, hangover; tolerance, addiction chance/need, withdrawal, recovery after 10 days. Craving starts and stops at different thresholds. Policies No drugs / Social drinking / Unrestricted, set on Assign tab (F3). Traits Abstainer, Indulgent, Hedonist shape autonomous use. Brewing bench (beer from brewgrain), drug table (tea, tablets); research Brewing, Herbal remedies, Prosthetics; crops brewgrain, ashleaf.
- Decisions: wiki numbers (ENG-003). UI: Assign tab selector, drug lines in Health tab.
- Open: policy editor with schedule, thresholds, carry quantity; no smokeleaf, go-juice, hard drugs, per-drug scheduling UI, overdose.

## SIM-041-social-core
People registry, family graph, opinions, interactions, memories, grief (ENG-003 M3+M7) · 2026-10-24 · 141eccc86
- Changed: registry includes off-map relatives and the dead; kin from parent links; starting families seeded. Sparse directional opinion edges = relation bonus + traits + beauty/disfigurement + memories, clamped +-100; compatibility from pair hash and age. Interactions every 60 ticks, staggered, nearby same-room pawns: chitchat, deep talk, slight, insult, kind words (Gracious); friend/rival with hysteresis; memories stack x0.9. Smoothed mood +12/h up, -8/h down. Social recreation at tables/campfires; chat partner mirroring.
- Changed: grief runs through the relationship graph (incl. off-map relatives and killer opinion; Cold-hearted exempt); colonist died -3 for 6 days; rescue, harm, botched-surgery memories; history log capped 500. 20 new traits (Striking, Fair, Homely, Grotesque, Gracious, Cold-hearted, Bloodthirsty, Austere, Asexual, Gay, Bisexual, Grating voice, Rasping breath, Distrusts men/women, Idle, Abstainer, Indulgent, Hedonist, Augment fan, Natural purist).
- Decisions: wiki numbers (ENG-003). UI: UI-022.
- Open (then pending M-items): romance M5, social fights M6 (chance table was domain-only), prisoners M8, caravans/kidnap/visitors/raider relatives M9, animal bonds M10 (all done in SIM-042..047).

## SIM-042-romance-weddings
Romance, weddings, affairs, breakups, partner beds (ENG-003 M5) · 2026-10-24 · c34c0c869
- Changed: lovers from attraction (orientation traits, age >=16, no blood kin); rebuffs leave memories/moods. Affair: cheated memory -70, mood -20; breakup ex-lover -50/mood -15; divorce ex-spouse -70/-20. Proposals -> engaged, or rejection (-30 opinion, mood -18, 10-day cooldown). Weddings at marriage spot (fallback party spot, campfire, home): gathering, ceremony, reception; one marriage across save/load; +40 mood married, +20 attendees, surname adoption 50%. Parties +8. Partner beds (double bed): sleeping alone -4, partner-opinion mood +-10, lovin' +8 (no pregnancy). Different start/stop thresholds and cooldowns prevent flapping.
- Decisions: wiki numbers (ENG-003). Determinism: seeded RNG only. Known issue then: a pawn that never moved gained a default Motion record on load, so StateHash differed after save/load (fixed in SIM-048).
- Open: manual Romance button (Biotech, excluded by design).

## SIM-043-social-fights
Social fights (ENG-003 M6) · 2026-10-24 · c34c0c869
- Changed: fight chance from insult 4% / slight 0.5%, multiplied by bloodthirsty, hunger, alcohol, withdrawal factors. Uncontrolled fists-only brawl of at least 420 ticks; non-violent pawns don't strike; ends on down, death or recovery. Cathartic +38 / angering -22 opinion per participant, rolled independently; harmed-me memory. Drafting blocked during a fight; pair cooldown prevents repeats.
- Decisions: wiki numbers (ENG-003). Open: insulting-spree mental break (10.1 destructive breaks).

## SIM-045-prisoners
Prisoners: capture, life, warden work, recruit, release, execute (ENG-003 M8) · 2026-10-24 · c34c0c869
- Changed: downed raiders are captured and carried to prisoner beds; arrest has a success chance, failure triggers a resist fight. Prisoners have needs, Imprisoned mood, are room-bound; doctors tend in place. Warden work type delivers food (hysteresis) and visits that reduce resistance by opinion and mood factors x social impact. Recruit keeps relations (recruited-me +20 for 120 days); release +15 goodwill; execute applies relatives' killer opinion.
- Decisions: wiki numbers (ENG-003). Open: prison break, warden chat history list.

## SIM-046-world-caravans-areas
Runtime world, caravans, visitors, allowed areas (ENG-003 M9a) · 2026-10-24 · c34c0c869
- Changed: planet rebuilt at runtime with a route; caravans (form dialog, walk to map edge, travel at slowest Moving speed, visit a friendly settlement 1 day, return; food days; in-caravan chitchat/deep talk; death on the road with grief). World tab (F8) with globe and caravan list. Visitor groups from friendly/neutral factions wander half a day, socialise, leave; wanderers join. Allowed areas (up to 10): paint, rename, assign via Assign tab and area manager; pathing and job filtering respect them, drafted pawns ignore them.
- Decisions: wiki numbers (ENG-003). Open: caravan animals, cargo, trade; area overlay and delete; transport pods; kidnapping/raider relatives (M9b).

## SIM-047-animal-bonds
Animal bonds, masters, training, veterinary care (ENG-003 M10) · 2026-10-24 · c34c0c869
- Changed: bond chances per 2500 ticks: tame 1%, training 0.7%, tending 0.4%, proximity 0.1%. Training obedience, guard, release with steps, cooldown and decay; obedience makes the animal follow a drafted master. Veterinary tending and natural healing for tame animals. Bond moods +5 master / -3 not master; died/lost -8, given away -10, released -5; cold-hearted exempt. Master death: animal confused or 10% manhunter. Mother/offspring shown.
- Decisions: wiki numbers (ENG-003). Open then: guard/release behaviour (done in SIM-048).

## SIM-048-social-polish
Social Impact scaling, grief labels, guard/release, letters (ENG-003 polish) · 2026-10-24 · 5791b03b8
- Changed: `OpinionMemory.Scale` (permille, clamp 200..1500; 0 reads as 1000 so old saves are unchanged) from initiator's Social skill + Talking capacity (`social.ImpactScale`). `mood.Memory.Label` for relation-specific grief ("my wife died"). `InteractCtx` gains Affair and Together (breakup weight uses affairs, proposal weight time together). Letters for lovers, engagement, rejected proposal, breakup, divorce, affair, social fight, wedding, party, relative among raiders, kidnapped, recruited, bonded animal died.
- Changed: guard/release training (`train_animals.go` masterDuty, 60-tick cadence): a guard defends its master against hostiles within 5 tiles or the master's attacker in the last 120 ticks; a release-trained animal attacks the drafted master's target. Face scar drawn in HUD portrait and falling frames.
- Fix: `Load` no longer adds a default Motion record (StateHash mismatch after save/load for never-moved pawns).
- Determinism/saves: goldens re-pinned once for Scale; additive (Scale, Label), SaveVersion unchanged. Bench TickGrown +2-3%.

## SIM-049-usable-surgery
Usable surgery: replace healthy parts, new implants, waiting reasons · 2026-10-24 · 179a80093
- Changed: implants can replace healthy parts (the operation removes the part and those under it, then fits the implant); missing-ancestor block (no wooden hand under a missing arm). New implants prosthetic foot (750‰), glass eye (Prosthetics research + Crafting 4) and wooden nose; glass eye and wooden nose are cosmetic (count as artificial, clear the missing-part thought, disfigurement still follows the spec).
- Contracts: `game.OperationOptionView` and `HealthView.Options`; `OperationView.Status`/`Blocked`; `PartView.Part`/`Implant`; dev hooks `DevLosePart`, `DevSpawnImplants`. `lieDownForCare` prefers a free hospital bed.
- Decisions: waiting reasons in order: "needs research" (only when none in stock), "no X in stock", "the X can't be reached", "needs another colonist who can doctor", "waiting for the patient to lie down", "a doctor is on the way"; `Blocked` true for the first four. No flap: a patient lies down only when its operation can start; the 6 h give-up stays. Implant numbers are our own tuning (spec gives none).
- UI: Health tab picker lists every valid operation, unavailable ones greyed with the reason, queued ones show what they wait for; workbench "+ Add bill" pick list replaces per-recipe buttons (overflowed); dev menu "Lose a part..." and "Spawn implants"; three item sprites.
- Saves/determinism: `surgerySave.Hosp` additive; new item/recipe kinds append-only; no golden moved.
- Open: bionics tier, risk preview; no organ transplants, room-cleanliness factor; a bench with >~7 bills can crowd the Add bill button.

## SIM-050-mapgen-pipeline
Staged field-based map generator (PLAY-013, MAPGEN-2) · 2026-10-24 · e5f89f8e6
- Changed: landing-site maps come from `domain/world/mapgen`: continuous fields (warped multi-scale elevation with uphill mountain mass, fertility, wetness, stone regions), landforms (rock by hilliness share, tapered branching caves, coasts, meandering rivers with banks/floodplains/canyons/fords, lakes), then the discrete map (terrain precedence, biome patches, ore lumps, stone debris, A*-routed smoothed roads, scored landing spot, small ruins, steam vents, clustered vegetation/grass); validation with local repair last. Each step has its own `randx.Derive` stream. Legacy `generation.Generate/GenerateBiome/GenerateSite` frozen for legacy colonies, scenarios, pins. Plan `.agents/handoffs/PLAY.md#play013_plan`. `Setup.GenVersion = mapgen.Version` (2). Landing spot saved (`saveState.Landing`) and used for arrival, Home, wildlife, messages.
- Changed: wildlife (`wildlife_habitat.go`, Setup games only): herds start in reachable open land 25+ tiles from landing; arrivals walk in from a reachable edge; no domestic/pet species spawn wild. `site.Site` gains `Uphill/HasUphill`, `Rain`, `CoastNear`, `Settled`. Tools: `pkg/noise2`, `randx.Derive/Float64`, `cmd/mapgen` (PNG per field, `-grid`, `-sweep hills`, `-tile`), dev "Map fields" overlay.
- Decisions (ours, tuned in cmd/mapgen): rock share Flat 4%, Small 10%, Large 22%, Mountainous 42% (±25%); rough-stone 4%, gravel 6%; mountain bias 0/0.08/0.18/0.38; caves in rock bodies >= max(200, 450*Scale^2), one tunnel per 900 rock cells (<=5), width 4.5-7 tapering to 1.3, branches 9%/step; rivers widths 3/7/12*sqrt(Scale), fords at least every 86 cells; coasts 14-40*Scale; debris 0.003*density; landing spot >=70% open land within 12, nothing wet/rock within 4; reachability >=70% of soft land. No reference constants copied.
- Perf: whole pipeline 0.30 s at 500^2, 0.08 s at 250^2 (budgets 2.0/0.6 s).
- Determinism: Setup-game hashes include the new map; legacy and scenario goldens unmoved.
- Open: road re-routing in validation (done SIM-053), moss in damp caves, sealed "ancient danger" rooms (by decision; later SIM-054), bridges (SIM-053). Steam vent has no power use (no power system yet, `PowerReady` future).

## SIM-051-fog-of-war
Fog of war (PLAY-013, MAPGEN-2 §5) · 2026-10-24 · e5f89f8e6
- Changed: new colonies start fogged. Everything reachable from the landing spot over walkable ground is revealed plus each tile's neighbour ring; deep water doesn't stop the reveal; sealed caves and rock interiors stay hidden. A tile becoming walkable next to a revealed one (mining, deconstruction, moved building) floods and reveals only the newly connected area; 20+ tiles posts "A cave opened up." Legacy colonies/old saves have no fog.
- Decisions: placement and orders refuse fogged tiles ("unexplored"), except mining designations (any hidden tile, so marks don't leak caves; reveal drops marks on non-rock); cut, harvest, forbid, smooth, handling designations skip hidden things. Hidden items/plants/animals/buildings/frames are left out of read models (simulation still sees them).
- Read model/UI: `WorldView.Fog`/`FogVer` (copy-on-write); `fog_draw.go` near-black #0B0B0E layer, 1-tile soft edge, baked per 32-tile chunk; minimap black; selection skips hidden; hover "Unexplored"; camera starts on `WorldView.Landing`.
- Saves: `saveState.Fog` (absent = no fog); StateHash skips it for fogless colonies. Cost: 400-tile cave reveal ~15 us.
- Limits: zone, Home, plan, roof painting and stains are not filtered on hidden tiles.

## SIM-052-cave-plants-light
Simulation light level for plants; duskmoss (PLAY-014, MAPGEN-3 §1) · 2026-10-24 · b6b0ecfd5
- Changed: plants see a per-cell light level: daylight on unroofed cells, lamps (0.7, radius 6), campfires (0.6, radius 5), glow plants (duskmoss 0.1, radius 1); rock, walls, doors block. Cached in `game/light.go`, rebuilt only on structure, glow-plant or load changes. Moss (`MinLight` 35) grows only on lit cells.
- Changed: duskmoss (new original species): grows in the dark on cave stone (`plant.CaveFertility` 40), very slow (~40 days), slows walkers (125%), glows faintly; withers in light from 0.45 with harm stopping only below 0.35 (hysteresis), losing 8% of full growth/hour; removed when withered with one notice per patch per day. Other species unchanged.
- Changed: mapgen `steps/vegetation/cave.go`: dark cave floor gets duskmoss clusters of 1-4 cells (~15-25% coverage); lit cave mouths (within 2 of open sky) may get moss.
- UI: `PlantView.NeedsDark/NeedsLight/Withering`; faint cool glow, inspector light lines.
- Limits: a campfire lights whether or not burning (avoids grid rebuild per fuel change; SIM-060 later made fuelled-only); crops and trees have no light needs yet (checklist `[-]`).

## SIM-053-roads-bridges
Road bridges (mapgen) and player bridges (PLAY-014, MAPGEN-3 §2-3) · 2026-10-24 · b6b0ecfd5
- Changed (mapgen): roads cross river/lake water on straight 2-wide wooden bridges bank to bank, snapped to the river normal, never over sea (`Result.Ocean`); fords gone for roads. Routing: water 12/cell, diagonal water steps 2.2x, vault footprints and open sea impassable, rock 7+ deep impassable first try (retry cuts at 25). `Route`/`Lay` exported. Validation `roadsConnected` re-routes a road a repair broke ("road N re-routed") or removes it; bridges count walkable.
- Changed (game): foundation layer (`construction.LayerFoundation`, kind `Bridge`, wood 6, work 110, refund 3) behind Bridges research. Placeable on water, marsh, mud, never open sea (saved ocean bitset; legacy maps none). `cellWalkable` at every walkable-bit site; bridge walks 100%, never Soaked, counts as dry footing; stockpiles allowed, growing zones not. Load: light only (`Requires` vs `Provides`): wooden walls/doors/fences/furniture fine; stone/steel walls, floors, carpets, graves refused ("needs firmer ground than a bridge"); on marsh/mud the ground's own strength still counts.
- Decisions: over deep water a span grows from the shore; haulers skip unreachable bridge frames (avoided a pick-up/drop loop). Deconstruction refused while in use. A destroyed bridge breaks what needed it (no refund); items on deep water sink (one message per bridge); pawns/animals move to nearest walkable. Generated road bridges are finished decks.
- UI/art: `structure/bridge_00..15` (mask N1 E2 S4 W8), water shadow on open edges, Architect Bridge tool, inspector and hover lines "Bridge over deep water / marsh / mud".
- Limits: no fire system at the time so bridges don't burn (later SIM-058); plank direction mismatches where horizontal and vertical spans meet.

## SIM-054-sealed-vaults
Sealed ancient vaults (PLAY-014, MAPGEN-3 §4) · 2026-10-24 · b6b0ecfd5
- Changed (mapgen `vaults.go`, after start before ruins): attempts round(W*H/250^2) clamped 1-3, none on sea ice. Layout k=1-4 casket bays (3 wide, 1x2 caskets) in 1-2 facing rows + contents strip + 1-cell wall ring (footprints 6-15 x 9-12). Placement: anchor is rock, >=70% of footprint plus 2-cell ring rock, >=30 from landing, >=12 from other vaults, no caves/water/roads/features. Carve: smoothed stone floor, walls of local stone blocks (70%) or steel. Occupants: Grateful 30%, Hostile 25%, sleeper+crawler 15%, Beetles 10%, Empty 10%, Remains 10%; loot 3-6 stacks (components, medicine, silver, gold, rifle 35%, rare prosthetic/eye/tablet 15%); 2-6 defender spots.
- Changed (game `vault.go`, `casket.go`): states Generated -> Warned -> Breached -> CasketsOpened -> Cleared. Defenders are dormant hive insects, (points+25)/50 clamped 2-6, brutes <= n/3; none at threat scale 0, Wanderer preset or hive swarms disabled; dormant animals skip thinking; hurting wakes. Colonist within 2 fires the "Ominous walls" letter once. Fog reveal of the interior wakes defenders (danger letter) but doesn't open caskets. Open casket order (reuses deconstruct job, 250 work; "No colonist can reach it") or any damage opens every casket in the vault: grateful sleeper joins, hostile is a one-person rifle raider, crawler and beetles hostile, remains are bones with an old melee weapon; opened shells salvage 20 steel. Ancient pieces aren't wealth, Home anchors or repaired. Cleared once caskets open and no threat remains.
- UI: `vault_draw.go` casket sprites, inspector button with `BuildingView.OpenReason`, `sfx.casket_open`, "Dormant" label, dev Map-fields layer; `cmd/mapgen` draws vaults.
- Limits: no mechanoids, hive insects are the only defenders.

## SIM-055-allowed-areas
Allowed areas rebuilt to the user's spec (PLAY-015 §1) · 2026-10-24 · 0b1389229
- Changed: `world/area` stable never-reused ids, palette colours, `Delete`, `Invert`; old saves get ids 1..n. Assignment refs: area id, Home (-2), Unrestricted (-1), for colonists and tame non-penned animals; every area command carries the id. Delete resets users with "N pawns are now unrestricted".
- Decisions: soft rule: destinations must be inside (work targets, haul destination, food, beds [none inside -> sleeps on floor inside], recreation, wander, hunting [given up after prey is out 600 ticks]); paths may cross outside at +300%/cell plus fire avoidance (no area and no fire: grid query unchanged). Overrides: drafted, mental break, downed, forced orders, fleeing, attack, rescue/tend/surgery, warm, hide, animals following a drafted master. Cleaning, repair, firefighting need Home and the pawn's area. A bill ingredient outside the area defers the bill.
- Saves: world_state save v2 with `HuntOut`. Limits: no trained hauling animals exist to restrict.

## SIM-056-pens
Pens as markers with enclosure, roping and settings (PLAY-015 §2) · 2026-10-24 · 0b1389229
- Changed: `construction.PenMarker` (10 wood, Architect > Misc). Enclosure = flood fill from the marker over cells a penned animal can stand on, bounded by fences, closed gates/doors, walls, rock, deep water, blocking buildings; open if it reaches the map edge or exceeds 2,500 cells; lazily recomputed; alert "Pen N is open".
- Changed: 29 penned species (`animal.Roster.Penned`: farm animals, tamed grazers, herd birds) can't pass gates/doors unless roped. Rope job (Handling): 60 work; animal follows the handler and is released in the closest reachable enclosed pen accepting its species and sex. `SetPenSettings`: species and sex filters, auto-cut plant list (every 2 h), food figures (grazing vs demand per day, plus trough stock).
- Decisions: unpenned livestock roams (radius 30), alert "Unpenned livestock"; after 3 days unpenned an animal 40+ cells from Home has 12% daily chance to wander off; roping/penning resets the clock (saved in `PenAnimals`). Roaming livestock avoid wander targets toward visible predators (keeps flee from flapping). Painted pen zones become markers on load.
- UI: Animals-tab "follow drafted" toggle (`SetAnimalFollow`, `animal.State.NoFollow`, zero = follows as before). `DesignatePen` still paints inert zones for old saves; Architect tool gone. Limits: Master dropdown lists all colonists, not only capable handlers.

## SIM-057-home-snow
Home auto-expansion and per-tile snow (PLAY-015 §3-4) · 2026-10-24 · 0b1389229
- Changed: Home auto toggle (`SetHomeAuto`, saved `NoHomeAuto`, default on; "Auto" button in the top strip) grows around finished buildings (incl. pen markers) and newly painted stockpile/grow/pen zones (2-cell margin); repairs limited to Home; Zone tools "Expand/Clear home area"; "Build/Remove roof area" moved to Zone; persistent `roof.AreaBuild` re-plans roofs after collapse.
- Changed: snow is per-tile depth 0..255 on open-sky cells (not walls/water/rock; roofed 0); builds by weather intensity, melts by temperature, halves in rain; each chunk updates once per climate interval at its own tick from weather and clock alone (loads exact); walking slowed above 76, grass stops regrowing from 26; `SnowDepth` = average where snow lies; old saves' single depth fills every open cell.
- Changed: snow clearing area (cyan): `ClearSnow` job (Cleaning, after ordinary cleaning) takes cells >=38, 20-80 ticks work, respects allowed areas.
- Limits: grass tufts still fade by the map-wide snow figure (per-tile needs a grass rebake). Bench 0.75 ms per full climate step at 500^2.

## SIM-058-fire
Fire: spread, damage, firefighting, danger (PLAY-015 §5) · 2026-10-24 · 0b1389229
- Changed: `world/fire` pure rules (our numbers): growth +4/step with fuel, -6 without; spread from size 35 at flammability*size/100*0.06, x1.5 downwind, x0.2 into roofed cells; outdoor rain/snow >=0.3 takes -12/step. `fire.go` every 30 ticks, per-chunk lists. Ignition: lightning (one strike per 2-hour slot in storms, stateless schedule, `WorldView.Strikes`), bursting animals, dev "Start fire". Damage: buildings 1+size/12 per step (a burnt-down building leaves no refund or blueprint; bridges use the collapse cascade), plants/grass/items burn, pawns Burn wounds (`CauseBurns`), animals Injury, ash stains. A size-50 fire heats its room like a campfire (outdoors 5x5). Rolls hashed from seed, tick, cell so the main stream never shifts.
- Changed: Firefight is the first work type, on by default (priority 3); `FightFire` job -10 size per 30 ticks; eligible fires inside Home AND the pawn's area (Unrestricted = Home only); drafted pawns can be ordered to Extinguish anywhere. Danger (`fire_danger.go`): path cost +1900% on fire, +200% beside; idle pawns step out (600-tick hold, no-flap tested); animals flee within 4 cells; alert "Fire in the colony!".
- Saves/golden: scale goldens re-pinned (only Firefight priorities changed, checked field by field on all six); old saves keep Firefight off until enabled. Bench 500-cell blaze 0.08-0.17 ms/step.

## SIM-059-plant-damage
Gradual cold/heat plant damage (PLAY-016 §3) · 2026-10-25 · ce02a68bb
- Changed: `plant/damage.go`: damage builds over hours: 25%/h at the kill threshold, +5%/h per C beyond, cap 60%/h (crops' FrostKill, wild plants' ColdKill/HeatKill); rates accumulate exactly across staggered chunk windows. At 100% the plant is removed (batched notice). Daily per-species warning ("Frost is damaging the corn" / "Heat is scorching..."; deduped, no save field). Harvest yield and straw scale by (100 - Damage)%.
- Decisions (hysteresis, `Plant.Damaging`): damage stops 2C back on the safe side; below 60% the plant heals 10%/h; from 60% it keeps dying at 25%/h.
- Read model: `PlantView.Damage`/`DamageKind`. Limits: berry bush has no kill temperature (dormant only); a field dying across a climate-interval boundary reads as two notices; no golden moved.

## SIM-060-play017-colony-rules
Colony rules: sowing, light, hauling claims, apparel, tame feeding, chase warnings (PLAY-017) · 2026-10-25 · 7b4dc6abd
- Changed: sowing (`food.go`) `assignZoneClear` cuts a wrong plant (wild, moss, bush, tree, other crop) off a zone cell first; a ripe wrong crop is harvested; out of season and under buildings nothing is cleared (`sowableIgnoringPlant`); forced `OrderSow` uses the species' sow work.
- Changed: plants grow for max(sunTicks, lampTicks): sun window on unroofed cells or any hour with artificial light >= `plant.StrongGrowLight` (0.5); grass same rule at 24/14 speed in the sun window (daily regrowth unchanged); campfires light only while fuelled. Ripening calls `markRipeHarvest`; `wantsHarvest` needs `Designated` (Cancel holds). `PlantView.GrowState`/`RipeIn`.
- Changed: haul claims (`haul_claim.go`): `Stack.Unclaimed`/`HaulMarked`; only `haulToStockpile` skips unclaimed unmarked stacks (map loot, chunks, vault/casket loot, raider/visitor gear, non-colony animal corpses, wild eggs); pickup or storage claims; commands `DesignateHaul`, `SetHaulMark`.
- Changed: apparel picks the best Body x Head outfit scored by `comfortShift`; `dressCheck` before work; `dressInterrupt` (60-tick bucket) drops interruptible work; 1C dead band and per-slot cooldown; Equip postponable; forced "Wear" slot kept (`worn[NoSlot]`); log lines only; index `Game.clothes`.
- Changed: tame animals eat the nearest diet-allowed food (stockpiles included) with `FeedClaim`; skip zone/harvest-marked plants; pens and areas bound the search; tame predators hunt wild prey only. Chase warnings (`chase.go`): one danger message per chase (hungry fallback, revenge, charge, fighting back via `hurtAs`), re-warned only on a new target or an hour after the chase ended (`ChaseWarnedAt`/`Target`).
- Saves/golden: save v3 (older saves mark loose chunks/carcasses outside stockpiles as found, ripe zone crops get their mark). Goldens re-pinned; field by field only Animals and Messages moved (4 scale runs + grown). Game package under -race ~175 s.
- Open: food carried to a prisoner keeps its flags; a stockpile painted over found items doesn't claim them; pack members each warn once (two wolves pause twice). Plan `.agents/handoffs/PLAY.md#play017_plan`.

## SIM-061-bench-before
Benchmark baseline before the care package (record) · 2026-10-29 · 37f9a0948
- Measured: `bin/benchmark -sizes 250,500 -speeds 3 -fixtures grown,raid -hash` on cached fixtures, before SIM-061. Tick mean/p99 (ms): grown 250 0.211/0.722, grown 500 0.216/0.542, raid 250 0.282/1.267, raid 500 0.323/1.418; frame mean 0.64-0.97, p99 1.3-3.7; no frame over 5 ms on grown, 0.3-0.5% on raid.
- Decisions: state hashes after 3000 ticks are the behaviour proof for refactors: grown 250 `0x7d91c51db00ec208`, grown 500 `0xb525d0318b98b2d4`, raid 250 `0x584b5b06c1e2ffdc`, raid 500 `0xefdc3010260906b7`. Later phases must keep tick p99 at or below this baseline.

## SIM-061-care-package
Care package, sub-package rule, pawn-interaction gap analysis (SIM-061…066 programme) · 2026-10-29 · 37f9a0948
- Changed: no behaviour change. Programme: colonist-to-colonist interaction at RimWorld fidelity (SIM-061…066; backlog `.agents/gaps/PAWN_INTERACTION_GAP_ANALYSIS.md`). Rule (SOLID_GO §7, §8, §10, root AGENTS.md): a cluster may become a sub-package of the god type's package behind consumer-owned roles in `deps.go`, adapters in `<sub>_host.go`; features of a package at its ceiling go there. Arch guard: a `game/<sub>` package never imports `game`. ENG-003 extended to pawn care and mental breaks (wiki numbers, original names/prose). HEXAGONAL_ARCHITECTURE §4 lists `game/care`, `game/mind`.
- Changed: `internal/application/game/care`: `Care` root, `Carries` (two-way carry index replacing `g.carrying` and its linear scan), `Policy`, `Doctor` (rescue, tend, emergency tend, `tendedUntil`), `Rest`, `SafeBedFor` (reproduces the old rescue and bed-rest rankings), `PawnOrders` (today's "Tend X"), `CareView`. `Host` embeds eight value-only roles; `game/care_host.go` adapters add no `Game` method. Deleted from game: `care.go`, `bedrest.go`, `med_cmds.go`.
- Contracts (inert until later): `command.Order` `OrderRescue … OrderCarryToPrisonBed` (target in `Command.Target`); job kinds `FeedPatient`, `Strip`, `VisitSick`, `CarryToPrisonBed`; `PawnView.CarryingPawn`. Mental break tables (new kinds `InsultingSpree`, `TargetedInsult`, `Berserk`, `MurderousRage`, `Catatonic` appended; `Tier`, `Thresholds`, `ShiftThresholds`, `MTBTicks`, `Pick` with lower-tier fallback, `DurationFor`) from the wiki, unused by the sim until SIM-066.
- Limits (budget paid): game non-test lines 57,019 -> 56,091; files 185 -> 183; `Game` methods/fields 2,105/152 -> 2,047/149; budget lines lowered; each later phase must leave package game no larger than the previous commit. care ~1,000 lines, coverage 96.1%.
- Decisions: the wiki publishes no feeding trigger above Hungry, downed food-fall rate, guest-death goodwill or visit-sick opinion minimum (gap doc records each); `needs.CriticalHunger` (2,500/10,000) already equals the wiki Hungry 0.25. Accepted: game calls components directly (no forwarders); `pickUpMedicine`/`tendMedicine` stay in game (vet work); `SafeBedFor` takes a `BedNeed` because the two rankings measure distance differently.
- Proof: benchmark hashes unchanged (see SIM-061-bench-before); fixtures loaded from before the change prove save compatibility.
- Open: `make lint` not run (installed staticcheck built with Go 1.23 can't load the Go 1.26 module; reinstall with `go install honnef.co/go/tools/cmd/staticcheck@latest`). Next: SIM-062.

## SIM-062-pawn-orders
Orders on a pawn: rescue, capture, arrest, prioritize tending · 2026-10-29 · 9a547ad0e
- Changed: right-click a pawn lists "Rescue" (downed colonist out of bed), "Capture" (downed raider/visitor), "Arrest" (standing colonist/visitor), "Prioritize tending" (undrafted) / "Tend" (drafted). Disabled rows say why ("no free bed", "no free prisoner bed", "carried by <name>", "cannot reach", "not able to act", "cannot target itself"). Shift queues any, queue names the target. Forced rescue works drafted or undrafted (rescuer stays drafted); patient goes to `SafeBedFor`'s pick.
- Decisions: pawn-targeted orders are `Prioritize` with `command.Order.TargetsPawn()` and `Command.Target`, routed to `care.Force`. One `judge(doer, order, target)` decides both menu row and `Force` so they can't disagree. A forced rescue claims the patient so the Doctor doesn't race it. `EmergencyTend` joins the Shift queue; a queued tend no longer wipes later orders. Capture and arrest moved into care (`care/capture.go`; `prisoner_capture.go` deleted); `carryToBed` is the single carry driver; `Arrestable` checks roster membership (a released prisoner walking out isn't arrestable). New role `care.Prison` (8 methods). Gate finding: `orders.hasPawnOrder` no longer counts Arrest when forcing the drafted menu (`TestHasPawnOrder` "arrest alone"), else a drafted right-click onto a colonist opened a menu instead of walking.
- Limits (budget paid): game 56,091 -> 55,891 lines, 183 files, methods 2,047 -> 2,038, fields 149; care coverage 97.5%. Hashes unchanged (fixtures place no orders).
- Open: a drafted group right-click checks the first drafted pawn's options; "Prioritize tending" also offers a standing wounded colonist (RimWorld only tends lying patients); rescue covered downed colonists only (SIM-063). Checklists: vanilla 11.5 Rescue `[x]`->`[-]` (guests), UI 13.2 `[x]`. `make lint` not run (as SIM-061).

## SIM-063-rescue-fidelity
Rescue fidelity, safe beds, guests, prisoners back to bed · 2026-10-29 · a951aa085
- Changed: rescue covers every downed cause except collapse-asleep (`CauseNoBed`) incl. malnutrition, disease, blood loss, heat/cold, anaesthesia out of bed; colonists and guests (visitors, traders, allies, faction-less strangers). `care.SafeBedFor` filters beds with fire within a tile or air past the wiki's 90C margin (`Reach.BedHazard`), beds the patient can't claim or held for others, prison beds for non-prisoners. Patient needing care or a guest: medical hospital bed, any medical bed, non-medical hospital bed; others: own bed then any free; own claim kept, another bed held for the stay. Captives rank prison beds the same (`care.PrisonBedFor`).
- Changed: carrier drafted, ordered away, fleeing or downed drops the patient; mid-carry death via `Doctor.PatientDied` hooked in `dieBy`. A warden carries a downed prisoner lying outside a bed back to a prison bed (autonomous and "Take <name> to bed").
- Changed (`care.Hospitality`): rescued guest tended under the guest care group, leaves healed with +12 goodwill; guest dying downed -5 (published "non-hostile member killed"; wiki gives no guest-death value). A faction-less stranger asks to join on recovery: 50% when leaving is safe, 100% when a hostile is on the map or air unsafe. Letter Accept (`command.AcceptLetter`, new) / Decline (`DismissLetter`). `Game.DevJoinOffer` is the only source (no stranger event yet).
- Fix: `careRoster.IsColonist` counted visitors as colonists (they took orders and a rescued guest was never tended).
- UI: inspector "Resting in a medical bed", "On bed rest", "Guest of <faction>"/"Stranger" (`PawnView.OnMedicalBed`, `GuestFaction`); join letters Accept/Decline. Journeys `TestJourneyOrdersRescueVisitor`, `...TakePrisonerToBed`, `TestJourneyHudJoinOfferAccept`.
- Open: event bringing stranger (vanilla 19.5 `[-]`); resting guest's job line reads "Sleeping". Committed with SIM-064…066; gates in SIM-066.

## SIM-064-feeding-patients
Feeding patients, downed hunger · 2026-10-29 · a951aa085
- Changed: downed pawns' food need falls at the awake rate (rest frozen; the wiki publishes no downed rate), prisoners too. `care.Feeding`, a Doctor work giver after tends, feeds a patient who can't feed itself (downed colonist or taken-in guest, not carried); bed-rest pawns who can stand get up to eat. Feeds at or below Hungry (2,500/10,000 = `needs.CriticalHunger`); one meal fills almost full; patient claimed by its feeder; no food retries after 1,000 ticks (`FeedRetry`, saved).
- Changed: warden prisoner food run shares the driver (also fixed a warden giving up after its first re-path). Order "Feed <name>" offered below 80% food, disabled "no food". Log "Fed by <name>"/"Fed <name>". No-flap: `TestFeedDoesNotFlap`, `TestFeedPatientDoesNotFlap`; journey `TestJourneyOrdersFeedPatient`.
- Open: feeding downed animals (MED gap §7).

## SIM-065-visit-sick-strip
Visiting the sick, strip · 2026-10-29 · a951aa085
- Changed: `care.Visits`: an idle standing colonist with recreation <85% visits a friend lying awake in bed (patient on bed rest, downed or needing tend). Friendship: visitor's opinion >=20 (original tuning; wiki publishes none) and patient's >=0. Visit lasts 1,500 ticks, both gain 0.216 of the recreation bar (wiki), one social interaction midway, patient log "Visited by <name>"; patient not reserved so doctors aren't blocked. Awake patients in bed (downed included) are valid idle-chat partners.
- Decisions: pair cooldown is stateless: a pair may start only in the first 600 ticks of each 2,400-tick window (>=900 ticks between visits, asserted by the no-flap test) because the agent could add no saved state.
- Changed: `care.Stripping`: "Strip <name>" on any downed pawn (colonists included) or prisoner, 90 ticks, apparel and weapon drop as items; stripping a downed faction guest costs -40 goodwill (wiki pages disagree; decision -40); `Gear.HasGear` disables with "nothing to strip". Journey `TestJourneyOrdersStripPrisoner`.
- Open: per-pair visit cooldown with saved state.

## SIM-066-mental-breaks
Mental breaks that target others; W3 integration (SIM-063…066) · 2026-10-29 · a951aa085
- Changed: new sub-package `game/mind` owns break risk, choice, acting out, recovery and social fights (moved by P1); `mental.go` gone; roles Pawns, Breaks, Acts, Aggression, Brawls, Events adapted in `game/mind_host.go`. Risk checked hourly; tiers at mood 35/20/5 shifted by break-threshold traits; MTBs 4/0.8/0.5 days; `mentalbreak.Pick`/`DurationFor` wiki tables with rolls on the social stream; a day of catharsis follows.
- Changed: insulting spree and targeted spree (insults via the social path, can start a fight; retarget at most every 250 ticks); berserk (nearest creature until downed or timeout); murderous rage (hunts one colonist/prisoner until it dies, or rager downed/arrested); catatonic (downs the pawn, `health.CauseCatatonic`, then rescued and fed like a patient). Any colonist may "Arrest" a breaker (ends the break, `g.mind.End` in `makePrisoner`); drafted colonists get "Melee attack/Shoot <name>" on a berserk/murderous colonist (no "raider"), which also opens the drafted menu.
- UI: `PawnView.MentalTarget`, colonist bar tooltip names the break, letters for aggressive breaks, `LetterView.JoinOffer`; read-model fields `CarryingPawn`, `OnMedicalBed`, `GuestFaction`. `AcceptLetter` covered by UI buttons and a journey (`TestEveryInputActionHasAJourney` holds).
- Limits (budget paid): care root was over 40 methods, so each feature got a component (`Feed`, `PrisonBeds`, `Guest`, `Visits`, `Strip`), root back under 40; SOLID_GO §10 now says so and allows adapters beside wrapped code when the file budget is full. P1 moved prisoner state, a prisoner's life and warden work into new `game/prison`; recruiting and guest joining share `g.enlist`; no behaviour change (hashes matched). game 55,891 -> 55,844 lines, 183 -> 182 files, methods/fields 2,038/149 -> 2,010/148; coverage care ~97%, mind 96.4%, prison 93.9%.
- Determinism/golden: benchmark hashes changed intentionally (downed hunger, feeding, visits, rescue causes, breaks): grown 250 `0xf6c61f40b377d985`, grown 500 `0x1a01790f2f095343`, raid 250 `0x8066e6b6f3956edd`, raid 500 `0x7185b71c1b1e1272`; tick p99 <= SIM-061 baseline. `TestGrownGolden` (scenario) re-pinned to `0x73d615f22d9b245`, stable over `-count=2`. The four phases are one commit (shared contract files). `make lint` not run (SIM-061).
- Open: tantrum/object destruction and escape breaks (vanilla 10.1 destructive `[-]`); feeding downed animals; stranger event; per-pair visit cooldown; "Resting (injured)" job line; drafted group right-click. Checklists: vanilla 10.1 eligibility `[x]`, catatonic `[x]`.
