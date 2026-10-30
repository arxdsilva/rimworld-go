# Defence gap analysis (DEF-0..DEF-4)

Scope: bring ranged and melee combat, armor, cover, explosions, traps, turrets, mortars, doors, drafted behaviour, raids, mechanoids and infestations to RimWorld-wiki fidelity (vanilla core only, no DLC). Load only for DEF-* tasks. Phase handoffs go in `.cursor/handoffs/DEF-N-*.md`. Sister documents: `BEDS_GAP_ANALYSIS.md` and `PRODUCTION_GAP_ANALYSIS.md` (PRD-4 crafts the weapons and armor that DEF-2 defines).

## Status (2026-10-29)

Wave D done for DEF-3 (cover, IEDs, turrets, mortars, doors; handoff `handoffs/DEF-3-cover-traps-turrets-mortars-doors.md`). Wave C done for DEF-2 (weapons, armor, explosions; handoff `handoffs/DEF-2-weapons-armor-explosions.md`). Wave B done (PRD-1/BED-1/DEF-1). Wave A done earlier: the `game/combat` extraction, the combat domain packages and the weapon `Def` extension (DEF-0). Wave E done for DEF-4 (raids, drafted control, mechanoids, ship part; handoffs `handoffs/DEF-4-R1-raids-drafted-control.md` and `handoffs/DEF-4-R2-mechanoids-ship-part.md`). The programme's five waves are done; the gaps that remain are in section 3.10. Every row in section 3 is `open` unless it says `done` or `partial`. Rows are ticked per phase by the lead, in the same commit that ships the phase.

## 1. Decision record

| Item | Decision |
|---|---|
| Goal | The user asked for the beds, production and defence systems "so we can implement what is missing", at RimWorld fidelity. This document covers fighting and the threats the colony fights. |
| Scope | All of vanilla core, no DLC. Breach raids, mech clusters and other DLC content are in section 7. |
| Numbers | ENG-003 in `AGENTS.md` was extended on 2026-10-29 to beds, production and defence. Published RimWorld wiki numbers are used as tuning data. Names and prose stay original. |
| Clean room | No wiki text, code, art, sound or sprite data is copied. Each constant is a named constant with a `// wiki: <page>` comment, or `// original tuning` where the wiki gives none. Section 4 lists them. Weapon, armor and mechanoid designs are original. |
| Art and sound | Art and sounds ship with each phase that adds a weapon, building or creature (skills `rimworld-go-art-assets` and, for mechanoids, `rimworld-go-creatures` with all four sounds: `call`, `attack`, `hurt`, `die`). Sound generation for this effort is pre-approved by the user; the lead states the count before each `cmd/sfxgen` run. Nothing ships silent. |
| New packages | `internal/application/game` is at its SOLID ceiling. Strike resolution, line of sight, cover, defending and turrets move into the concise sub-package `internal/application/game/combat` (DEF-0). Raids stay in game until DEF-4, which moves `raids.go`, `raid_plans.go` and `hive.go` into `internal/application/game/raid` when it adds drop pods and mechanoids. Rule: `.cursor/SOLID_GO.md` section 10; consumer-owned `deps.go` roles of at most 8 methods, adapters in game, one Game field, saved state keeping its gob names. |
| Domain | `internal/domain/combat/{damage,armor,cover,attack}` are empty stubs today (each holds a `doc.go` that says "TODO"). DEF-0 fills them with pure, deterministic code. `weapon.Def` grows warmup, cooldown, burst, penetration and accuracy bands, with today's values kept so behaviour is unchanged until DEF-1. |
| Budget | DEF-0 moves more than its adapters, so package game shrinks first. No later phase may leave package game larger (lines, files, `Game` methods) than the previous commit. |
| Hysteresis | Turret lock, mortar manning, fire-at-will, retreat, siege-to-assault and any threshold-driven AI start and stop at different thresholds or have a cooldown, with a no-flap test (the sentry turret lock is the model: `turretLock` 10, `turretRelease` 12 in `turret.go`). |
| Determinism and hot path | Shot rolls come from the existing defence stream, never a global RNG. Projectiles live in an index iterated once a tick, no map-wide scan. Gates: `cmd/benchmark raid/500/x3` frame p99 and `BenchmarkDrawArt500` draw calls and allocations unchanged for overlays. |

## 2. Reading the tables

Legend for the `State` column: `open`, `done`, `out of scope (needs X)`. `Today` evidence gives a path and a function name; line numbers drift while extractions run, so find the function. Paths are under `internal/application/game` unless a full path is given. Every reference was checked against the code on 2026-10-29. Files named for DEF-0 (`combat.go`, `combat_state.go`, `defense.go`, `turret.go`) are being moved to `game/combat`.

## 3. Gap tables

