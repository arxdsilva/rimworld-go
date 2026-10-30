# Combat gap analysis (CBT-0..CBT-5)

Scope: the fighting itself, vanilla core only (no DLC): drafting and orders, ranged and melee attacks, the damage model, armor, weapons and equipment, turrets, traps and structures, explosions, hostile and colonist AI, combat experience, prisoners, combat UI, and sounds and effects. Load only for CBT-* tasks. Phase handoffs go in `.cursor/handoffs/CBT-N-*.md`. This document starts where `DEFENCE_GAP_ANALYSIS.md` (DEF-0..DEF-4, done) stopped: it lists what is still wrong or missing after those waves. Sister documents: `DEFENCE_GAP_ANALYSIS.md`, `PRODUCTION_GAP_ANALYSIS.md`, `MEDICAL_GAP_ANALYSIS.md`.

## Status (2026-10-30)

CBT-0 done (this document, the checklist corrections, the index entry). CBT-1..CBT-5 open. Every row in section 3 is `missing` unless it says `done` or `partial`. Rows are updated by the lead in the commit that ships the phase.

| Phase | Delivers | State |
|---|---|---|
| CBT-0 | This analysis; checklist ticks 17.5, 16.1 and 11.3 corrected | done |
| CBT-1 | Damage model (critical): wound kind from the shot's own weapon, body-part-first layered armor, vital part destruction by overkill, apparel hit damage, stopping power and stagger, stun type | open |
| CBT-2 | Attack mechanics: melee hit chance curve and dodge, melee tools, melee with a gun, Euclidean ranges, weather and cover-angle factors, downed targets, auto-undraft, auto-engage, grenade walk-in, drop-weapon command and spare slot, combat XP | open |
| CBT-3 | Weapons, equipment and structures: weapon stacks keep quality and stuff, missing weapons and belts, raider armor kits, wiki mini-turret and turret projectiles, wiki spike trap, rearm items | open |
| CBT-4 | Combat AI: cover seeking, threat-weighted targeting, undrafted threat response, tantrum, prison break, mechanoid doors, pods and retreat, raid leftovers, hives, hunting | open |
| CBT-5 | Explosions (items, plant and wall multipliers, cook-off, flame radius), per-kind blast art, combat UI | open |

## 1. Decision record

| Item | Decision |
|---|---|
| Goal | Close the fidelity gaps that survived DEF-0..DEF-4, ranked by how wrongly the colony behaves in a fight. The worst is the damage model: most weapons never make a bleeding wound. |
| Numbers | ENG-003 in `AGENTS.md` applies (extended 2026-10-29 to beds, production and defence): published RimWorld wiki numbers are used as tuning data; names and prose stay original. Each constant gets `// wiki: <page>`, or `// original tuning` where the wiki gives none. |
| Wiki stubs | Several wiki pages are thin or stubs for this scope: Drafting (a short page; the 10,000-tick auto-undraft is its one hard number), Raid (strategies and points, few combat numbers), Shield belt and Smokepop belt (item pages, no behaviour formulas), Prison break (no timings). Where a number is not published the row says so and the phase picks an original tuning, recorded in its handoff. |
| Clean room | No wiki text, code, art or sound is copied. New weapons and gear get original names and original art through `rimworld-go-art-assets`; sounds through `cmd/sfxgen` with user-approved spending. |
| Budget | `internal/application/game` is at its SOLID ceiling. Every phase puts new behaviour in `game/combat`, `game/raid` or domain packages behind consumer-owned roles, and pays any added game line by moving code out in the same change. Never raise a `solid_budget.txt` line. |
| Determinism and hysteresis | Rolls come from the existing streams (`rng_streams.go`), no global RNG, no per-tick global scans. Every threshold behaviour (cover seeking, retreat, flee or fight, tantrum, prison break, auto-undraft) starts and stops at different thresholds or has a cooldown, with a no-flap test. |
| Saves | New saved state keeps gob names and gets Export/Import plus a round-trip test. Golden hashes are re-pinned only in phases that change behaviour on purpose, and the handoff says so. |
| Player visibility | Each phase ships read-model fields and the HUD/UI representation in the same change, or states in the handoff why none fits. |

