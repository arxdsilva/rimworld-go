# DEF handoffs

Families: DEF, CBT, SHT, EAI. Each section condenses one task's handoff; the full original of `<stem>` is `git show 76d605d04:.agents/handoffs/<stem>.md` (a plan: `git show 76d605d04:.agents/handoffs/_plans/<stem>.md`). New sections go at the end, in the format of `.agents/templates/HANDOFF.md`.

## CBT-1-damage-model
Damage model · 2026-10-30 · 5d88bf5b6
- Changed (checklist 11.3, 17.6 `[x]`): `damage.Hit{Weapon, Form, Amount, Pen, Stopping}`; forms Blunt, Cut, Stab, Bullet, Arrow, Scratch, Bite, Crush, Burn, Flame, Bomb, EMP, Stun. `weapon.HitOf`, `armor.Landed`, `combat.Wounds` role (`Absorb`, `HurtHit`). Body-part-first armor `armor.Struck` (coverage groups, plies, `GroupOf(part)`); coverage, layers, garment HP derived from kinds (save format unchanged).
- Changed: new wound kinds Stab and Gunshot (bleed, scar, fester at 10 %; spike traps stab). `body.Strike` (`body/hit.go`) with wiki overkill windows: brain and heart can be destroyed and kill. `Outfit.HitWear`: 1 HP per 4 damage; ratty, tattered, destroyed with a message.
- Decisions: stagger step cost x6 for 95 ticks, never extended while running; stun 20 ticks/point; blunt stuns 15 % / 100 % (wiki: overkill windows, stopping power 1/6 speed 95 ticks, blunt stun chances, layers/coverage, EMP per point). Original: 40-tick blunt stun, 1 HP wear per 4 damage.
- Changed: removed the 3-region roll, `armorRoll`, `Creatures.Armor`, `Harm.Hurt`/`HurtAs`, `woundKindBy`; new `game/hit` package; `Spread`/`Reconcile`/`TrimOpen` moved into `body`. About −60 game lines, −1 Game method. Golden hashes re-pinned.
- UI: `Staggered` on `PawnView` and `AnimalView`; Health tab names stab and gunshot wounds.
- Open: multi-part cut table to verify; raiders still wear no armor and weapons lose quality on equip (CBT-3); journey HelmetGuardsHeadOnly skipped (game-level test covers it).

## CBT-2-attack-mechanics
Attack mechanics · 2026-10-30 · 5d88bf5b6
- Changed (17.5 melee `[x]`, 17.1 auto-undraft `[x]`, 17.3/17.4 notes; 16.1 stays `[-]`): `projectile.Shot.Hit` set at fire time and saved (old saves rebuild via `shotHit`); nothing reads the attacker's weapon at landing.
- Changed: melee (`combat/melee.go`): wiki hit curve (skill 20 → 90 %, skill-less creatures 62 %), wiki dodge (`attack.DodgeChanceOf`), damage ×0.8–1.2, per-mille rolls. Wiki tools per melee weapon; humans adjacent use a gun's stock/grip/limb; humans fist, head, teeth; animals their roster attack plus a second (original: 60 % share, 25 % chance).
- Changed: Euclidean ranges (max and min), turrets and mortar included; autocannon refuses 8 tiles. Weather factors (unless both ends roofed); cover checks 8 tiles, angle bands, 33/67 % at close range, combined covers. Downed humans attacked only when ordered; auto-engage covers manhunters, mechs, insects; grenade orders walk into range then throw.
- Decisions: auto-undraft after `AutoUndraft` = 10,000 quiet ticks, re-checked before release (no flap). `DropWeapon` drops or stows into a new saved spare slot (`domain/pawn/inventory`); the spare falls when its carrier dies. Combat XP per second of attack cycle: 80 hit / 30 miss (original).
- UI: Drop weapon button (STO-3).
- Open: `PawnView.Spare` not exposed (done CBT-3); dodge indistinguishable from a miss in `TraceView`/log (CBT-3 adds `TraceView.Dodged`); gun-stock blows need a blunt sound (done CBT-5); weapons lose quality on equip (CBT-3).