### 3.1 Ranged and melee attacks

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Shots are projectiles in flight; misses can strike others in the line or the cover; allies can be hit. | `combat.Shots` (`game/combat/shot.go`: `fire`, `Step`, `fly`, `through`, `land`) flies `projectile.Shot` cell by cell; stray hits (`attack.StrayHit`), interception (`attack.InterceptChance`), friendly fire beyond 5 tiles (`attack.FriendlySafe`), save via `Export`/`Import`. Original tuning: an aimed shot hits its target only within `Tracking` (1 tile) of the aim point. | None; natural cover in DEF-3. | DEF-1 | done |
| Each weapon has warmup, cooldown, burst count, range and an accuracy curve over touch, short, medium and long range. | `weapon.Def` bands, warmup, cooldown, burst and burst ticks per weapon in the DEF-2 catalogue (`domain/combat/weapon`); `Strikes.Do`/`next` use them (`combat/strike.go`); burst weapons (assault rifle, machine pistol, LMG, minigun and others) fire bursts through `job.Count`. | None. | DEF-0, DEF-1, DEF-2 | done |
| Armor penetration and damage types (sharp, blunt, heat, bomb, burn, EMP). | `Def.DamageType` and `Def.ArmorPen`; `Strikes.armorRoll` feeds `wound` (`combat/strike.go`). | None. | DEF-0 (domain), DEF-1 | done |
| Hit chance takes shooter skill, target size, lighting and weather into account. | `Strikes.report` (`combat/strike.go`) multiplies band, per-tile shooter accuracy (`attack.ShooterFactor`), `attack.SizeFactor` and cover. Light and weather are fixed at `NoLightPenalty` and `ClearFactor`, not read from the map. | Light and weather factors from `light.go` and `climate.go`. | DEF-1 | partial |
| Melee: adjacent attacks with cooldown, hit chance, weapon and body attacks. | Done for fists and animals; DEF-2 adds mace, gladius, longsword and pila beside club, knife and spear. | Melee weapons are not stuff-scaled and a wielded weapon loses its crafted quality on equip. | DEF-2 | partial |
| Targets are chosen by threat and cover is sought. | DEF-4 raiders: colonist or ally, a powered turret in sight (also with no colonist reachable, wiki: Raid), a rival faction's raider closer, then walls and doors (breach). | Threat weighting; cover seeking for ranged raiders. | DEF-4 | partial |
| Combat log entries and records. | `logStrike`, `logEvent` (`eventlog.go`); DEF-4 `combat/log.go`: a 64-entry ring of fights and threats (raid arrived, drop pods, colonist struck down, raider downed or killed, retreat, theft, fires, siege), saved, read model `WorldView.CombatLog`. | Combat records and threat history screens (UI 24.3). | DEF-4 | partial |

### 3.2 Armor and apparel

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Apparel has sharp, blunt and heat armor; a hit is deflected or reduced by the worn layers, with a quality and stuff effect. | `armor.Hit` resolves layers by region; `armor.AppendPiece` (`domain/combat/armor/apparel.go`) scales by stuff and quality (plate x0.9, simple helmet x0.5, advanced x0.7 of the stuff, steel fallback); `combat_host` reads the worn pieces; dressing handles the Legs slot. | Raiders wear no armor yet. | DEF-0, DEF-1, DEF-2, PRD-2 | done |
| Armor catalogue: flak, plate, marine, helmets. | Flak pants, vest and jacket, plate armor, marine armor, simple and advanced helmets and the rest of the 30 weapon and armor item kinds; art in `assets/source/blender/artlib/families/armor.py`, drawn by `render/creature/worn.go` (layer order and stuff tints). | Recipes (PRD-4); raider armor. | DEF-2 | partial |
| Armor stat shown in the gear tab and tooltips. | `PawnView.ApparelArmor []armor.Set` and `WeaponStats` drive `inspector/gear_facts.go` (`GearStatLines`); journey `TestJourneyGearTabWeaponStats`. | Hover tooltips (UI 12.3 tooltip rows stay partial). | DEF-1, DEF-2 | done |

### 3.3 Cover

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Cover on the shooter side of the target cuts the hit chance; objects have a cover value; shots can strike the cover. | `Sight` reads permille cover from `domain/combat/cover`: buildings (`cover.OfBuilding`), rock, grown trees, cacti, bushes and stone chunks (chunk tiles cached 250 ticks). Wall and door 75 (wiki, was 40), barricade 55, turrets 40/50. `CoverPct` is deleted; the overlay uses `cover.OfBuilding`. | None. | DEF-0 (domain), DEF-3 | done |
| Barricades; natural cover from trees, rocks and chunks. | Barricade building (55) and sandbags; natural cover from trees, rocks, cacti, bushes and chunks read from the map. | None. | DEF-3 | done |
| Cover overlay for the player. | Shift+O toggles `Combat.Cover` (`ebiten/hotkeys.go`, `combat_overlay_host.go`); `Combat.DrawCover` (`render/overlay/combat_draw.go`). | None. | DEF-1 | done |

### 3.4 Weapons

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Melee: knife, club, spear, mace, longsword. Ranged: bows, pistols, revolver, rifles, shotgun, machine gun, sniper. Thrown grenades. Single-use launchers. | 30 item kinds and weapon defs (`domain/combat/weapon`, item catalogue); `domain/combat/kit` picks raider kits by faction tech and points. | Rocket launchers skipped; incendiary and EMP launchers exist but no single-use rule; recipes (PRD-4). | DEF-2 | partial |
| Thrown and explosive weapons; launchers. | Frag grenades, molotov cocktails, EMP grenades and incendiary and EMP launchers as weapon kinds (the smoke blast kind exists in the explosion domain but no weapon uses it); grenades arc over cover and scatter at range; explosive weapons are blocked from hunting (`TestNoHuntingWithExplosives`). Raiders throw them via the kit. | Colonist grenade throw order (DEF-4); rocket launchers. | DEF-2, DEF-4 | partial |
| No ammunition for ordinary guns. | True today. | None. | - | done |