## 2. Reading the tables

State legend: `done`, `partial`, `missing`. `Today` evidence is `file:function` with a line number where the 2026-10-30 inventory gave one; line numbers drift, so find the function. Paths are under `internal/` and `game/` is `application/game/`; `combat/` is `application/game/combat/`. Wiki page names in the Vanilla column are rimworldwiki.com pages. The evidence was spot-checked by grep on 2026-10-30; section 5 lists the lines found stale or wrong.

## 3. Gap tables

### 3.1 Drafting and orders

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| A drafted colonist with no threats and no orders is released after 10,000 ticks (wiki: Drafting). | Drafts stay on until the player undrafts (`combat/draft.go`). | Auto-undraft with a timer reset by threats and orders. | CBT-2 | missing |
| Fire at will, hold position, move and attack orders. | DEF-4: `combat/draft.go` fire at will and hold position, `combat/orders.go`. | None. | DEF-4 | done |
| A grenade order walks the colonist into range. | `combat/draft.go` (about line 187) refuses the order out of range (`weapon.InRange`). | Walk in, then throw. | CBT-2 | missing |
| Drop a held weapon on command; carry a spare in inventory. | No drop-weapon command (grep finds none); `domain/pawn/inventory` is only a `doc.go`. Equip replaces the old weapon (`game/combat.go` about lines 30-35, 181: `g.equip[id] = st.Kind`). | Drop command, a single spare-weapon slot. | CBT-2 | missing |
| Undrafted colonists flee or fight by threat (wiki: Combat, threat response). | Auto-engage fires only at hostiles attacking a colonist (`game/combat.go` about line 158). | Per-hostile threat response, flee or attack. | CBT-2 (engage), CBT-4 (flee) | missing |

### 3.2 Ranged attacks

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Hit chance = per-tile skill accuracy ^ distance x weapon band x factors; per-tile shooting 0 -> 89 %, 10 -> 97 %, 20 -> 99 %; Careful shooter +5, Trigger-happy -5; weapon bands touch 3, short 12, medium 25, long 40 tiles (wiki: Shooting accuracy). | `attack.ShooterFactor`, weapon bands in `domain/combat/weapon` (DEF-1/DEF-2). | None known. | DEF-1 | done |
| Weather and light scale accuracy; fog and rain penalties (wiki: Shooting accuracy). | `combat/strike.go` `report` fixes `Light: NoLightPenalty`, `Weather: ClearFactor` (about lines 208-213); `attack.FogFactor` (about line 120) is unused. | Read light from `light.go` and weather from `climate.go`. | CBT-2 | partial |
| Ranges are Euclidean distances. | `weapon.InRange`/`Distance` use Chebyshev (`domain/combat/weapon/weapon.go` about lines 291-310). | Euclidean range and minimum range. | CBT-2 | missing |
| Miss radius scales with accuracy: 100 % -> 1.0, 22 % -> 2.0, 11 % -> 4.0, 7 % -> 6.0, 4 % -> 8.0, 2 % -> 10.0; a stray lands 50 % on nothing, 50 % on whatever is in the landing cell (wiki: Combat). | `combat/shot.go` (`fire`, `fly`, `through`, `land`), `attack.StrayHit`. Original tuning: an aimed shot hits only within `Tracking` (1 tile) of the aim point. | Miss radius table not wired; tracking constant is original. | CBT-2 | partial |
| Interception 40 % x body size, clamped 4 % to 80 %, none within 5 tiles, full at 12+; allies safe within 5 tiles of the shooter (wiki: Combat). | `attack.InterceptChance`, `attack.FriendlySafe` (DEF-1). | None known. | DEF-1 | done |
| Targets are chosen by threat weight (wiki: Combat). | Colonist targeting nearest-first; raiders see 3.9. | Threat-weighted choice. | CBT-4 | missing |
| Weapon has warmup, cooldown, burst and an accuracy curve. | `weapon.Def` (DEF-0/DEF-2); `Strikes.Do`/`next`. | None. | DEF-2 | done |

