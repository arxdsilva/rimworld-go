# DEF-2: weapons, armor, explosions

Date: 2026-10-29. Wave C. Backlog `.cursor/DEFENCE_GAP_ANALYSIS.md`.

## What changed
- **Catalogue** (C2, opus): 30 weapon and armor item kinds. Melee: mace, gladius, longsword, pila beside club, knife and spear. Ranged: short and recurve bows, greatbow, revolver, autopistol, machine pistol, assault and sniper rifles, pump and chain shotguns, heavy SMG, LMG, minigun, incendiary and EMP launchers. Grenades: frag, molotov, EMP. Armor: flak pants, vest and jacket, plate armor, marine armor, simple and advanced helmets. Weapon names are singular ("frag grenade"); item kinds plural. Rocket launchers skipped.
- **Explosions** (`domain/combat/explosion`, `game/combat/blast.go`): damage full at the centre to half at the rim, line of sight from the centre. Bomb x2 against buildings; flame ignites; EMP stuns turrets (`AimSave.StunnedUntil`); smoke cuts aim to 30 % (`Blasts.SmokeFactor`; clouds saved as `Smoke []explosion.Cloud`). Grenades arc over cover and scatter at range. Explosive weapons are blocked from hunting (`TestNoHuntingWithExplosives`).
- **Raid kit** (`domain/combat/kit`): `kit.Pick` by faction tech and points. Raiders wear no armor.
- **Armor** (`domain/combat/armor/apparel.go`): `AppendPiece` scales plate x0.9, simple helmet x0.5, advanced helmet x0.7 by stuff (steel fallback) and by quality; `combat_host` reads the worn pieces; dressing handles the Legs slot.
- **Read models:** `PawnView.WeaponStats` (`weapon.View`), `ApparelArmor []armor.Set`; `BurstView.Blast` and `Radius` (tenths); explosions leave a Landed trace.
- **Art** (A2): `assets/source/blender/artlib/families/{arms,armor}.py`, 22 weapons and 8 armor pieces, validator 0 errors over 5,576 sprites. Renderer (RW): `render/world/keys.go` item keys ("EMP grenades" → `item/emp_grenades`), `render/creature/held.go` keys for all 22 weapons, `render/creature/worn.go` draws `PawnView.Worn` in the order shirt → flak pants → vest → parka → jacket → plate/marine → beard → hair → helmet with stuff tints (zero allocations, `AllocsPerRun`), `Bus.ListenBursts` fed from `WorldView.Bursts` in `advanceTicks` (volume by radius); the heard-blasts set now survives `ListenWorld`'s memo rebuild. Tests check every kind has a sprite and every weapon/armor sprite has a reader; `make art-check` 0 orphans.
- **Sound** (S2): cues re-keyed for the new weapons; `ListenBlasts` plays explosion sounds by Blast kind.

## Wiki numbers versus original tuning
Wiki: weapon stats, armor ratings and coverage, blast radii, fuse, bomb x2 vs buildings. Original tuning: the full-to-half falloff and the scatter at range. Designs and names are original.

## UI
Gear tab `WeaponStats` lines and per-garment armor lines (`inspector/gear_facts.go`, `GearStatLines`); blast sounds and burst/blast read models for the renderer.

## Tests and numbers
`domain/combat/{explosion,kit,armor,weapon}` tests, `game/combat/blast_test.go`, `game/weapons_armor_test.go`. Coverage: explosion and kit 100 %, armor 93.8 %, weapon 90 %, game/combat 96.6 %. Game package 9 lines smaller. Bench x3 frame p99: grown/250 1.44 ms, grown/500 1.43, raid/250 2.17, raid/500 1.98.

Journeys (`journey_wave_c_test.go`): `TestJourneyRoomCardStats`, `TestJourneyCraftedItemShowsQuality`, `TestJourneyGearTabWeaponStats`, `TestJourneyBurstFire` and `TestJourneyGrenadeExplosion`. Burst-fire and grenade were fixed in integration (the raid alert pauses the game after `RaidArmed`, so the journeys were updated); all five pass with `-race` (re-run by DC for the last two).

## Gaps and follow-ups
- Melee weapons not stuff-scaled; wielded weapons lose crafted quality and stuff on equip.
- Rocket launchers; single-use rule for launchers.
- Raider armor; colonist grenade throwing (DEF-4).
- Explosions do not damage items and do not chain; no x4/x8 plant and wall multipliers.
- Recipes for the catalogue arrive in PRD-4.
- No per-kind blast art (fire, EMP, smoke clouds); armor not drawn on portraits or the colonist bar; ground items not stuff-tinted.
- Draw benchmark (BenchmarkDrawArt500, wave B → wave C): draw calls 419 → 425 at z1, 575 → 590 at z0.5, 271 → 303 at zmin; allocations 3,636 → 3,801, 3,413 → 4,026, 2,024 → 2,517. The memory profile puts the growth in ebiten's per-draw-call path (more sprites in the changed scene: kit weapons, quality furniture); the new render code allocates nothing.
- Sim note: `countRoomBuildings` (BED-2) calls `construction.Footprint`, which allocates per call during room rebuilds; a non-allocating footprint walk would remove it.