### 3.5 Explosions and fire

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Explosions with radius, falloff, fire, EMP and smoke variants; they damage buildings, items and pawns, and chain. | `domain/combat/explosion` and `combat/blast.go`: damage full at the centre to half at the rim (original tuning), line of sight from the centre, bomb x2 vs buildings, flame ignites through the host `Ignite`, EMP stuns turrets (`AimSave.StunnedUntil`), smoke cuts aim to 30 % (`Blasts.SmokeFactor`, saved as `Smoke`). | Items are not damaged; chain reactions not coded; bomb x4/x8 for plants and walls. | DEF-2 | partial |
| Explosion sound and visual. | Sound: `ListenBlasts` plays by Blast kind; `BurstView.Blast`/`Radius` (tenths) and a Landed trace feed the renderer. | Renderer: `Bus.ListenBursts` (volume by radius) and the burst flash; no per-kind blast art (fire, EMP, smoke clouds). | DEF-2 | partial |

### 3.6 Traps, turrets, mortars, doors

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Spike traps. | `construction.SpikeTrap`; `springTrap` springs at `trapChance` percent for `trapDamage` and re-lays the frame; `trapsUnderfoot` checks movers; sappers step around traps (`sapperBreaks`). | None for the spike trap. | - | done |
| IED traps (high explosive, incendiary, EMP, smoke). | `game/combat/ied.go`: four kinds on the explosion service, 15-tick fuse (wiki), blast, re-laid blueprint. High explosive radius 3.9 (wiki). DEF-4: a bullet landing on a charge lights it (wiki), a blast lights the charges it reaches (chains), sappers know 30 % of charges (wiki: Spike trap, fixed per charge). | None. | DEF-3, DEF-4 | done |
| Mini-turret needing rearm and barrel wear; autocannon; uranium slug turret. | Autocannon and uranium slug turret with wiki stats (min range, bursts). Barrels 60/90/30 rounds (wiki); a spent barrel stops firing; `RearmTurret` haul by idle undrafted colonists (steel per barrel 9/20/7 = wiki cost / 9, scaling); Hold fire (`SetTurretHold`). | Uranium and reinforced barrel items (steel stands in). | DEF-3 | partial |
| Mortars: manned, shells, forced target, minimum range. | Mortar building (180 HP), shell items, 20 shells per barrel; range 29-500, warmup 240, cooldown 1680, forced miss radius 9 scaled by crew Shooting (wiki). Crew order, `ManMortar`, `SetMortarTarget`/`ClearMortarTarget`. DEF-4: a siege's raiders build two mortars (`Mortars.Raise`, 15 HE + 5 incendiary shells, original split) and crew them (`Mortars.Man`), aimed at colonists; colonists never man, order or rearm them. | None. | DEF-3, DEF-4 | done |
| Doors: hold open, opening speed by stuff, autodoor, forbid. | `domain/building/door`; `Kind.IsDoor()` (door or autodoor) at the game and adapter sites. Opening speed by stuff (wiki), powered autodoor faster, 45-tick open and 120-tick auto-close (original), never closes on someone; `SetDoorHoldOpen` lets hostiles through; open doors let shots through; Barred is unwalkable to colonists. | Door forbid UI (Forbid exists only on sleeping places). | DEF-3 | partial |
| Building damage and repair. | `damageBuilding`, `assignRepair`, `doRepair`, `DamageAt` (`defense.go`). | None. | - | done |

### 3.7 Drafted behaviour

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Draft, move, attack target order, melee order. | Done (checklist 17.1, UI 13.1, 13.2). | None. | - | done |
| Fire-at-will toggle and hold position. | DEF-4 `combat/draft.go`: `SetFireAtWill` off: `AutoEngage` starts nothing (an auto attack under way is cancelled); `SetHoldPosition`: an auto attack starts in place and is dropped once out of reach (orders still walk). Saved in the defence block; `PawnView.FireAtWill`/`HoldPosition`. | None. | DEF-4 | done |
| Grenade throw order. | DEF-4: equipping a grenade stack takes it whole (`Drafts.Carry`); `ThrowGrenade` waits the warmup (90, wiki) and lobs through `Shots.Lob` and the blast path, scattered by the wiki forced-miss curve; every colonist grenade thrown spends one, the last disarms; `PawnView.Grenades`. | Walking into range (a throw too far is refused with a message). | DEF-4 | done |
| Targeting feedback: range overlay, line-of-sight line, hit-chance tooltip, invalid shot explanation. | `Game.HitChance` (`application/game/combat.go`) returns an `attack.Report`; `DrawWeaponRange`, `DrawShotFeedback`/`ShotLines` (hit chance, line of sight, why a shot is invalid) and `DrawProjectiles` in `render/overlay/combat_draw.go`, wired by `combat_overlay_host.go`. | None. | DEF-1 | done |