### 3.3 Melee

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Hit score = melee skill (+4 with no skill) + 12 x (min(sight, 150 %) - 100 %) + the same for manipulation + 4 for Brawler; post-process: -20 -> 5 %, 0 -> 50 %, 20 -> 90 % (wiki: Melee hit chance). | `hitChanceFor` (`game/skills.go` about lines 528-541): weapon base chance x clamp(100 + 4 x (level - 7), 50, 160) %. | Replace with the wiki curve. | CBT-2 | missing |
| The defender can dodge by melee skill (wiki: Melee hit chance). | None. | Dodge roll. | CBT-2 | missing |
| Melee damage varies x0.8..1.2 (wiki: Combat). | Fixed per weapon. | Random factor from the combat stream. | CBT-2 | missing |
| Weapons have tools (blade, point, handle) with their own damage and cooldown; a gun can be used as a club when adjacent; humans have several unarmed attacks, animals more than one. | One attack per weapon; humans fists only; animals one attack (`combat/strike.go` about lines 75-82, 103). | Tool list per weapon and species. | CBT-2 | missing |
| Adjacent pawns fight in melee even when holding a ranged weapon. | The code shoots point-blank (checklist 17.5 was ticked wrongly). | Melee with the stock when adjacent. | CBT-2 | missing |
| Downed humans can be attacked; downed enemies may die at once (wiki: Injury). | `combat/strike.go` (about line 58) skips downed humans. | Attack downed pawns by rule. | CBT-2 | missing |
| Melee weapons scale with stuff and quality. | DEF-2 mace, gladius, longsword, pila, club, knife, spear exist; not stuff-scaled; quality lost on equip. | Stuff scaling; keep quality. | CBT-3 | partial |

### 3.4 Damage model

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| The wound kind comes from the weapon's own damage type: cut, stab, bullet, blunt (crush), burn, bite, scratch (wiki: Damage types). | `woundKindBy` (`game/body.go` about lines 143-160) maps only knife, spear, rifle, claw and gore to Cut, bite to Bite, peck to Scratch; every other weapon bruises, and bruises never bleed (`domain/pawn/body/bleed.go` about lines 54-59). It reads the attacker's current weapon when the shot lands (`combat/strike.go` about lines 156-159); spike traps bruise. | Carry kind and type on the projectile and strike, never the current weapon. Checklist 11.3 corrected. | CBT-1 | missing |
| A hit picks the body part first, then armor from every garment covering that part, outermost first (wiki: Armor). | The armor region is rolled independently (`domain/combat/armor/apparel.go` about lines 25-36: head 10 %, legs 20 %, torso 70 %) from the wounded part (`game/body.go` about lines 110-139). | Part by coverage, armor by that part's group. | CBT-1 | missing |
| Overkill destroys parts by damage-type windows: Cut 0-10 %, Crush/Blunt/Stab 40-100 %, Bullet/Arrow/Scratch 0-70 %, Bite 0-10 %; chance = (overkill - lower) / (upper - lower), else the part drops to 1 HP (wiki: Injury). | A vital part is never destroyed by a hit (`game/body.go` about lines 104-109, 126; `domain/pawn/body/anatomy.go` about lines 97-128): the excess spills to the next pick. | Vital part destruction; brain or heart loss kills. | CBT-1 | missing |
| Multi-part cuts hit 1/2/3/4 parts with 30/45/20/5 %; blunt adds a 40 % internal strike (wiki: Injury). | Single part pick. | Multi-part rule. | CBT-1 | missing |
| Stopping power: if body size <= stopping power, the target moves at 1/6 speed for 95 ticks (wiki: Combat). | None. | Stagger on hit. | CBT-1 | missing |
| Stun type: blunt hits to the torso at >= 90 % of max HP stun 15 %, to the brain at >= 50 % stun 100 % (wiki: Damage types). | None. | Stun damage type and chances. | CBT-1 | missing |
| Pain = damage x 1.25 per HP for most types; pain shock at 80 % (wiki: Injury). | MED-* pain model. | Check constants against the wiki when CBT-1 touches wounds. | CBT-1 | partial |
| Bleeding: heart x5, neck x4; infection bite/burn 25 %, others 10 %; scar 2 % x clamp((HP lost - 4) / 10, 0, 1); healing 8 HP/day (+4 ground, +8 bed, +14 hospital, tend +4..+12) (wiki: Injury). | MED-* (`MEDICAL_GAP_ANALYSIS.md`). | Out of this document. | MED | done |