## CBT-3-weapons-gear-structures
Weapons, gear and structures · 2026-10-30 · 73ffc63cd
- Changed (16.1 now `[x]`, 16.3 single-use launchers, 17.8 shield belt/smokepop, 24.2): wiki-number weapons rocket launcher, triple rocket launcher (single-use, consumed by one pull), smoke launcher, charge rifle, charge lance, ikwa. `WeaponStats` reports `MinRange`, `SingleUse` (no launcher has a wiki min range).
- Changed: `combat.Gear` (`game/combat/gear.go`): equipped weapon keeps quality, stuff, wear, saved in the Turrets block (old saves load Normal steel). Melee damage scales by stuff; accuracy and damage by quality; drop and spare keep the make. `PawnView`: `WeaponQuality`, `WeaponStuff`, `Spare`, `ShieldPct`, `Smokepops`, `Kills`, `DamageDealt`; `TraceView.Dodged`.
- Changed: shield belt in `game/hit` before armor (wiki energy, recharge, 3200-tick break; EMP breaks it; wearer can't shoot/throw). Smokepop pack 3 uses (original), raiders pop when shot; lead added `PopSmokepop`. Raider armor kits `domain/combat/kit` by tech and weapon cost; raider views show Apparel/Worn/ApparelArmor.
- Changed: mini-turret wiki numbers; every turret shot is a projectile (strays, friendly fire, armor); turret explodes half the time when destroyed. Spike trap: 5 stab hits scaled by stuff; enemies/manhunters 100 %, other wild animals 20 %. Rearm: slug turret wants uranium, mortar a reinforced barrel; unobtainable, steel stands in. Kills/damage counted; death from wounds credits the last attacker. `craft.Slots` now 5 (belt layer), `Outfit.Armor()`.
- UI: see CBT-5. Game −6 lines.
- Open: uranium and reinforced barrels have no source; explosions don't count toward records.

## CBT-4-combat-ai
Combat AI · 2026-10-30 · 73ffc63cd
- Changed (17.8 target selection, cover, threat response, grenade avoidance; 24.3/24.4; 24.6 hives; 19.4 prison breaks; tantrum): raiders use threat-weighted targets, cover seeking that holds its spot, grenade dodging, loot on retreat, crop burning, siege resupply, staged assault timer 10–15 h (`domain/raid`). Escaped prisoners fight out.
- Changed: mechs bash walls, shut doors, arrive by drop pod, never retreat (wiki); EMP via `explosion.Adapted`, shared with turrets. Colonists: drafted fire at will, cover steps; threat response Flee/Attack/Ignore (lead added `SetThreatResponse`, `PawnView.ThreatResponse`). Hunters fetch any non-explosive ranged weapon. Stalking skipped (wiki page missing, not in core).
- Changed: tantrum Major tier 8000–12000 ticks, smashes nearby objects. Prison break rooms join per the wiki rule. Hive: construction `HiveMound`, wiki 130 HP, cap 30, spread within 10 tiles, −17 C. Kidnap moved into `game/raid` (`kidnap.go` deleted). Doors forbiddable via `BuildingForbid`. Game ≈ −90 lines, −1 file, −7 methods.
- Decisions (original): prison break 60-day mean, 3-day grace, 10-day calm; hive breed/spread rates; cover numbers; tantrum length. Wiki: hive HP/cap/radius/temp, mech cluster rules, room-joining rule. No-flap tests for cover seeking and threat response. Raid frame p99 1.80 → 1.91 ms (250), 2.07 → 2.10 (500).
- UI: threat response control, door forbid (CBT-5).
- Open: stalking skipped; mech voices blocked; hive dark/warm placement unverified.

## CBT-5-explosions-and-combat-ui
Explosions and combat UI · 2026-10-30 · 73ffc63cd
- Changed (17.7; UI 12.3, 13.3, 13.4, 14.1; 24.3 records): bomb ×2 on passable buildings, ×4 impassable and plants, ×8 walls (wiki); EMP 30 ticks/point, 2200-tick turret adaptation (wiki). Bombs wear and destroy weapons/apparel at 0 HP; burnable raw stacks destroyed by a hit ≥ 50; stone/metal immune (original). Cook-off: shells/grenades in a bomb or flame blast or lying in fire go off 30 ticks later via the IED fuse queue (`Fuse.Stack`, saved), ≤ 8 a tick, each stack once (original). Flame ignites every cell in radius by flammability; creature cell 70 %, centre always (original).
- Changed: `WorldView.Smoke` lists live smoke clouds; `causePhrase` became `health.CausePhrase`. Raid hashes match HEAD for this change alone.
- UI: min-range ring from `WeaponStats.MinRange`; cover overlay with natural cover, direction-aware (`AngleFactor`×`RangeFactor`); target ring; Gear tab weapon quality/stuff, damage/cooldown tooltip, spare, kills/damage, shield bar, smokepop Use, threat response control; door pane Forbid/Allow (`bldg.forbid`); History Records tab; "dodged" floaters; blast effects (fire 50, EMP 30, smoke 1200 ticks); shield bubble; held-weapon keys, belt layers, hive mound build key.
- Art/sound: `arms.py`, `belt.py`, `effect.py`, `hive.py` (6020 sprites, 0 errors); 10 procedural sounds (`wave3_recipes.go`), cues `render/cue/wave3_sounds.go`; audio 24.0 of 30 MB.
- Decisions (lead): `TestJourneyMechOpensDoor` now needs a real break-in (door bashed, room wall down, or mech inside). Colonist command row has nine buttons; at 1280 wide it reaches the Global panel (`TestHUDRightClickNeverStealsADraftedMove` picks a point above the grid). Goldens re-pinned (wave 3). BenchmarkDrawArt500 draw calls 410 → 429 (z1); no allocation in new render code.
- Open: explosions don't count toward records; mech voices blocked; light accuracy factor fixed; command row crowding may need a second row or overflow menu.

## DEF-0-combat-package
The combat package and the combat domain · 2026-10-29 · 81281cbd3
- Changed (programme BED/PRD/DEF, `.agents/gaps/DEFENCE_GAP_ANALYSIS.md`, wave A): `application/game/combat` (components `Sight`, `Strikes`, `Guard`, `Traps`, `Breach`, `Turrets`; host roles Creatures, Rosters, Ground, Skills, Jobs, Harm, Events, Dice plus Power, Flight, Hunts, Breaches, adapted in `game/combat_host.go`). `turret.go`, `combat_state.go` gone; save keeps `Turrets` gob names. Health, equipment, hunting, building damage/repair, path maps stay in game.
- Changed: `domain/combat/{damage,armor,attack,cover}`: armor = effective rating − penetration; below half deflects, below full halves, else passes; halved sharp becomes blunt; layers outer to inner; rating cap 200 %. Accuracy = shooter per-tile accuracy^distance × weapon bands × weather × smoke × cover. Cover: sandbags/barricade 55 %, chunk 50 %, tree 25 %, wall/rock 75 %. `weapon.Def` gained warmup, burst, penetration, damage type, bands; the 12 old weapons reproduce today's formula (`TestLegacyWeaponsReproduceToday`) until DEF-1.
- Limits: hashes identical; game package lines 55,844 → 54,621, files 182 → 181, `Game` methods/fields 2,010/148 → 1,950/147; budget lines lowered. Raid 500 p99 within noise; combat coverage 98.2 %.
- Decisions: ENG-003 extended to BED/PRD/DEF in `AGENTS.md`. Lead checklist fixes: 7.4 room stats/roles `[-]`, 16.3 ranged `[-]`, warmup `[-]`, cooldown/flee/arrival/assault/kidnap/retreat `[x]`, storage priorities `[x]`.
- Open: `make lint` pending a staticcheck built for Go 1.26.

## DEF-1-projectiles-armor
Projectiles, burst, armor, combat overlays · 2026-10-29 · 3095e4453
- Changed (wave B; `combat/shot.go`, `strike.go`): each ranged shot decided at the muzzle: aim roll (band × per-tile accuracy^distance × size × light × weather), wild miss within the miss radius, then a cover roll that can damage cover. In flight walls, doors, rock stop it; creatures on the path may intercept (40 % × body size); allies within 5 tiles of the shooter are safe. Burst shots left in `job.Count`; `Shots` in flight saved.
- Decisions (original tuning, wiki has no rule): a shot hits its target only if within 1 tile of the aim point; otherwise a 50 % stray hit on whatever is in that cell.
- Changed: wiki stats for knife, club, spear, bolt-action rifle (fists, animals, sentry gun keep today's values); apparel armor leather parka, wool parka, duster, tuque; hits roll a body region then layers outer to inner.
- Changed: `TraceView.Stage` (Whole, Fired, Landed), `WorldView.Shots`, `Game.HitChance(shooter, target) (attack.Report, bool)` (`hasRanged` became a function; folded into `SimWorld`, Simulation union stays 8).
- UI: range ring on the selected drafted colonist; line-of-sight line and hit-chance card over hovered hostile; cover overlay Shift+O (`overlay.cover`, `TestJourneyCombatOverlay`); projectile streaks. Sound: nine procedural recipes (`cmd/audiogen/combat_recipes.go`), cues by stage in `render/cue/combat_sounds.go`. Audio budget raised 20 → 30 MB by user decision (ART doc §25).
- Limits: game 54,621 → 54,565 lines, `Game` methods 1,950 → 1,945; budgets lowered. Hashes: grown 250/500 and raid 500 unchanged, raid 250 moved (combat); `TestGrownGolden` re-pinned (reason beside the pin). Lead fixes: Forbid toggle only on sleeping places; Bills tab width clamped to 560 px; overlay feed uses reused scratch buffers.
- Open: hit chance uses fixed light and clear weather (fog 50 %), not map light or current weather (DEF-3/DEF-4).

## DEF-2-weapons-armor-explosions
Weapons, armor, explosions · 2026-10-29 · 2c49ba996
- Changed (wave C): 30 weapon/armor item kinds: melee mace, gladius, longsword, pila; ranged short/recurve bows, greatbow, revolver, autopistol, machine pistol, assault and sniper rifles, pump and chain shotguns, heavy SMG, LMG, minigun, incendiary and EMP launchers; grenades frag, molotov, EMP; armor flak pants/vest/jacket, plate, marine, simple/advanced helmets. Weapon names singular, item kinds plural. Rocket launchers skipped (CBT-3).
- Changed: `domain/combat/explosion`, `game/combat/blast.go`: damage full at centre to half at rim (original), LOS from centre. Bomb ×2 vs buildings; flame ignites; EMP stuns turrets (`AimSave.StunnedUntil`); smoke cuts aim to 30 % (`Blasts.SmokeFactor`, clouds saved as `Smoke []explosion.Cloud`). Grenades arc over cover and scatter at range (original). Explosive weapons blocked from hunting (`TestNoHuntingWithExplosives`).
- Changed: `kit.Pick` by faction tech and points. `AppendPiece` scales plate ×0.9, simple helmet ×0.5, advanced helmet ×0.7 by stuff (steel fallback) and quality; Legs slot dressing. Read models: `PawnView.WeaponStats` (`weapon.View`), `ApparelArmor []armor.Set`; `BurstView.Blast`, `Radius` (tenths); explosions leave a Landed trace.
- UI: Gear tab `WeaponStats` and per-garment armor lines (`GearStatLines`); renderer draws 22 weapons and 8 armor pieces (`worn.go` order shirt → flak pants → vest → parka → jacket → plate/marine → beard → hair → helmet, stuff tints, zero allocs); `Bus.ListenBursts`; `ListenBlasts` plays explosion sounds by Blast kind. Art validator 0 errors over 5,576 sprites.
- Limits: game package 9 lines smaller; draw calls 419 → 425 (z1) from the changed scene, new render code allocates nothing.
- Open: melee not stuff-scaled and equip loses crafted quality (CBT-3 fixed); rocket launchers (CBT-3); raider armor (CBT-3); colonist grenades (DEF-4); explosions don't damage items or chain, no ×4/×8 (CBT-5); recipes arrive in PRD-4; no per-kind blast art; armor not on portraits/colonist bar; ground items not stuff-tinted; `countRoomBuildings` (BED-2) allocates via `construction.Footprint`.

## DEF-3-cover-traps-turrets-mortars-doors
Cover, traps, turrets, mortars and doors · 2026-10-29 · f39f6375d
- Changed (wave D; sections 3.3, 3.5; contract commit `8f3932359`: commands `SetDoorHoldOpen`, `SetMortarTarget`, `ClearMortarTarget`, `SetTurretHold`, jobs `ManMortar`/`RearmTurret`, `BuildingView` HoldOpen/Barrel/Shells/TargetX/TargetY/HasTarget/HoldFire): `domain/combat/cover` permille cover from buildings, rock, grown trees, cacti, bushes, chunks (chunk tiles cached 250 ticks). Wall/door 40–75 (wiki), barricade 55, turrets 40/50. `CoverPct` deleted; overlay uses `cover.OfBuilding`.
- Changed: IEDs: 15-tick fuse, blast, re-laid blueprint; high explosive radius 3.9; incendiary/EMP/smoke burst as their shell (original). Autocannon and slug turret wiki stats; barrels 60/90/30 rounds, spent barrel stops until swapped, steel per barrel 9/20/7 (wiki cost / 9, original), Hold fire.
- Changed: mortars: crew order (drafted colonist on its spot, else `SetMortarTarget.Pawn`, else nearest idle drafted within 12 tiles, original); range 29–500, min 29.9, warmup 240, cooldown 1680, forced miss radius 9 scaled by crew Shooting; 20 shells/barrel; shell items mortar/incendiary/EMP/smoke. `RearmTurret` hauls shells and barrel steel (idle undrafted colonists).
- Changed: doors (`domain/building/door`): `Kind.IsDoor()` at 17 game sites. Opening speed by stuff (wiki); powered autodoor faster; base open 45 and auto-close 120 ticks (original); never closes on someone. HeldOpen lets hostiles through; Barred = forbidden door unwalkable to colonists. Open doors let shots through. MaxHP (wiki): autodoor = door, barricade = sandbags, heavy turrets 380, mortar 180.
- UI: door Hold open, turret Hold fire, mortar Set/Clear target with barrel/shells/target lines; `orders/mortar_aim.go` aim mode (next map click, right-click/Esc cancels); aim hint on the shell's Hint channel (lead); renderer draws guns; art family `fortify`; 14 procedural sounds (`structure_recipes.go`).
- Limits: hysteresis: mortar cooldown, door delay, spent barrel silent until rearmed. Game now 54,494 lines. Draw calls 425 → 407 (z1). `TestGrownGolden` re-pinned.
- Open (DEF-4): bullets setting off IEDs, chains, sappers, raiders with mortars (done DEF-4-R1/CBT-5); door forbid UI (CBT-5); uranium and reinforced barrel items.

## DEF-4-R1-raids-drafted-control
Raids, drafted control, IED leftovers, combat log · 2026-10-29 · 9838c9c14
- Changed (wave E, §3.1, 3.5, 3.7, 3.8; built on `game/raid`): `domain/raid/rules.go` (`PickArrival`, `PickStrategy`, `Fit`, `FleeLoss`, `Broken`, siege/staging/pod/target-priority constants; 100 % covered). Drop pods (`pods.go`, `edge.go`): one per raider, centre/edge/scattered, 240-tick fall, roof break with crush, hostile `ArrivalView`s; dev hook `RaidDropPods(kit ...item.Kind)`.
- Changed: strategies: staged assault (`stage.go`), siege with two raider-crewed mortars and wiki storm (`combat/mortar.go` `Raise`/`Crew`/`Man`), Steal and Burn (`mischief.go`), rival factions fight each other, retreat at rolled flee share, give-up day. `Threats.Settle` fits a pick to tech/points. Big threats: a raid face splits into crashed ship part, mechanoid raid (`Mechs.Raid`) or people, by `Threats.Major`.
- Changed: drafted control (`combat/draft.go`): fire at will, hold position, grenade belts, `ThrowGrenade`; `PawnView.FireAtWill/HoldPosition/Grenades`. IEDs: bullets light a charge, blasts chain, sappers know 30 % of charges (`sapperTraps`). Combat log (`combat/log.go`): 64-entry saved ring, `WorldView.CombatLog []CombatEventView{Tick, Text}`.
- Decisions: wiki: entry 45+15/30 (+10 haywired, mechs only), flee share 40–70 %, siege 1.5–3 days, 8 % per hit, two mortars, sapper 30 %, weights (raid 7.4, ship parts 2.0+2.0, factions 1 each, mechs 0 below 300 points). Original: pod fall 240, roof crush 15, staging 10 tiles/3 h/6–14 h, siege 300 points and 32-tile camp, shell split 15/5, steal 50 silver, 3 fires. No-flap tests for staging, retreat, fire at will.
- UI: read models only; screens landed in U5 (UI 13.1, 13.2, 24.3); `PawnView.RaidPlan` names the strategy. `TestDraftDefaults` replaced the contract inertness test. Game budget lowered to 54,474 lines, 180 files, 1,935 methods; `TestGrownGolden` re-pinned.
- Open (most done in CBT-4): opportunistic theft, crop burning, 10–15 h assault timer, siege resupply, mech groups via pod hook, walking into grenade range, threat-weighted targeting, cover seeking.

## DEF-4-R2-mechanoids-ship-part
Mechanoids and the crashed ship part · 2026-10-29 · 9838c9c14
- Changed (wave E; contract CT2: roster rows, `weapon.MechBlades/PulseLance/NeedleCaster`, `construction.ShipPart`, incident `CrashedShipPart`): original-design species Harrow (blade skirmisher, fast), Spindle (sniper), Bulwark (heavy gunner, XL, slow), Quill (skirmisher), wiki role stats; archetype `ArchMech`, behaviour `Mechanoid`. Brain `Mechs.Think` (`game/raid/mech.go`): never the animal brain; no needs, never tamed/down/flee; retarget every 60 ticks; plating counts as armor.
- Decisions: guard leash 30, release 36 (hysteresis, original). EMP stun 15 ticks/damage (wiki) then 2200 ticks immunity; `AnimalView.Mech`, `Stunned`. No wound body (no bleeding, infection, stains).
- Changed: wreck: corpse stack (`item.CorpseAnimal`, mech `Stack.Animal`), Dried (never rots), predators skip, no meat; `ItemView.Wreck`. PRD-4 shred recipe yields via `animal.WreckYield` (wiki: Harrow/Spindle/Quill 15 steel; Bulwark 30 steel + 10 plasteel; PRD-4 divides by 10). `Mechs.Raid(points, arrival, now)` from 300 points; mechs land at the arrival spot (no pod entity).
- Changed: ship part (`game/raid/shippart.go`): `construction.ShipPart` 6x3, 1200 HP, Ancient (no blueprint, not wealth); withers plants in a radius 5 → 100 tiles over 20 days (`BlightRadius`). Guards sleep until a colonist comes close, the hull is hit, or a guard is hurt/EMPed, then wake for good (`TestShipPartWakeNoFlap`). Destroyed: 60 steel, 10 plasteel, component, advanced component. Incident weight 2, 400-point gate, one at a time; saved via `Mechs`. Storyteller face weighting is R1's (ship part 40 vs people 74).
- Changed: roster rows in Go and `animal_roster.py` (`TestEverySpeciesHasArt`, `TestRealAnimalBodies`); full art sets per mech (5957 sprites, 0 errors).
- Open: sounds BLOCKED `[!]`: prompts written (`assets/source/audio/animal_prompts.json`) but no voice generated, no credits spent; insect group stands in (`VoiceGroups`). Blocked on user: ElevenLabs key missing, MOSS-SoundEffect under evaluation. `THIRD_PARTY_NOTICES.md` unchanged. Mechs can't pass closed doors yet (CBT-4 adds bashing); mech groups don't use the pod hook.

## SHT-MIN-RSC-0-gap-analyses
Gap analyses of shooting, mining and rescue · 2026-11-05 · 84f1acb97
- Changed: docs only. New `.agents/gaps/SHOOTING_GAP_ANALYSIS.md`, `MINING_GAP_ANALYSIS.md`, `RESCUE_GAP_ANALYSIS.md` (leverage ideas, sourced numbers, rows, phases). Corrected COMBAT, DEFENCE (lines 37, 137), PAWN_INTERACTION gap docs; `DOCUMENT_INDEX.md` three new rows. User decision (2026-11-05): plan approval extends ENG-003 to SHT-*, MIN-*, RSC-* (`AGENTS.md`).
- Decisions (findings): rock HP is published for all five stones (no original tuning needed); gun cooldown is not a gap (Aiming Time unaffected; only Biotech has a cooldown factor); stone falls to half per tile, not a quarter (ours 10 blocks/chunk, wiki 20 at 25 % of chunks); plain-rock mining isn't the worst (steel tile 4.2× longer, steel vein ≈ 7× ours).
- Changed (new rows): capturing a downed neutral has no cost (RSC-203); shooting a downed target has no rule (SHT-008); `AimCooldown` slows melee (SHT-103); scanners work under a roof (MIN-304). RSC-205 (doctor repeating an unreachable trip) from code reading, no test yet.
- Limits: Doctor, Capture, Medical bed wiki pages returned 404; Traits, Events, Factions rows came via a summary and say so; re-read before a phase uses them. No Go changes, no checklist ticks, no HUD (docs only).
- Open: lethal thick cave-ins, and adopting all mining numbers at once (recommendations in the mining doc).

## EAI-0-enemy-ai-contract
Enemy AI contract and the `rage` extraction · 2026-11-09 · f8a109751, 36e7840ec
- Changed: animal aggression moved into `game/rage` (hash-neutral). The shared hostility rule: a wild or enraged tame animal hunting a colonist, guest or tame animal is a foe for colonists, guards and turrets. Skeletons for `rage.Memory`, `wealth.Tracker`, raid `Senses`, the incident queue, letter choices, `Faction.Enemies`; read models `AnimalView.Bashing/Grief/Defending`, `MessageView.Repeat`, `LetterView.Choices`, `Game.StoryDebug()`, `Game.WealthHistory()`.
- Save impact: format v17 (wealth tracker seeded, `Faction.Enemies` filled by `migrateSave`). Seed 1 benchmark hashes and seed 7's 120 fixtures moved on purpose: a wild predator hunting tame animals is now hostile.
- Open: none.

## EAI-1-animals
Animals: door bashing, targets, herds, packs, grief · 2026-11-09 · 5c8c4ca07, 25bfd0d6e, 4db4d03c4
- Changed: shut-out manhunters, predators and insects bash doors (insects also walls) and give up after a time (EAI-001); manhunters pick standing people and tame animals by rings, lock and switch only for a closer foe (EAI-002); a hurt animal calls its herd (EAI-003); predators take downed colonists where the difficulty allows, and livestock is defended (EAI-004); packs are sized by threat points (`animal.ThreatCost`, EAI-005); a grieving tame manhunter is a foe (EAI-006).
- Perf: the breach search first flooded whole boxes for foes no door led to; it now runs only for foes worth breaking in for, after a region check, within a box and node budget (25bfd0d6e, 4db4d03c4).
- HUD/UI: `AnimalView.Bashing`, `.Grief`, `.Defending`.
- EAI-007 journeys `TestJourneyEAIManhunterPack` and `TestJourneyEAIPredatorCharge` landed later in 5917ec06a.

## EAI-2-raiders
Raider sight, turret danger and indexed picks · 2026-11-09 · c334e2e9e, 24c080e11, 61967cbd7
- Changed: raiders advance on home until they see a foe, remember it 600 ticks, share group sight 1200, and turn on a shooter (EAI-010); raiders and colonists step out of a sparking turret's blast (EAI-012); standing counts once a tick (EAI-020), picks every 15 ticks (EAI-021), indexed `NearestThreat` (EAI-022), thief, burner and kidnap-guard scans through indexes (EAI-023).
- HUD/UI: `PawnView.Advancing` on the colony sheet; a sparking-turret alert.
- Perf: `BenchmarkTickRaid` about 20-30 % faster a tick; Raid/500 299k -> 198k ns/op in the lead's interleaved run (812c036fb).
- Decisions: the raid fixture's movement changed on purpose (sight-limited advance, 1-in-15 picks).

## EAI-3-points-wealth-factions
Threat points, wealth, factions at war, visitors defend · 2026-11-09 · f8b8c4a79, ab42be706, 992629ff5, cc8ba7a0d
- Changed: `ThreatPoints` capped at `PointsCap` 10000 and colonists weighted by a wealth curve of our own (EAI-013); `wealth.Tracker` sweeps in slices, one pass an in-game hour, 60 daily samples (EAI-014); `Faction.Enemies` by a seeded rule, rival raid groups fight, a beaten raid thanks the attacker's enemies (EAI-015); visitors and trader guards fight raiders and hostile animals (EAI-011).
- Decisions: no wiki number is used; the curve and cap are marked original tuning. No new RNG stream. Wealth sweep state is saved so a mid-sweep save loads into the same hash.
- HUD/UI: the History window's Wealth tab; the Factions window lists enemies.
- Save impact: v17; `saveV12Hash` re-pinned (the migration fills wars), `TestHabitsOffKeepTheHash` re-pinned twice.

## EAI-4-story-and-hud
Incident queue, letter choices, message merge, positive bias, story panel, infestations · 2026-11-09 · 26621c22c, 505e52d8c, 3cee1b30a, 812c036fb
- Changed: saved incident queue (EAI-030); letter choices and the refugee-chased event (EAI-031); repeated plain notices merge into xN within 600 ticks (EAI-032); positive-event bias on setup colonies (EAI-033); `devtools/story.go` story panel (EAI-034); infestations search the whole map for hive spots, or the story sends a raid (EAI-035); raid alerts name the plan (EAI-016).
- Limits: game-package ceilings raised in 812c036fb (user-approved, perf holds): Grown/250 +6 %, Raid/500 -34 %.
- Open: the legacy storyteller keeps its old draws and pack rolls.

## DEF-3.1-threat-weighted-raider-targeting
Threat-weighted targets audit, combat log kinds, History Threats tab and Damage taken · 2026-11-21 · 768e72901
- Changed: the History window has five tabs (Events, Combat, Threats, Records, Wealth); Threats lists the threat entries. Records has a Damage taken column (humanlike only; dropped on death). A blast's kill and damage credit its thrower once and set `LastBy`, so a raider shot by a grenadier turns on it.
- Decisions: `LogKind` (LogUnsorted, LogFight, LogThreat) with save v19; v18 entries load as LogUnsorted. `Gear.Count` skips the target side on a killing blow (`Died` runs first). Animals and mechanoids never reach `Died`, so they carry no taken damage. Row 39 closes by audit: no targeting or cover change; the four AC-1 tests pass under -race (2026-11-21).
- Save impact: SaveVersion 18 to 19 (`save.go:123`); v18 saves load with zero DamageTaken (`TestDef31SaveV18Loads`). Pins moved, each with a DEF 3.1 comment: 48x40 seed 42, 120x120 seed 1, sense_joy seed 2, scenario grown. `saveV12Hash` unchanged.
- Perf: BenchmarkTickRaid, interleaved n=6: 3.185 ms to 3.167 ms (p=1.000), allocs 8 to 8.
- HUD/UI: History Combat and Threats tabs; Records Damage taken column; en.json adds "Damage taken" and "No threats lately.".
- Boxes: DEFENCE gap row 39 `partial` to `done`; row 40 `partial` to `done`; row 197 Combat records, Kills and Damage dealt/taken `[x]`; §3.10 gains two open rows (finer DPS/range threat score; kits of escaped raiders keep DamageTaken and LastBy, N5). UI checklist 24.3 Combat records `[-]` to `[x]`, Damage dealt/taken `[ ]` to `[x]`, Threat/event history `[ ]` to `[-]`. `spec-backlog` #66 stays open until the lead's last commit closes it.
- Open: DEF-3.1 follow-ups: the finer DPS/range threat score (§3.10); N5 (kits of escaped raiders). Threat/event history still lacks a graph, a time-range control, and infestation or manhunter entries.

## CBT-3.2-ranged-hit-chance
Ranged hit chance, aim traits and interception by distance · 2026-11-22 · b0b462fd1
- Changed: a ranged shot's chance comes from one score (Shooting level plus shooting points, then the per-tile curve). Careful shooter +5 score and aim ×1.25; Trigger-happy −5 and aim ×0.5. Sight adds (sight − 100 %) × 12 points, capped at 200 %; Nearsighted −6. Interception is none within 5 tiles of the shooter and full from 12, for every creature, allies included.
- Decisions: distance is `weapon.Range` (Euclidean, rounded), the metric hit chance already uses. The 5..12 linear scale, the sight cap and Nearsighted −6 are original tuning (CLEAN_ROOM). `attack.FriendlySafe` and `Report.Offset` are gone, subsumed by the distance rule. Melee cooldown and punch are never scaled by aim (SHT-103). Mortar behaviour unchanged.
- Limits: `make arch` passes and `internal/arch/solid_budget.txt` is untouched. `internal/application/game` stays inside its ceiling: 176 of 176 files, 52803 of 52808 non-test lines (52806 before).
- Boxes: SHT-002, -003, -005, -101, -103 `done` in `SHOOTING_GAP_ANALYSIS.md`; COMBAT_GAP rows 51 and 55 `done`; DEFENCE_GAP rows 34 and 139 reworded (state unchanged); `combat/AGENTS.md` Shots and Contracts reworded. Backlog: no row names these items, none ticked. spec-backlog #64 is left open for the lead.
- Save impact: none (derived stat). No golden hash re-pinned. Tests re-scripted for the distance rule: `TestFriendlyFire`, `TestFriendlyFireInTheGame`, `TestHostileTurretShotSides` (colonist 3 moved from 2 to 12 tiles from the muzzle).
- Perf: BenchmarkTickRaid/500, 8 interleaved runs: 268.5 -> 266.4 µs (p = 0.505), max ms per 3 ticks 3.724 -> 3.761 (p = 0.798), allocations equal.
- HUD/UI: the hit card and the dossier shooting line show the new score through existing read models; no new screen.
- Test budget: CI's split (`scripts/ci_test.sh`: game and ebiten under -race -p 1, no -cover) is the 10-minute budget; game -race alone took 114 s here. `make test-unit` adds -cover and ran past 15 min on this machine at load average 11, unrelated to this change.
- Open: SHT-001, -004, -006 to -008, -104, -105 and the SHT-2xx and SHT-3xx rows stay open in `SHOOTING_GAP_ANALYSIS.md`. SHT-102 stays open: `combat.AimTicks(d)` still ignores the pawn at its callers.

## CBT-3.4-multi-part-cuts-and-pain
Multi-part cuts, inner blunt strikes and wiki pain constants · 2026-11-22 · 8335654fe
- Changed: `body.StrikeBlow` (`internal/domain/pawn/body/split.go:162`) splits a cut over 1-4 parts (30/45/20/5 %, x1.00/1.41/1.64/1.78, even split; an inner-part cut also hits its parent, bones excepted) and a blunt blow into an inner strike 40 % of the time (outer keeps 80-90 %; solid parts excluded). Humanlike is dead at Damage >= 100, a lost vital part included; animal plans die at wounds plus lost-part HP >= MaxHP; a killing share stops the rest. `SpreadBlow` does the same for animals. Pain is 12.5 permille per HP, burns 18.75, scars 6.25 (`body/pain.go:13`); `PainShock` stays 800 (`health/health.go:154`, `pain_shock_test.go`). `Blow.Split` (`game/hit/hit.go:88`) is set for cut and blunt forms only; bombs, bullets, arrows, spikes, fixtures, roof crush and fire stay unsplit.
- Decisions: numbers come from the wiki pages Damage Types, Injury and Pain (ENG-003). Original tuning, marked in comments: adjacent and coverage-weighted extra picks, blunt redirect shares, fresh-stump pain. The vital `LossKills` cap binds every share after the first. The `game` Injury clamp keeps Injury from falling on a blow.
- Re-pins: `TestGrownGolden` 0xbace7519d533d57e to 0x787620b960ac426d (the split moved it, not pain; overlay-checked by QA). `humanFingerprintHash` (`body/anatomy_test.go`) to 0x55011ab9b54b818d (scar pain 1 to 6.25 permille moved fixture #52's pain 520 to 625). The 6 `TestScaleGolden` pins are unchanged.
- Limits: `internal/application/game` stays at 52803 lines in 176 files; `solid_budget.txt` untouched.
- Perf: BenchmarkTickRaid/500 507.2 to 351.7 µs (p = 0.002, n = 6 interleaved). No regression; the speed-up is not a real gain, fights play out differently.
- Tests and CI: domain `TestCut*`, `TestBlunt*`, `TestStrikeBlowVitalAndDeath` (5000 seeds, all plans), `TestSplitScaledBody`, `TestPainPerKind`; game `TestKnifeHitSplits`, `TestBlastWoundsOnePart`, `TestInjureCutUnsplit`, `TestAnimalCutBlowSplits`, `TestAnimalFixtureBlowUnchanged`, `TestAnimalSplitExtraKills`, `TestAnimalSplitInjuryNeverShort`, `TestAnimalSplitNegativeRiseClamped`. CI split: game under `-race -p 1` in about 128-148 s, ebiten alone about 139 s, the other 168 packages pass. `-race -cover` on game alone ran 1006 s (93.3 % coverage); CI does not use -cover on game (gate override). New domain code coverage 96.9 %.
- HUD/UI: the Health tab already lists wounds per part, so extra cut and inner wounds show there; no new UI.
- Boxes: COMBAT_GAP rows 78 (multi-part cuts, blunt inner strike) and 81 (pain constants) `partial` to `done`; MEDICAL_GAP row 49 and backlog Pain line reworded to the new rates (state unchanged). MEDICAL_GAP row 72 Today column refreshed. `spec-backlog` #65: both boxes met; closed by this change.
- Open: #269 animal Injury and body drift: tend and natural heal lower Injury while lost parts stay (`animal_health.go:165`, `:212`), so the animal body-dead rule can stop a split early on a healed animal. #270 Scratch, Stab, Bite and projectile multi-hit rules from the wiki. #271 A crack wound kind (1.0 %/HP) does not exist. #272 `lossDamage` duplicates the `Damage()` loop; kept for the Body method budget.

## CBT-272-dedupe-loss-damage
One loop for body damage and the animal death rule · 2026-11-23 · 772b349e6
- Changed: `Body.Damage()` (`internal/domain/pawn/body/wound.go:222`) now calls `lossDamage` and only adds the vital-loss floor; `lossDamage` moved beside it from `split.go`. Values are unchanged for every caller.
- Decisions: `lossDamage` stays a free function so `body.Body` gains no method.
- Limits: `body.Body` stays at 99 methods (`internal/arch/solid_budget.txt:20` untouched); `make arch` green.
- Boxes: none: no backlog, spec or gap row names this cleanup; issue #272 (not `spec-backlog`) closes with this change.
- Save impact: none. HUD/UI: none (internal refactor, no player-visible change).

## SHT-75-shooting-gaps
Shooting gaps: accuracy inputs, downed creatures, friendly fire, aim, hit card · 2026-11-23 · 264bfdb4b
- Changed: manipulation adds shooting points (weight 8, capped at 100 %); the curve has all 21 published points. A downed creature in a shot's path can intercept it, at 250 ‰ of the standing chance; a shot at a downed target is ×8.5 within 3 tiles and ×0.5 from 5. Every first aim scales with the shooter's aiming time. The new trait Scrapper (melee +4, shooting −10) sets the brawler melee bonus. Cover: fence gate 75 %, lamp 20 %, torch 15 %, plant pot 20 %. Daily skill XP is full up to 4000, then 20 %, for every skill; decay follows the published per-level table.
- Changed (UI): the hit card's lines add up to its headline and it shows for any selected colonist; a warmup ring sweeps on each aiming shooter; Friendly fire sliders in the new-game and story options screens.
- Decisions: `DownedIntercept` 250 ‰ is original tuning (no published number). Friendly fire scales only an aimed colonist shot crossing a colonist, in interception and the landing stray; visitors and leaving raiders are not colonists here; turrets, raiders and wild shots are never scaled. A downed animal no longer takes a landing stray (the shot falls through to the building roll). The XP soft cap works in hundredths through `Carry`, so small grants past it keep their 20 %.
- Limits: the ring sweeps `AttackCycle`, so a repeat shot shows cooldown + aim as one sweep. The soft cap counts gain before decay. `internal/application/game` 52803 of 52808 non-test lines, 176 files; ebiten root 12168 of 12180, 60 files; `solid_budget.txt` untouched.
- Save impact: save v20. `FriendlyFirePct` is new; v19 and older saves load at 100 %; codec key `ff` (an old string decodes to 100). The skill sheet's saturated cell now means past the soft cap. `MigrateDifficulty` (pre-v9 saves) no longer misses its preset on fields added later (quest settings since v15, friendly fire now).
- Re-pins: `TestScaleGolden` (all 6 seeds), `TestGrownGolden` 0x787620b960ac426d to 0xd184896f98be7431, `TestHabitsOffKeepTheHash` both seeds, `saveV12Hash` and `testdata/save_schema.txt`, `TestLearningSaturation` (9200 a day). New games shift with the appended trait; Legacy-rule seeds do not.
- HUD/UI: AC rows above; downed interception has no new HUD line (hit trace and log entry show it).
- Boxes: `SHOOTING_GAP_ANALYSIS.md` SHT-001, -004, -006, -007, -008, -102, -104, -201, -202, -203, -204, -301, -302, -303 `done`; SHT-002, -005, -101, -103 re-checked (citations refreshed). Vanilla checklist 17.3 **Small-object cover**, 17.4 **Shooting score inputs**, **First aim scales with aiming time**, **Downed creatures in combat**, **Friendly Fire difficulty**, **Hit-chance card**, 17.5 **Brawler effect** added `[x]`; **Skill XP** reworded. `spec-backlog` #75: every box met; closed by this change.
- Open: #273 SHT-105 Nimble trait. `make art-check`'s Python step needs Pillow on this machine (its Go steps pass). A hand-edited out-of-range `ff` shows unclamped in the sliders (combat clamps it).

## SHT-105-light-footed-trait
Light-footed trait: the Nimble melee dodge · 2026-11-23 · #273
- Changed: the trait Light-footed (original name `LightFooted`, `trait/trait.go:76`) sets the Nimble melee dodge; its Bio-tab note reads "Melee dodge +15" (`trait/trait.go:250`); the skill sheet "Melee dodge chance" line rises by the curve (`dossierview/build.go:107`).
- Decisions: original name; no group, needs or rarity; score not percentage (+15 dodge score at `attack/melee.go:106`); trap-spring moved to `SHT-105-nimble-trap-spring`.
- Decisions also records: the trap-spring ×10 % shipped in `SHT-105-nimble-trap-spring`.
- Limits: `game` non-test lines 52805 of 52808 after the change, 176 files; `solid_budget.txt` untouched.
- Save impact: none, append-only ID remapped by key.
- Re-pins: `TestScaleGolden` 48x40 seed 1 0x4c0442b8090a2e0c to 0xa7d602efa9a46d1e, seed 7 0x408ae14ed8f82600 to 0x67bd5ef1a52e30f6, seed 42 0xbde998be94577c83 to 0x2ee993824248e785; 120x120 seed 1 0xd98331cd4d373f4 to 0x753046d8e53f6773, seed 7 0xc6a20957c3d8cdc3 to 0xc0d088d53929de1e, seed 42 0xd3415cf2f5606139 to 0x3289ef2592a0ea67. `TestGrownGolden` 0xd184896f98be7431 to 0xb58aab09cdc09002. `TestHabitsOffKeepTheHash` seed 1 0x2caa5c32e507bb85 to 0x9c7aa34af35f45ff, seed 2 0xe4fd24b373892f01 to 0x063e5c9daeead1f9. New games shift with the new trait; Legacy-rule seeds do not.
- HUD/UI: the existing Bio-tab trait note and the skill sheet "Melee dodge chance" line; no new screen.
- Boxes: gap SHT-105 `done`, checklist 17.5 **Nimble effect** `[x]`, no `spec-backlog` issue.

## SHT-105-nimble-trap-spring
Nimble trap spring chance, gosec notes, pinned tools · 2026-11-23 · #273
- Changed: a Nimble victim (Light-footed) springs any trap, IEDs included, only 10 % of the time; a miss leaves the trap set and shows no message. The Bio-tab note reads "Melee dodge +15; rarely springs traps" (`trait/trait.go:250`). A full 255-trait book now rolls all 255 traits (before: none).
- Decisions: the chance is `trapNimblePct` = 10 of 100, the wiki's Nimble ×10 % (`combat/defense.go:41`); the roll is drawn only for a Nimble victim on a trap, before the IED branch so charges count, on the main stream (`h.Roll`, `defense.go:87`); a book of ≤254 traits generates the same pawns as before (`Generated` bounded by min/max, `trait/book.go:116`).
- Limits: `game` 52805 of 52808 non-test lines, 176 files; `solid_budget.txt` untouched.
- Save impact: none. Re-pins: none; `make test-golden` and the Golden and HabitsOff tests pass unchanged.
- Tooling: `#nosec G103` on the unsafe memcmp view (`game/indexes.go:492`); the plant sprout result is discarded with `_, _ =` (`game/plant_light.go:214`); staticcheck 2026.2.2, gosec v2.30.0 and govulncheck v1.9.0 are pinned, and the loops gate's staticcheck step runs (0 findings).
- HUD/UI: the Bio-tab Note only; no new screen.
- Boxes: gap SHT-105 `done` and checklist **Nimble effect** `[x]`, both reworded; no `spec-backlog` issue.
- Open: #275 repo-wide gosec v2.30.0 baseline (915 issues).

## CBT-3.4b-scratch-stab-bullet
Scratch, Stab and Bullet multi-part rules; Bite and Arrow pinned single-part · 2026-11-24 · 3b7084330
- Changed: a Scratch splits an outer blow over 2 distinct parts at 67 % each, an inner one over its chain (`internal/domain/pawn/body/scratch.go:73`). A Stab retargets an outer part to an inner child 60 % of the time, never the brain; the chain takes 40 % per inner part and 75 % to the outer parent (`stab.go:24`). A Bullet on an inner part adds the full hit to its parent (`bullet.go:16`). Bite and Arrow stay single-part. Animals (SpreadBlow) get the same plans.
- Decisions: wiki numbers only (ENG-003); every other choice is marked `original tuning` in its comment (distinct picks, post-armor base for Stab, brain remap, rounding). Spike traps, gore and spears are Stab; claws and pecks are Scratch (`domain/combat/weapon/hit.go:26-27`).
- Limits: game code changed only in comments (`game/body.go:63-64`, `game/hit/hit.go:88`, same line counts); `solid_budget.txt` untouched.
- Save impact: none; `Split` is not persisted.
- Re-pins (new body-stream draws and shares, each with a dated cause comment): `scenario/grown_test.go` state hash, `game/sense_joy_test.go` habits-off hashes (seeds 1, 2), `game/scalegolden/golden_test.go` 48x40/seed42 and 120x120/seed1; `game/wildlife_test.go` reads the blow's trace damage (claw is Scratch: 2 x 67 %); `game/raid_def4_test.go` no-flap loop ends once Ada leaves alive (bullet bonus downs her sooner); `game/combat_test.go` wolf duel seed 5 -> 6 (win rate unchanged over seeds 1-40).
- HUD/UI: none; the Health tab already lists wounds per part, so the extra parts show there.
- Boxes: COMBAT_GAP rows Scratch, Stab, Bite, Bullet, Arrow `done` (new), row 78 "Not done" updated, §4 and §5b lines added; backlog **Damage types** `[x]` kept, annotated. spec-backlog: none (#270 has no spec-backlog issue).
- Open: #278 Poke and RangedStab (no form in the game).

## CBT-3.4c-poke-rangedstab
Poke and RangedStab damage forms: Stab variants and their weapons · 2026-11-24 · 142d64e1c
- Changed: a Poke (club and mace handles, a spear's second shaft, an ikwa's second handle, each bow's limb) is armored as blunt, wounds as a bruise, splits as Stab with a 40 % inner chance, never stuns and never wounds the brain (`internal/domain/pawn/body/stab.go:32`, `internal/domain/combat/weapon/tool.go:44-82`, `internal/application/game/hit/hit.go:181`). A RangedStab (the pila) is sharp for armor and reaches the brain (`stab.go:35`); a shield belt stops it and a smokepop pops on it (`internal/application/game/combat/gear.go:260`, `:274`).
- Decisions: wiki numbers only (ENG-003): Poke 40 % gate, RangedStab reaches the brain. One `planStab` reads a `stabRule` row (`gate`, `brainBar`) per split, with no flag (`stab.go:20-36`). Original tuning, marked in comments: Poke's Bruise wound and brain bar, the 500/500 bow limbs, and the belt stopping RangedStab as an arrow (K5). The wiki's table says "Affects shields: no" for RangedStab; this does not follow it.
- Limits: no new `Body` method; `internal/arch/solid_budget.txt` untouched (`Body` stays at 99, line 20); `internal/domain/pawn/body` has 38 non-test files (limit 60).
- Save impact: none.
- Re-pins: none (no application or golden test shifted).
- HUD/UI: none; the Health tab already lists wounds per part, so a Poke or pila wound shows there.
- Boxes: COMBAT_GAP rows Poke and RangedStab `done` (new); overkill windows (row 77, §4) and the multi-part line (row 78, §4) name them; §5b's #278 line becomes #279; backlog **Damage types** `[x]` kept and annotated. Issue #278 (no checklist): done in 142d64e1c, closed once pushed.
- Open: #279 hooves (Kick, `internal/domain/animal/animal.go:187`) are not moved to Poke.

## CBT-269-animal-split-room
An animal's split blow stops at its own hit points, not the body's lost-part total · 2026-11-24 · 4f5ce58fd
- Changed: a split blow on an animal (cut, blunt, scratch, stab, bullet) stops once its rise reaches the hit points the animal had before the blow, the number `Animal.Dead()` reads (`body.SpreadBlowWithin`, `internal/domain/pawn/body/spread_blow.go:39`; called at `internal/application/game/animal_health.go:70` with room `a.HP()+dmg`). A healed animal with a lost leg is no longer cut short while it still has hit points, and a starved animal's lethal split stops at the share that kills it.
- Decisions: the ticket's "or" branch (the stop rule reads `Injury`), not a healed-lost-HP offset on `Body`: no new field, no save change, no new RNG draw, and it also covers starvation (Injury above the body total). For an undrifted animal the two rules are equal share for share (killAt == MaxHP, room == killAt - lossDamage), so goldens hold; `TestSpreadBlowWithinMatchesBodyRule` pins that. `SpreadBlow` keeps its behaviour for pawns and existing callers. Injury and the body total may still differ after a natural heal (`naturalHeal`, `vet_animals.go:120-132`); nothing authoritative for death reads the body total on animals now.
- Re-pins: `TestAnimalSplitStopNeverLowersInjury` and `TestAnimalSplitNegativeRiseClamped` hand-built the drift state (the bug) and failed unmodified; both re-seated with their invariants kept (exact rise on an early stop; Injury never falls; the negative `SpreadBlow` return stays pinned at seed 7). `TestAnimalCutBlowSplits` and `TestAnimalSplitExtraKills` now take `SpreadBlowWithin` as reference. No golden shifted.
- Limits: no new `Body` method (99, `internal/arch/solid_budget.txt:20`); game package non-test lines 52807 of 52808 (`:15`).
- Save impact: none. HUD/UI: none; the Health tab already shows the animal's wounds and hit points.
- Boxes: COMBAT_GAP multi-part row (row 78) `done` kept, its animal sentence now names `SpreadBlowWithin` and #269. spec-backlog: none (#269 has no spec-backlog issue); #269 is closed on push.
- Open: #280 the part-loss cap (`LossKills`, `internal/domain/pawn/body/anatomy.go:108`) still reads the body total on a healed animal.

## CBT-10-prisoner-state
Prisoner state's escape behaviour was already live (CBT-4); proved through the tick and ticked · 2026-11-25 · d8eb1d00d
- Changed: no behaviour. #10 reported "no escape behaviour", but CBT-4 (`73ffc63cd`) shipped prison breaks (`internal/application/game/prison/escape.go`) and ticked 19.4 **Escape attempt** and its COMBAT_GAP row, not 19.1 **Prisoner state**. `TestLiveStartsPrisonBreak` now drives `Prison.Live`, which the raid phase runs every tick for each prisoner (`raid/raiders.go:45`), into the roll: a hit on the prisoner's hourly check past the grace starts a break of its room; a miss or a downed prisoner starts none. Before, only `breakOut` was tested directly.
- Decisions: no change to who may start a break or to the original timings (60-day mean time, 3-day grace, 10-day calm).
- HUD/UI: the existing "Prison break" threat alert (`prison_host.go:183`); the Prisoner tab still shows no break history (UI spec Wave UI-5 **Prisoner** `[-]`, unchanged).
- Save, determinism, mod impact: none.
- Boxes: backlog 19.1 **Prisoner state** `[-]` → `[x]`. spec-backlog #10: its one line is met; closed by the PR.
- Open: a sleeping prisoner can start or join a break (`prison/life.go:64` runs after the downed return only); whether starters must be awake is undecided, recorded as a finding of the #10 run (PR Open follow-ups).

## CBT-3.4-core-scalar-integration
Reconcile multi-part damage forms with the native Core damage owners · 2026-11-25 · this commit
- Changed: the appended Scratch, Stab, Bullet, Poke and RangedStab split forms use the same upstream share planner for the live animal damage owner; the scalar injury budget, not historical anatomical loss, stops the remaining shares. A selected replacement still receives its own direct damage.
- Decisions: share the upstream split predicate for all seven multi-part forms; retain upstream form IDs, armor categories, weapon tables, share rules and draw order. No new balance decision or saved field.
- Proof: new per-form tests cover healed anatomical loss, scalar stop, replacement isolation and actual HurtHit animal dispatch. Prior six scale pins, two habits-off pins and grown pin remain unchanged on the combined source. The wolf duel retains the original kill/corpse assertion on seed 5; the intervening upstream seed 6 now explicitly proves a downed wolf and active attack at the same 2,000-tick bound. Both outcomes were reproduced before adjusting the fixture.
- Limits: game root 52634 of 52650 non-test lines, no raised budget.
- Save impact: none; neither the detached save capture nor canonical encoding changed in the reconciliation.
- HUD/UI: existing wound/health views show the native per-part results; no new screen.
- Boxes: existing CBT 3.4b/c damage-form rows remain done; no new checklist feature or spec-backlog issue.
- Open: upstream #279 remains outside this contribution.


## CBT-269-core-placement-accounting
Reconcile upstream owner-room split accounting with physical placements · 2026-11-25 · this commit
- Changed: adopt upstream `SpreadBlowWithin` (57ce81e7a parent); remove duplicate `SpreadAnimalBlow`. Keep one native split planner, body-owned `SpreadBlow`, ordinary unsplit `Spread`, and replacement isolation. In the scalar-owner path count the damage newly accepted by a part, retain unapplied damage separately, and stop after the placement that exhausts pre-hit HP, including deferred spill. Preserve whole-share overkill semantics and the bounded root fallback.
- Proof: a planned 17-point cut that destroys a leg with a 25-point child wound places 17 new points; a blocked heart and deferred bullet spill conserve the actual scalar placement budget. Body Race and game healing, starvation, five-form HurtHit, replacement and wolf regressions pass. All 2,730 parity inputs are retained: 2,450 match legacy body and draws, and the 280 changed outcomes use an independent physical-placement oracle.
- Decisions: the owner-room threshold is scalar HP, not anatomical net damage. New damage is not refunded when a child disappears in the same blow. Preserve prior body-owned behavior and its tests. New scalar placement can change RNG draw consumption when legacy spill or nominal early stop differs; no change to the RNG generator or planner. The upstream seed-8 healed-limb fixture preserves Injury 28 with 7 surviving wound points and anatomical net rise 21, rather than requiring the anatomical sum to equal Injury on a new loss.
- Qualification: the body and game regressions, all nine scale/habits/grown pins, architecture and static checks pass.
- Bench: all seven healthy quadruped split forms improved in three interleaved samples on Ryzen 7 9700X / Go 1.27.4, e.g. form 1 499.5 to 205.2 ns/op and form 7 248.7 to 134.0 ns/op in the controlled microbenchmark; allocation counts and bytes unchanged.
- Limits: game root 52633/52650 non-test lines; no raised limits or new Body method.
- Save impact: no schema, persistence or canonical encoding change. HUD/UI: existing native health view uses the resulting per-part wounds and scalar HP.
- Boxes: COMBAT_GAP row 78 remains done, updated for scalar placement accounting; #269 (not spec-backlog) is addressed in the upstream parent. No new checklist feature.
- Open: #280 remains open; the healed-limb sever fixture passes, and LossKills checks vital ancestry and organ failure. #279 remains outside this scope.

## MIN-1-core-mining-materials
Native mining labour, capacities, yield and stonecutting · 2026-11-25 · this commit
- Changed: actual material-specific mining work, native mining skill/capacities and shared wall/drill output; one-in-four plain-rock chunks, stonecutting and chunk masses. Scanner site ore cells use the same material yield.
- Decisions: published MIN-1 table, original positive-stack rounding; append ore IDs without renumbering, preserve mining stream across save/replay.
- HUD: existing tile/job/item views expose the resulting real ore, labour and stack quantities; no new screen.
- Boxes: MINING_GAP MIN-101 through MIN-109 done; MIN-301 open (scanner-site ores only). #68 partial.
- Open: #68 MIN-2 roof collapse and remaining MIN-3/4 rows, including ordinary surface ore worldgen.

## SHT-1-core-shooting-reconciliation
Native shooting, interception and combat traits · 2026-11-25 · this commit
- Changed: manipulation/sight aim points, distance and downed interception, native friendly-fire settings, Scrapper trait effects and daily XP cap/decay; retain real weapon and armour owners.
- Decisions: preserve existing form/trait IDs and draw order where semantics unchanged; retain original downed intercept tuning. New-game friendly fire 40, explicit v20 values and older 100 preserved.
- Save/HUD: settings in new-game/story options, existing Bio/Health/skill read models; typed v20 migration and causal golden evidence.
- Boxes: SHOOTING_GAP SHT-001/003/005/006/007/104/302 done (existing rows corrected); vanilla **Downed creatures in combat**, **Friendly Fire difficulty**, **Brawler effect** [x].
- Open: #279 hoof Poke mapping remains outside this contribution; #280 handled in CBT-269 ledger.

## DEF-4-core-weighted-threat-census
Weighted threat points and independent map/caravan budgets · 2026-11-25 · this commit
- Changed: native weighted resident/animal/goods census, owner-scoped colony threats and summed independent permanent colony/caravan budgets for quests. Keep shared adaptation and bounded points.
- Decisions: no pooled nonlinear wealth curve or double-counted off-map assets; temporary quest sites excluded. Existing native faction selection and spawn ownership remain.
- Save/HUD: persisted adaptation/census inputs, existing History wealth and raid/quest presentation; actual multi-map/caravan/stasis regressions.
- Boxes: QUESTS_GAP QST-025 done; DEFENCE_GAP §3.8 raid points and #252 partial.
- Open: #252 remaining threat-curve and census parity.

## MIN-3-native-ore-and-scanner-rules
Surface ore, scanner admission and mining geometry · 2026-11-25 · this commit
- Changed: seven surface ore profiles; size ranges; bounded connected veins; reject undersized lumps; roof checks on both scanner types at start and work; auto-forbid drill yield; cardinal mining cells.
- Decisions: mapgen version 3 intentionally changes new map generation; legacy saved geometry unchanged. Rare ore densities are local tuning. Old ore implementation alone reproduces all prior map goldens; 11 changed pins recorded, sea ice unchanged. Scanner footprint check allocation-free.
- Boxes: MIN-301..305 and MIN-308 done; refs #68.
- Open: MIN-201..207 collapse and damage, MIN-306/307 infestation, MIN-402 risk projection; MIN-401 presentation remains open.

## MIN-306-307-native-infestation
Native infestation eligibility and pacing · 2026-11-25 · this commit
- Changed: both storytellers use live strict infestation admission, thick rock, within 30 tiles of colony structure, no one-cell room, above -17 C, relative cold and light weights. A successful event sets a saved 20-day deadline; no event at the hive cap.
- Decisions: original relative weights for cold/light; one weighted draw only for valid candidates; explicit dev swarm retains its previous fallback and spread behavior. Save field defaults to zero.
- Boxes: MIN-306 and MIN-307 done; refs #68.
- Open: roof collapse and mining risk projection; presentation remains open.

## MIN-2-native-support-cave-in
Native structural support loss, damage and risk projection · 2026-11-25 · this commit
- Changed: circular six-cell support, real support removal queued in saved map state, revalidation at climate pass, thick rock collapse and repeat mining, thin roof crush damage, rubble dirt and stack damage; read-only MiningRoofRisk.
- Decisions: original loose-stack damage amounts; jaw/brain approximate head/neck targets. Voluntary roof removal is nondamaging; no roof-only support chains. Existing natural roofs without a loss event are not destroyed on load.
- Save/HUD: additive RoofLoss with owned snapshot copy and normalization. Native danger letter and collapse stamp; overlay and pause remain open. Historical replay hashes retain the square representation of the derived Supported bit only, reconstructed from snapshot terrain/buildings without mutating live roofs. All old v12/habits/scale/grown pins pass unchanged. The recreation workload excludes hazardous mining but retains unchanged sleep/food/work thresholds and asserts injury-free pawns; actual collapse has independent regression coverage.
- Boxes: MIN-201..207 and MIN-402 implemented in native model, MIN-401 presentation separate; #68 native slice. #252 original 24.1 wealth row already implemented and shared across colony/caravan goods.
- Open: roof-risk overlay and auto-pause.

## DEF-15-turret-fuse-hiss
Turret fuse hiss · 2026-11-26 · 696371bd1
- Changed: a turret hisses while it is Sparking, powered or not; the nearest 2 are heard (`amb.turret_fuse_hiss`, the key `fuse:<id>`, `render/cue/machines.go:88`).
- Decisions:
  - a positional loop rather than a one-shot (K2): the loop plays while Sparking holds and fades when the key is left out;
  - read every frame in `Machines`, not memoised in `ListenWorld` (`render/cue/signals.go:59`), which runs only when ticks advance;
  - volume 0.4: above the machine hums (0.3) because it warns of a blast, below the re-arm (0.7) (`audiogen/structure_recipes.go:141`); seed 946 keeps the render byte-equal across runs (`TestTurretFuseHissLoop`);
  - ambience bus, because `BusOf` maps the `amb.` prefix to it (`pkg/audiomanifest/audiomanifest.go:150`).
- Limits: the ebiten root is untouched, and its budget line stays at 12180 (`internal/arch/solid_budget.txt:16`); no `Bus` method was added; one key string is allocated per sparking turret per frame (`TestFuseHissAllocs`).
- HUD: none new, because Sparking already shows as the "Turret about to explode" alert (`hud/alerts_power.go:123`) and as the inspector line.
- Determinism: presentation only, with no save or replay impact.
- Boxes: checklist 2246 `[x]` (bold name); gap rows MULTIMAP:105 (the running, stale since EAI-012) and :106 (Now cells, the table has no state column); the spec-backlog issue #15 stays open for AC-1 (`[-]`).
- Open:
  - line 2234 (Visit friendly settlement: trade and gifts) is ticked once PR #322 merges;
  - `make art-check`'s Python step needs Pillow (PIL), which the machine lacks; it was not run.

## EAI-5-positive-incidents
Ore meteor, Supply drop and Sky song (25.2, #13) · 2026-11-26 · this commit
- Changed:
  - three Nature/helpful incidents, appended after `TransportPodCrash`, all with original names, text and tuning;
  - **Ore meteor**: 4-7 four-connected cells of rough rock with one ore (steel, silver or gold) on open, dry, unroofed, unzoned ground 5-14 tiles from home;
  - **Supply drop**: 2-3 forbidden stacks of distinct kinds, each within its stack limit, near home;
  - **Sky song**: for 1.5-2.5 days every colonist present gets "lifted by the sky song" (+6, `mood.SkySong` id 114), which expires with the song.
- Decisions:
  - the logic lives in the stateless `game/windfall` behind a consumer-owned Host and Dice. Package `game` holds only the dispatch cases and the host (`incidents.go`);
  - Fits uses bounded `travel.Ring` searches and never rolls the RNG. Fire checks its precondition before any draw, so a refused Fire draws nothing;
  - the song is on only while `now < SongUntil`, with no separate flag. It cannot refire while playing;
  - colonists who join mid-song, or are on another map, get no thought (A7, N7). A meteor may wall off a path (N8, accepted).
- Save/HUD:
  - the save gains one additive gob scalar, `incident.State.SongUntil` (0 in older saves), and no SaveVersion bump;
  - the HUD gains a read-only "Sky song, N.N h left" chip from `WorldView.SkySongEnds`, plus one Good letter per incident and three dev-panel entries.
- Limits: the `game` lines ceiling is +66 (52650 → 52716). No new top-level game file. Perf under `-race`:
  - ColonyHour is 149.1 → 149.8 ms (p=0.82);
  - TickGrown/500 rose +16.7% back to back (p=0.002), but in an interleaved rerun the change is not significant (p=0.29) on a loaded machine;
  - the new code runs only on the nature roll, never per tick.
- AC-1 (transport pod crash) was already met by RSC-001; this task only verified it.
- Boxes: checklist "Resource meteorite", "Cargo pod drop" and "Psychic soothe-like temporary positive mood event" are `[x]`. Gap ALLOW F8 is done for cargo pods; quest rewards are open. spec-backlog #13 is completed by this PR.
- Open: saved `...Until` ticks (song, eclipse, fallout) are not clamped on load (backlog finding).