### 3.8 Raids, sieges, infestations, mechanoids

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Raid points from wealth and colonists, hostile faction chosen, group built from a budget. | `raidPoints`, `Wealth`, `raidKit`: kit now picked by `kit.Pick` from faction tech and points over the DEF-2 catalogue. | Raiders wear no armor; checklist 24.1 Wealth calculation open. | DEF-2 | partial |
| Raiders wear armor by tech level. | None. | Raider armor in the kit. | DEF-3 | open |
| Colonist grenade throwing and rocket launchers. | Grenade throw order (DEF-4, above). | Rocket launcher kind. | DEF-4 | partial |
| Edge arrival and immediate assault. | `raidArrive`/`raidAssault`/`raidFlee` modes (`raids.go`); `planAssault`. | None. | - | done |
| Siege: camp out of range, dig in, bombard with mortars, then storm. | DEF-4: camp 32 tiles out (past the mortar minimum), sandbags, two raider-crewed mortars, storm after 1.5-3 days or on 8 % of hits (wiki: Raid); crews leave their guns to storm; the give-up day starts at the storm. | Resupply drops (12 meals). | DEF-4 | done |
| Sappers and breachers. | `planSappers`: tunnel through walls, doors and rock from day 10 (`sapperBreaks`, `digRock`). | None. | - | done |
| Drop pods: centre, edge, scatter variants; staging and assault trigger. | DEF-4 `raid/pods.go`, `raid/stage.go`: map entry 60/30 of 90 (wiki 45+15 walk-in, 30 pods), pods split evenly (original), tribes walk; pods fall 240 ticks (original), break the roof (15 crush, original; wiki: pods deal no damage themselves), leave a pod shell; `ArrivalView.Hostile`. Staged raids gather 10 tiles in, storm on a colonist in sight, gathered after 3 h or a 6-14 h limit (original; wiki: "usually under a day"). | None. | DEF-4 | done |
| Prepare-before-attack, target priority, steal and burn. | DEF-4: staged raids; target priority (above); Steal raids take the nearest stack worth 50+ silver and run (dropped where they fall); Burn raids light up to 3 wooden buildings, then assault. Strategy bands on the old plan roll (original), `raiddef.Fit` (siege needs industrial and 300 points). | Opportunistic theft on retreat; burning crops. | DEF-4 | done |
| Retreat and kidnap. | Flee at half hit points, at the group's rolled 40-70 % lost (DEF-4, wiki: Raid), an idle hour or a day (`raids.go`); `kidnap.go`. | The 10-15 h assault timer (wiki). | - | done |
| Faction-on-faction hostility. | DEF-4: raid groups of two different hostile factions on the map fight each other (`raidMischief.Rival`, a once-a-tick rivalry pass). | Visitors and allies against raiders of other factions. | DEF-4 | done |
| Mechanoids (scyther, lancer, centipede, pikeman), EMP vulnerability, crashed ship parts. | DEF-4 R2 (`game/raid/mech.go`, `shippart.go`): four original mechanoids (Harrow, Spindle, Bulwark, Quill) with wiki role stats and their own brain, EMP stun then 2200-tick immunity, no wound body, wrecks shredded at the machining table, `Mechs.Raid` from 300 points, the 6x3 ship part (1200 HP, plant blight 5 to 100 tiles over 20 days, guards that wake once, salvage on destruction, incident weight 2, 400-point gate). Art: full sets for all four. | Closed doors block mechs; mech groups do not use the pod hook; all four voices per mech (blocked, below). | DEF-4 | partial |
| Infestations: hives, reproduction, dark and warm placement. | `hiveSwarm` (`hive.go`): three to six insects break out of the rock from day 12; hostile from spawn, never flee. No hive building. | Hive building, spawn rules, reproduction. | DEF-4 | open |

### 3.9 Already at fidelity (no work)

Draft and move orders, attack orders, melee combat, line of sight, building damage and repair, spike trap, the powered sentry gun with lock hysteresis, assault, siege and sapper raids, flee and kidnap, manhunter packs, damage numbers and threat markers. State: `done`.

### 3.10 Remaining after wave E

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Mechanoids pass doors. | Raiders bash closed doors; mechs do not. | Mechs open or bash doors. | later | open |
| Mech groups arrive by pods. | `Mechs.Raid` lands at an arrival spot with no pod entity. | Use the pod landing hook. | later | open |
| Mechanoid voices (`call`, `attack`, `hurt`, `die` each). | 16 prompts written; the insect voice group stands in; nothing generated. | Blocked on the user: the ElevenLabs key is missing and an open-source MOSS-SoundEffect model is under evaluation. Creature skill rule 5 is not met until then. | later | open [!] |
| Raiders steal from stockpiles on their way out. | Steal raids take one stack and run. | Opportunistic theft on retreat. | later | open |
| Raiders burn crops. | Burn raids light up to 3 wooden buildings. | Burning crops. | later | open |
| Assault timer. | Staged raids wait 3 h (original). | The wiki's 10 to 15 h assault timer. | later | open |
| Siege resupply. | Sieges are fixed at the start. | Resupply during a siege. | later | open |
| Raiders avoid walking into grenade range. | Raiders do not weigh a grenade's reach. | Pathing and target weighting for it. | later | open |
| Threat-weighted targeting and cover seeking (row 3.1). | Colonist or ally, turret, rival, then walls. | Weights and cover seeking. | later | open |
| Weapon quality kept on equip; `Wealth()` shared value functions. | See `PRODUCTION_GAP_ANALYSIS.md` 3.7. | Shared with PRD. | later | open |

## 4. Wiki constants

Filled in wave A from the wiki research. Each row becomes a named constant with `// wiki: <page>`. A row that the wiki does not publish records the decision.