### 3.5 Armor

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Layers innermost to outermost: Skin, Middle, Outer, Belt, Headgear, Eyes; applied outermost first; conflicts only on the same layer with overlapping coverage groups (Full head, Upper head, Eyes, Shoulders, Arms, Torso, Waist, Neck, Legs) (wiki: Armor). | Three slots, one garment each (`game/combat_host.go` about lines 66-73: body, legs, head). No layers. | Layer and coverage-group model; several garments per layer when coverage does not overlap. | CBT-1 | missing |
| Flak vest 100/36/27 (sharp/blunt/heat %) torso and neck; flak pants 55/8/10 legs; marine armor 106/45/54 full; recon 92/40/46; plate x90 % of stuff; flak helmet x70 %; simple helmet x50 % (wiki: Armor and item pages). | DEF-2 catalogue and `armor.AppendPiece` (`domain/combat/armor/apparel.go`); plate x0.9, simple helmet x0.5, advanced x0.7. | Re-check the catalogue against the numbers when layers land. | CBT-1 | partial |
| Apparel loses hit points from absorbed hits. | Only daily wear (`craft/condition.go` about lines 113-140). | Hit damage on worn pieces. | CBT-1 | missing |
| Raiders wear armor by tech level and points. | Raiders get a weapon only (`game/raids.go` about lines 183-186). | Armor kits. | CBT-3 | missing |
| Shield belt blocks ranged shots and breaks on EMP; smokepop belt pops smoke when hit (wiki: item pages, stubs). | Neither exists. | Two belts with original tuning where the wiki gives none. | CBT-3 | missing |

### 3.6 Weapons and equipment

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| An equipped weapon keeps its quality and stuff. | `g.equip` stores only `item.Kind` (`game/combat.go` about lines 30-35, 181); crafted quality is lost on equip. | Equipped weapon is a stack (quality, stuff, HP). | CBT-3 | missing |
| Vanilla core ranged weapons: pistol, revolver, rifles, shotgun, LMG, sniper, bows, grenades, mortar shells. | DEF-2: 30 weapon and armor item kinds. | Rocket launchers (single-use), smoke launcher, charge rifle, charge lance, ikwa. | CBT-3 | partial |
| Hunting works with any ranged weapon and stalks (wiki: Hunting). | `combat/hunt.go` equips an `item.Rifle` (about lines 32-83); no stalking. | Any ranged weapon; stalk. | CBT-4 | partial |
| Single-use launchers are discarded when spent. | No launchers. | Part of the new gear. | CBT-3 | missing |