| Constant | Value | Source | Used by |
|---|---|---|---|
| Weapon table: damage, warmup, cooldown, burst, range, accuracy at touch, short, medium, long, penetration, per weapon | Per weapon (dmg / AP % / warmup / cooldown ticks / range tiles / burst x gap / acc touch-short-medium-long %): short bow 11/16/81/99/22.9/1/75-65-45-25; recurve 14/21/87/99/25.9/1/70-78-65-35; greatbow 17/15/120/90/29.9/1/65-85-75-50; revolver 12/18/18/96/25.9/1/80-75-55-40; autopistol 10/15/18/60/25.9/1/80-70-40-30; machine pistol 6/9/30/54/19.9/3x7/90-65-35-15; bolt-action 18/27/102/90/36.9/1/65-80-90-80; assault rifle 11/16/60/102/30.9/3x10/60-70-65-55; sniper 25/38/210/90/44.9/1/50-70-88-90; pump shotgun 18/14/54/75/15.9/1/80-87-77-64; chain shotgun 18/14/72/81/12.9/3x10/57-64-55-45; heavy SMG 12/18/54/99/22.9/3x11/85-65-35-20; LMG 12/16/108/96/25.9/6x7/40-48-35-26; minigun 10/15/150/90/30.9/25x5/20-25-25-18; charge rifle 16/35/60/120/27.9/3x12/55-64-55-45; charge lance 30/45/102/162/32.9/1/65-85-85-75; pila 25/10/240/150/18.9/1/80-71-50-32. Stopping power 0.5 to 3 per weapon; stagger slows 83% for 95 ticks at stopping power >= 1. Launchers and grenades use forced-miss (no accuracy bands): incendiary and EMP launcher 210/210/23.9, frag grenade 50 Bomb 90/160/12.9, doomsday and triple rocket 270/270/36. Melee table and crafting costs: see the research file. Burst, warmup, cooldown, bands types in `internal/domain/combat/attack` and `internal/domain/combat/weapon` | https://rimworldwiki.com/wiki/Weapons and per-weapon pages | DEF-0, DEF-2 |
| Accuracy band distances (touch, short, medium, long) | Touch 3, short 12, medium 25, long 40 tiles, linear interpolation between; behaviour past 40 not published (domain clamps to long accuracy) (in `internal/domain/combat/attack`, `DefaultBands`, matches research) | https://rimworldwiki.com/wiki/Weapons (Accuracy) | DEF-0, DEF-1 |
| Hit chance terms: shooter skill, target size, light, weather | Hit chance = shooter accuracy per tile ^ distance x weapon band accuracy x weather x smoke x cover. Shooter accuracy per tile by skill: 0 89%, 5 94.5%, 10 97%, 15 98.167%, 20 99% (curve -20 70, -10 80, 30 99.65, 40 99.8, 60 99.9); careful / trigger-happy +5 / -5 skill; turret 96% (skill 8 equivalent). Target size: body size clamped 0.5..2.0. Weather: fog 50%, clear 100%, others not published. Smoke 30% (domain differs: reads it as a x0.3 factor because the wiki is ambiguous; wiki conflict). Light: no base penalty published (domain uses 1000, original tuning). Wall bullet block 75%; miss radius by accuracy 100%:1.0, 22%:2.0, 11%:4.0, 7%:6, 4%:8, 2%:10 tiles; friendly-fire safe distance 5 tiles (in `internal/domain/combat/attack`, matches research) | https://rimworldwiki.com/wiki/Weapons, /Shooting, /Shooting_accuracy, /Combat | DEF-1 |
| Armor resolution rule (deflect, reduce) and layer order | Effective armor A = rating - AP (subtracted). Roll 0..100: below A/2 deflects (damage 0), A/2 to A halves, above A no effect; rating cap 200%; layers outermost first, each checked once with full AP on the remaining damage; sharp halved becomes blunt after layers (in `internal/domain/combat/armor`, matches research; domain uses a half-open permille roll). Calculated AP = 1.5 x base damage fraction (flame 0). Quality multiplier on apparel protection 0.6/0.8/1/1.15/1.3/1.45/1.8 | https://rimworldwiki.com/wiki/Apparel, /Armor, /Damage | DEF-0, DEF-1 |
| Armor values per apparel and stuff; coverage by body part | Sharp / blunt / heat % (HP, coverage, move speed): flak vest 100/36/27 (200, torso+neck, -0.12); flak pants 55/8/10 (140, legs); flak jacket 55/8/10 (260, torso, neck, shoulders, arms); plate armor 90% of stuff (290, whole body except head, -0.8); marine armor 106/45/54 (340, -0.25); recon armor 92/40/46 (280, 0); cataphract 120/50/60 (400, -0.5); simple helmet 50% of stuff (100); flak helmet 70% of stuff (120); marine helmet 106/45/54 (160); recon helmet 92/40/46 (120); cataphract helmet 120/50/60 (180). Stuff armor table and per-part coverage percentages: not published here; cowboy hat and advanced helmet: no rating, decision pending (DEF-2) | https://rimworldwiki.com/wiki/Armor, /Apparel | DEF-2 |
| Cover values per object; cover rules for trees, rocks, chunks | Wall 75%, door 75%, sandbags 55%, barricade 55% (57 before 1.3), stone or slag chunk 50%, rubble 50%, saguaro cactus 35%, tree 25%, fence 25%, bush 20%, plant pot or stool 20%, table or bed 40%, dining chair 35%; turret cover 40% (mini) and 50% (autocannon, uranium). Angle multipliers <15 deg 100%, 15-27 80%, 27-40 60%, 40-52 40%, 52-65 20%, >65 0%; point blank 33.3% adjacent, 66.7% at 1 tile. Coded kinds (in `internal/domain/combat/cover`): sandbags 55, barricade 55, wall 75, tree 25, chunk 50 match; natural rock 75 is a domain decision (the wiki gives no rock-wall value, so it is treated as a wall). Pawn and animal cover: not published. Embrasure: no wiki page | https://rimworldwiki.com/wiki/Cover, /Sandbags, /Barricade | DEF-0, DEF-3 |
| Damage types and their wound mapping | Armor category: sharp = cut, stab, scratch, bite, bullet, arrow, bomb; blunt = crush, blunt, poke, demolish; heat = flame, burn; EMP has no armor (in `internal/domain/combat/damage`: Sharp, Blunt, Heat, Bomb, Burn, EMP; Bomb resisted as sharp, matches research). EMP stun 0.25 s per damage point, mechanoids and turrets only; mechanoid EMP adapt immunity 2200 ticks; stun damage 0.33 s per point. Wound mapping per body part: not published here; decision pending (DEF-0) | https://rimworldwiki.com/wiki/Damage, /Mechanoid | DEF-0 |
| Explosion radius, damage, falloff, fire chance, EMP and smoke durations per source | Frag grenade 50 Bomb r 1.9 (fuse 100 ticks; cook-off at <= 20 HP r 2.66; bomb damage x2 vs buildings, x4 plants and impassable, x8 walls); molotov 10 Flame r 1.1; EMP grenade 50 EMP r 3.5; HE shell 50 Bomb r 2.9; incendiary shell 10 Flame r 2.9; EMP shell 50 EMP r 8.9 (stuns mechs, turrets, generators 25 s); smoke shell r 7.2, about 60 s; doomsday rocket 50 Bomb r 7.8 plus 3 secondary 10 Flame; triple rocket 3 x 50 Bomb r 3.9; turret destruction 50 Bomb r 3.9 (mini) / 5.9 (autocannon, uranium), 50% chance below 20% HP, 240 tick fuse; shell cook-off at damage over 70% HP, radius sqrt(stack) x 0.4 up to 6. Falloff and fire chance: not published; decision pending (DEF-2) | https://rimworldwiki.com/wiki/Frag_grenades, /Molotov_cocktail, /EMP_grenade, /IED_trap, /High-explosive_shell, /Incendiary_shell, /EMP_shell, /Smoke_shell, /Doomsday_rocket_launcher, /Triple_rocket_launcher | DEF-2 |
| Grenade and launcher stats | Frag grenade 90 warmup, 160 cooldown, range 12.9, blast 1.9, miss radius 1.9, speed 12 cells/s, crafted 20 steel + 80 chemfuel, 12000 ticks; molotov 25 cloth + 80 chemfuel, 6000; EMP grenade 20 steel + 6 comp, 24000; incendiary launcher 210/210, range 23.9, blast 1.1, miss 1.9, 75 steel + 4 comp, 20000; EMP launcher blast 1.1, 75 steel + 8 comp, 30000; smoke launcher blast 2.4; doomsday and triple rocket launchers 270/270, range 36, miss 1.9 / 2.9, not craftable. Projectile speed: launchers 40, grenades 12, rockets 50 cells/s. Forced-miss radius multiplier by distance 0-8 tiles 0, 9-24 0.5, 25-48 0.8, 49+ 1.0 (wiki: needs verification) | https://rimworldwiki.com/wiki/Weapons and per-item pages | DEF-2 |
| IED stats per kind; spike trap trigger chance and damage | High-explosive IED: 50 Bomb r 3.9, 2 HE shells, 1400 work, Construction 3, research IEDs, delay 15 ticks, 100% enemy trigger, bullets and arrows also trigger. Incendiary, EMP and smoke IEDs: not published (no wiki pages); decision pending (DEF-3). Spike trap: 5 stab hits, each 100 x stuff sharp multiplier x rand(0.8..1.2) / 5, AP = damage x 0.015 (steel 20 per hit, wood 8, plasteel 22, gold 15, silver 17); cost 45 stuff, 3200 ticks, Construction 3; trigger chance enemies 100%, wild animals 20%, wild men 30%, colonists and tame 0.5%, nimble x0.1; smart or sapper raiders know about 30% of traps (path cost 40) | https://rimworldwiki.com/wiki/IED_trap, /Spike_trap | DEF-3 |
| Turret stats: mini-turret, autocannon, uranium slug; ammo, barrel life, rearm | Mini-turret: range 28.9, damage 12, AP 18%, burst 2 x 8 ticks, cooldown 288, no warmup, accuracy 77/70/45/24, HP 100, 80 W, 70 steel + 3 comp + 30 stuff, 1800 work, barrel needs 80 steel per 60 rounds. Autocannon: range 32.9 (min 8.9), 27 dmg, AP 40%, burst 3 x 17, cooldown 210, accuracy 28/72/66/50, HP 380, 150 W, 350 steel + 40 plasteel + 6 comp, 15000 work, 180 steel per 90 rounds. Uranium slug: range 45.9 (min 11.9), 55 dmg, AP 82%, cooldown 240, single shot, accuracy 22/33/44/95 (odd shape, as published), HP 380, 150 W, 300 steel + 30 plasteel + 60 uranium + 6 comp, 60 uranium per 30 slugs. EMP disables a turret for time scaling with EMP damage, no adaptation | https://rimworldwiki.com/wiki/Mini-turret, /Autocannon_turret, /Uranium_slug_turret | DEF-3 |
| Mortar stats: range, minimum range, shell damage, warmup, cooldown, aim error | Range 500 tiles (published), minimum range 29.9, warmup 240, cooldown 1680 (only while manned), projectile speed 41 cells/s, forced miss radius 9 tiles (13 with Classic Mortars) x crewman multiplier (base 0.2, -0.025 per shooting level), barrel life 20 shots per reinforced barrel, cost 50 stuff + 1 reinforced barrel + 6 comp, 2000 work, Construction 5, research Mortars. Shell damage: HE 50 Bomb r 2.9, incendiary 10 Flame r 2.9, EMP 50 EMP r 8.9, smoke r 7.2 | https://rimworldwiki.com/wiki/Mortar, /High-explosive_shell | DEF-3 |
| Door opening speed by stuff; autodoor | Door: 25 stuff, 160 HP base, 850 work; open speed wood 120%, metals 100%, uranium 75%, stone blocks 45%. Autodoor: 25 stuff + 40 steel + 2 comp, 1100 work, Construction 6, 50 W (plain door if unpowered); open speed wood 480%, steel/silver/gold/plasteel/jade/obsidian/bioferrite 400%, uranium 300%, stone blocks 180% | https://rimworldwiki.com/wiki/Door, /Autodoor | DEF-3 |
| Raid points curve, raid type weights, drop-pod rules, siege timings | Points = (wealth points + pawn points) x difficulty x start factor x adapt factor, min 35, max 10000. Wealth points: 0 up to 14000 wealth, linear to 2400 at 400k (about 0.00622 per wealth), 3600 at 700k, 4200 at 1M+; storyteller wealth counts items + creatures + buildings x 0.5. Pawn points per colonist: 15 up to 10k wealth, 10k-400k 15 + (wealth - 10000)/3120, 140 at 400k, 200 at 1M; weights slave 0.75, cryosleep 0.3, baby 0, child 0 at 3 up to 1 at 18; tamed attack animal 8% of combat power. Threat scale: peaceful 0.1, community 0.3, adventure 0.6, strive 1.0, blood and dust 1.55, losing 2.2. Start factor 0.7 at <= 10 days, 1.0 at >= 40 (between not published). Adapt factor -30:0.4, 0:0.8, 30:1.0, 60:1.2, 120:1.6. Arrival modes (garbled table, verify): walk-in edge 45%, edge group 15%, drop pods 30%, haywired pods 10%. Drop pods: target not under mountain, 40% chance to hit an unroofed powered trade beacon, tribal cannot use pods. Siege: 2 mortars built, resupply 12 meals, assault after random 1.5 to 3 days, 8% per damage taken. Strategy weights and thresholds: not published; decision pending (DEF-4) | https://rimworldwiki.com/wiki/Raid_points, /Raid | DEF-4 |
| Mechanoid stats (health, armor, speed, weapons) and EMP stun duration | Scyther health 1.32, speed 4.7, armor 40/20/200, power 150, blades 20 dmg 30% AP cd 2 s; lancer 0.72, 4.7, 190, charge lance 30 dmg 45% AP; pikeman 0.85, 2.5, 110 (needle gun range not published); centipede 4.32, size 3, 1.9, armor 72/22/200, power 400 (variant weapon numbers not published); termite 2.16, size 1.6, 2.1, armor 56/22/200, power 110. Health/armor are listed in the Biotech section; core values assumed identical. EMP stun 0.25 s per damage point, adapt immunity 2200 ticks. Combat power: tribal villager 30, drifter 35, pirate gunner 65, mercenary gunner 85 / sniper 110 / slasher 140 / grenadier 70 | https://rimworldwiki.com/wiki/Scyther, /Lancer, /Pikeman, /Centipede, /Mechanoid, /Raid | DEF-4 |
| Hive spawn, reproduction rate, infestation point cost | Infestation at most once per 20 days; location under mountain roof, above -17 C, within 30 tiles of a colony structure; no new random infestation at >= 30 hives; hive 130 HP; new hive on a roofed tile within 10 tiles, none within 8 adjacent cells; hive jelly 20 per 26000-30000 ticks, 30 on destroy; insects aggro radius 10 tiles (whole infestation turns aggressive on damage). Hive reproduction interval, insect counts per raid-point level and infestation point cost: not published; decision pending (DEF-4) | https://rimworldwiki.com/wiki/Infestation, /Hive | DEF-4 |

## 5. Phase plan

**DEF-0, extraction and domain.** No behaviour change. `combat.go`, `combat_state.go`, `defense.go` and `turret.go` move into `internal/application/game/combat` behind consumer-owned roles; raids stay. The domain packages `damage`, `armor`, `cover` and `attack` are filled with pure code and tests (damage types, armor resolution, cover rules, accuracy bands, burst), and `weapon.Def` gains warmup, cooldown, burst, penetration and bands with the current values. Golden hashes and old saves stay byte-identical (`bin/benchmark -hash`); raid p99 within noise. Moves: no checklist item; it prepares 17.3, 17.4 and 17.6.

**DEF-1, projectiles, armor, hit chance.** Projectiles in flight on an index with stray hits and friendly fire; per-weapon warmup, cooldown, burst and accuracy bands; armor resolution wired into `hurtAs`; target size, light and weather in the hit chance. UI: range overlay, line-of-sight line, hit-chance tooltip, cover overlay, invalid shot explanation; firing and impact sounds. Behaviour changes, so hashes and `TestGrownGolden` are re-pinned with a written reason. Moves: vanilla 17.4, 17.6; UI 13.3.

**DEF-2, catalogue and explosions.** The weapon and armor lists with art and sounds, crafted by PRD-4 recipes. A general explosion service (damage, fire, EMP, smoke), grenades and single-use launchers. Raid kits follow the catalogue. Moves: vanilla 16.3, 17.7, 24.2 (equipment); UI 12.3.