### 3.7 Turrets, traps and structures

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Mini-turret: projectile shots with strays and friendly fire, armor applied. | Sentry gun (`domain/combat/weapon/weapon.go` about line 143) is original tuning and fires instantly; `combat/turret.go` applies armor to some hits (`armorRoll`, about line 446), but turrets have no projectile path. | Wiki mini-turret numbers; shots as `combat.Shots` projectiles. | CBT-3 | partial |
| Turret destruction blast with wiki numbers. | Original (`combat/turret.go` about lines 49-52, 478). | Wiki numbers. | CBT-3 | partial |
| Spike trap: 5 stab hits scaled by stuff; enemies 100 %, wild animals 20 % (wiki: Spike trap). | Original: `TrapDamage` 32 and `trapChance` 75 (`combat/defense.go` about lines 20-32, 61-86); the hit bruises. | Wiki trap. | CBT-3 | partial |
| Cover values: wall and door 75, sandbags and barricade 55, chunk 50, cactus 35, tree 25, bush 20, hopper 50, shelf 40, table and bed 40, stool 20, chair 35, mini-turret 40 (wiki: Cover). | DEF-3 (`domain/combat/cover`). | None known. | DEF-3 | done |
| Cover works by angle: < 15 deg 100 %, 15-27 80 %, 27-40 60 %, 40-52 40 %, 52-65 20 %, > 65 0 %; point-blank 33 %, one tile 67 % (wiki: Cover). | `domain/combat/cover/cover.go` (about line 71) and `game/combat.go` (about lines 164-183) have no angle multipliers or partial cover at point-blank. | Angle table and short-range partial cover. | CBT-2 | missing |
| Uranium and reinforced barrels used by rearm. | Stand-in items. | Real items in `combat/rearm.go`. | CBT-3 | partial |
| Doors: forbid, mechanoids pass them. | Door forbid has no UI (3.13); mechs cannot pass or bash doors (`game/raid/mech.go` about lines 127-237). | Door handling. | CBT-4 (mechs), CBT-5 (UI) | partial |

### 3.8 Explosions

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Bomb damage x4 to plants and impassable buildings, x2 to passable, x8 to walls (wiki: Damage types). | `combat/blast.go` `Explode`/`buildings`; no multipliers. | Multipliers. | CBT-5 | missing |
| Explosions damage items on the ground. | Creatures and buildings only. | Item damage. | CBT-5 | missing |
| Stored shells and grenades cook off when their stack burns or explodes. | None. | Cook-off chain. | CBT-5 | missing |
| Flame ignites by flammability across the radius. | `Explode` ignites the centre only (`combat/blast.go` about line 50, `h.Ignite(x, y)`). | Whole-radius ignition. | CBT-5 | missing |
| EMP: 30 ticks of stun per point, 50 points -> 1500 ticks, then adapted (2200 ticks of immunity) (wiki: Damage types). | DEF-2 EMP explosions. | Verify the adaptation window. | CBT-5 | partial |
| Smoke clouds block line of sight. | `Blasts.smoke` clouds saved (DEF-2). | Art per kind (3.13). | DEF-2 | done |

### 3.9 Hostile AI

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Raiders choose targets by threat weight and seek cover (wiki: Raid, Combat). | `game/raid/raiders.go` (about lines 135-160): colonist or ally, a powered turret in sight, a rival faction's raider, then walls and doors. | Threat weighting; cover seeking. | CBT-4 | partial |
| Raiders steal on retreat, burn crops, assault on a 10-15 hour timer after a siege, siege camps resupply, grenade avoidance (wiki: Raid). | Retreat and siege exist (DEF-4); none of these leftovers. | The five raid behaviours. | CBT-4 | partial |
| Mechanoids arrive by pod, open, use and bash doors, and retreat by cluster rules. | `game/raid/mech.go` (about lines 127-237): no door handling, no pods, never retreat. | Door verbs, pods, retreat. | CBT-4 | partial |
| Hives spread buildings, reproduce and prefer dark, warm places (wiki: Hive). | `game/raid/hive.go` (about line 39): no hive buildings or reproduction raid. | Hive buildings and reproduction. | CBT-4 | partial |
| Animals fight by their species attack. | One attack each (3.3). | Multiple attacks. | CBT-2 | partial |