**DEF-3, structures.** Barricades and natural cover; IEDs; mini-turret rearm and barrel, autocannon, uranium slug turret; mortars with manning, shells, forced target and minimum range; doors with hold-open, speed by stuff, autodoor and forbid. Art and sounds for each. Moves: vanilla 17.3, 7.3 Doors.

**DEF-4, drafted control and raids.** Fire-at-will and hold position, grenade throwing, then the `game/raid` extraction with a hash proof, drop pods (centre, edge, scatter), staging and assault trigger, target priority, steal and burn, siege bombardment and assault transition, faction-on-faction hostility, hives with reproduction, mechanoids with EMP vulnerability and crashed ship parts (four sounds each), combat records and threat history. Moves: vanilla 17.1, 17.8, 24.2..24.6; UI 13.2, 24.3 (combat records).

## 6. Checklist cross-reference

Each phase ticks these items when its acceptance criteria are met. Titles are quoted exactly. The lead edits the checklists; this file does not.

### 6.1 `RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md`

| Section | Item | Phase |
|---|---|---|
| 7.3 Core structures | **Doors** (`[-]`) | DEF-3 |
| 16.3 Weapon categories | **Ranged weapons** (`[-]`), **Thrown/explosive weapons**, **Single-use launchers** | DEF-1, DEF-2 |
| 17.1 Drafting | **Hold/fire-at-will behavior** (`[ ]`) | DEF-4 |
| 17.3 Cover | **Cover calculation** (`[-]`) | DEF-1, DEF-3 |
| 17.4 Ranged attack | **Target acquisition** (`[ ]`), **Warmup** (`[-]`), **Projectile flight** (`[-]`), **Burst fire** (`[ ]`) | DEF-1 |
| 17.6 Armor | **Armor coverage**, **Armor categories**, **Armor resolution** (`[ ]`) | DEF-1 |
| 17.7 Explosions | **Blast cells**, **Radial damage**, **Fire chance**, **Building/item/pawn damage**, **Chain reaction support for explosive content** (`[ ]`) | DEF-2 |
| 17.8 Combat AI | **Hostile target selection**, **Basic cover seeking** (`[ ]`) | DEF-4 |
| 24.1 Colony wealth | **Wealth calculation** (`[ ]`) | DEF-4 |
| 24.2 Raid generation | **Generate equipment based on faction/tech** (`[-]`), **Prepare-before-attack behavior** (`[ ]`) | DEF-2, DEF-4 |
| 24.3 Raid tactics | **Drop-pod assault**, **Center/scatter drop variants** (`[ ]`) | DEF-4 |
| 24.4 Raider morale | **Steal behavior** (`[ ]`) | DEF-4 |
| 24.5 Mechanoid threats | **Mechanoid pawn types**, **Mechanoid raid**, **Crashed ship-part threat** (`[ ]`) | DEF-4 |
| 24.6 Insects | **Hive**, **Hive reproduction** (`[ ]`), **Infestation incident** (`[-]`) | DEF-4 |

### 6.2 `RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md`

| Section | Item | Phase |
|---|---|---|
| 12.3 Combat-stat tooltips | **Weapon accuracy/range/warmup/cooldown**, **Armor by damage category** (`[ ]`) | DEF-1, DEF-2 |
| 13.2 Tactical orders | **Fire-at-will toggle**, **Hold position behavior if implemented** (`[ ]`) | DEF-4 |
| 13.3 Targeting feedback | **Weapon range overlay**, **Line-of-sight line**, **Cover visualization**, **Hit-chance tooltip**, **Invalid shot explanation** (`[ ]`) | DEF-1 |
| 13.4 Combat feedback | **Projectile/impact feedback** (`[x]`; note projectile flight) | DEF-1 |
| 24.3 Pawn records | **Combat records**, **Kills**, **Damage dealt/taken** (`[ ]`) | DEF-4 |

## 7. Out of scope

| Item | Missing prerequisite | State |
|---|---|---|
| Breach raids and breach weapons | DLC (Ideology and Royalty era content) | out of scope (DLC) |
| Mech clusters, mechanitors, mech bands and mech gestation | DLC (Biotech) | out of scope (DLC) |
| Psycasts, shield belts of royal origin, bladelink weapons, orbital strikes | DLC (Royalty) | out of scope (DLC) |
| Ideology violence precepts, ritual combat | DLC (Ideology) | out of scope (DLC) |
| Anomaly entities and horror combat | DLC (Anomaly) | out of scope (DLC) |
| Cryptosleep ship interiors as raid sites | Needs a mapgen site type | out of scope (separate work) |