### 3.10 Colonist AI

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Colonists seek cover when shot at. | None. | Cover seeking with hysteresis. | CBT-4 | missing |
| Colonists flee or fight when undrafted (see 3.1). | Auto-engage only. | Threat response. | CBT-4 | missing |
| Tantrum mental break (wiki: Mental break). | None (grep finds no tantrum code; mental breaks are the SIM-061..066 set). | Tantrum. | CBT-4 | missing |
| Hunting (see 3.6). | Rifle only, no stalking. | Any weapon, stalk. | CBT-4 | partial |

### 3.11 Combat experience

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Shooting and Melee experience is gained per attack with different rates for hits and misses (wiki: Skills). The exact vanilla rates are not published in the pages used. | Flat `combatXP` 120 per attack, hit or miss (`game/skills.go` about lines 36, 422); `learnCapDay` 4000. | Rates differ by hit or miss; original tuning where the wiki gives none. | CBT-2 | partial |

### 3.12 Prisoners

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Prisoners can break out when unrest and opportunity allow (wiki: Prisoner; the page gives no timings). | No prison break. | Prison break with hysteresis and original timings. | CBT-4 | missing |
| Capture, recruit and rescue. | Existing prison systems (`game/prison`); out of this document. | None. | - | done |

### 3.13 UI

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Minimum range ring of a weapon. | `combat_overlay_host.go` (about lines 110-117): no minimum ring. | Draw the ring. | CBT-5 | partial |
| Cover overlay shows natural cover with direction-aware values. | Shift+O overlay; natural cover missing (about lines 122-136). | Natural cover and angles. | CBT-5 | partial |
| Target highlight for a shooter's target. | None. | Highlight. | CBT-5 | missing |
| Per-pawn kill and damage records. | `combat/log.go` is a colony-wide ring; no per-pawn records. | Per-pawn counters on the pawn view. | CBT-5 | missing |
| Melee damage and cooldown shown in tooltips. | Gear tab shows `WeaponStats`; no melee tooltip. | Melee tooltip. | CBT-5 | partial |
| Door forbid control. | None. | Control and command. | CBT-5 | missing |
| Combat log. | DEF-4 `combat/log.go`, `WorldView.CombatLog`. | None. | DEF-4 | done |

### 3.14 Sounds and effects

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Weapon-specific fire, hit and impact sounds; per-kind blast effects (fire, EMP, smoke). | DEF-2/DEF-3 sounds for existing weapons; one generic blast art. | Sounds for every new weapon and gear piece (skill `rimworld-go-art-assets`; sfxgen spending approved first); per-kind blast art; art for uranium and reinforced barrel stand-ins. | CBT-3 (gear), CBT-5 (blasts) | partial |

## 4. Wiki constants to carry

Every constant is named in code with `// wiki: <page>`.

| Constant | Value | Page | Phase |
|---|---|---|---|
| Per-tile shooting accuracy | skill 0 -> 89 %, 10 -> 97 %, 20 -> 99 %; Careful +5, Trigger-happy -5 | Shooting accuracy | DEF-1 (done) |
| Weapon bands | touch 3, short 12, medium 25, long 40 | Shooting accuracy | DEF-1 (done) |
| Miss radius | 100 % 1.0, 22 % 2.0, 11 % 4.0, 7 % 6.0, 4 % 8.0, 2 % 10.0 | Combat | CBT-2 |
| Interception | 40 % x body size, 4-80 %, none <= 5 tiles, full at 12+ | Combat | DEF-1 (done) |
| Stopping power | body size <= power: 1/6 speed, 95 ticks | Combat | CBT-1 |
| Melee hit score | skill (+4 unskilled) + 12 x (min(sight, 150 %) - 100 %) + same for manipulation + 4 Brawler; -20 -> 5 %, 0 -> 50 %, 20 -> 90 % | Melee hit chance | CBT-2 |
| Melee damage variation | x0.8..1.2 | Combat | CBT-2 |
| Overkill windows | Cut 0-10 %, Crush/Blunt/Stab 40-100 %, Bullet/Arrow/Scratch 0-70 %, Bite 0-10 % | Injury | CBT-1 |
| Multi-part cut | 1/2/3/4 parts: 30/45/20/5 %; blunt +40 % internal | Injury | CBT-1 |
| Explosion multipliers | plants and impassable x4, passable x2, walls x8 | Damage types | CBT-5 |
| EMP | 30 ticks per point; 50 -> 1500 ticks; adapted 2200 ticks | Damage types | CBT-5 |
| Blunt stun | torso >= 90 % max HP 15 %; brain >= 50 % 100 % | Damage types | CBT-1 |
| Armor layers and coverage groups | Skin, Middle, Outer, Belt, Headgear, Eyes; groups listed in 3.5 | Armor | CBT-1 |
| Cover angle | < 15 deg 100 %, 15-27 80 %, 27-40 60 %, 40-52 40 %, 52-65 20 %, > 65 0 %; point-blank 33 %, one tile 67 % | Cover | CBT-2 |
| Auto-undraft | 10,000 ticks | Drafting | CBT-2 |
| Spike trap | 5 stab hits scaled by stuff; enemies 100 %, wild animals 20 % | Spike trap | CBT-3 |
| Pain, bleed, infection, scar, healing | See 3.4 | Injury | MED (done) |

Not published (original tuning, named in the phase handoff): combat XP per hit and per miss, prison break timings, shield belt and smokepop behaviour beyond the item page, tantrum duration.

## 5. Evidence corrections (spot-check, 2026-10-30)

About 20 of the inventory's evidence lines were checked by grep. The facts held (wound-kind mapping, armor regions, bleed, 3-slot apparel, `g.equip`, flat XP 120, melee hit curve, ignite at the centre, no drop command, no tantrum, no launchers, no inventory). These paths or claims were stale:

- `game/mentalbreak.go` does not exist (mental break code is in other files); the absence of a tantrum is confirmed by grep.
- `game/draft.go` and `game/explosion.go` live in `game/combat/` (`draft.go` line 187, `blast.go` line 50); `game/defense.go` and `game/hunt.go` likewise (`combat/defense.go`, `combat/hunt.go`).
- The sentry turret "ignores armor": `combat/turret.go` calls `armorRoll` at about line 446, so armor applies to at least the turret hit path. The "instant hits" claim stands, but the weapon comment still says "until DEF-3", which shipped.
- `filter.go` is `domain/building/zone/filter.go`, `item.go` is `domain/economy/item/item.go`.
- `strike.go:159` is now about lines 156-159 (the `Hurt` call after `armorRoll`).

## 6. Checklist cross-reference

| Checklist item | State now | Phase that closes it |
|---|---|---|
| Vanilla 17.5 **Adjacent melee combat** | `[-]` (was ticked wrongly) | CBT-2 |
| Vanilla 16.1 **Single equipped primary weapon** (equip/drop) | `[-]` (was ticked wrongly) | CBT-2, CBT-3 |
| Vanilla 11.3 **Damage types** | `[-]` (was ticked wrongly) | CBT-1 |
| Vanilla 17.5 **Weapon/body attacks** | `[-]` | CBT-2 |
| Vanilla 17.6 Armor and 16.2 Apparel | ticked for the DEF-2 slots | CBT-1 (layers), CBT-3 (raider kits) |

## 7. Out of scope

| Item | Missing prerequisite | State |
|---|---|---|
| Breach raids, mech clusters beyond core, psycasts and royal weapons, anomaly threats | DLC (Royalty, Biotech, Anomaly) | out of scope (DLC) |
| Mod-defined weapons | Plugin system (checklist 36) | out of scope |
| Medical treatment of wounds | `MEDICAL_GAP_ANALYSIS.md` | covered there |
