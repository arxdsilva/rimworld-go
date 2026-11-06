# DEF-3: cover, traps, turrets, mortars and doors

Date: 2026-10-29. Wave D. Backlog `.cursor/DEFENCE_GAP_ANALYSIS.md` (sections 3.3, 3.5). Contract commit `8f3932359` (arch(contract): wave D kinds, commands SetDoorHoldOpen/SetMortarTarget/ClearMortarTarget/SetTurretHold, jobs ManMortar/RearmTurret, BuildingView HoldOpen/Barrel/Shells/TargetX/TargetY/HasTarget/HoldFire) came first.

## What changed
- **Cover** (`domain/combat/cover`, `Sight`): permille cover from buildings, rock, grown trees, cacti, bushes and stone chunks (chunk tiles cached 250 ticks). Wall and door 40 to 75 (wiki), barricade 55, turrets 40/50. `CoverPct` deleted; the overlay uses `cover.OfBuilding`.
- **IEDs** (`game/combat/ied.go`): 15-tick fuse, blast, re-laid blueprint. High explosive radius 3.9; incendiary, EMP and smoke burst as their shell.
- **Turrets:** autocannon and slug turret with wiki stats. Barrels 60/90/30 rounds; a spent barrel stops firing; steel per barrel 9/20/7; Hold fire. Weapon defs "autocannon", "slug turret"; Barrel returns to 1000 on swap.
- **Mortars:** crew order (drafted colonist on its spot, else `SetMortarTarget.Pawn`, else nearest idle drafted within 12 tiles); range 29-500, minimum 29.9, warmup 240, cooldown 1680, forced miss radius 9 scaled by crew Shooting; 20 shells per barrel. Shell items "mortar shells", "incendiary shells", "EMP shells", "smoke shells". `RearmTurret` hauls shells and barrel steel (idle undrafted colonists).
- **Doors** (`domain/building/door`): `Kind.IsDoor()` (Door or Autodoor) at 17 game sites and the adapters. Opening speed by stuff (wiki); powered autodoor faster; 45-tick open and 120-tick auto-close; never closes on someone. HeldOpen lets hostiles through (cached set, structVer bump); Barred = forbidden door unwalkable to colonists. Open doors let shots through.
- MaxHP (wiki): autodoor = door, barricade = sandbags, heavy turrets 380, mortar 180.
- Game package moved blocks into `game/combat` (assignHunt/assignEquip, walkableNear, TraceView, held-door filter); now 54,494 lines.

## Wiki numbers versus original tuning
- Wiki: cover values, IED fuse and high-explosive radius, turret and mortar stats, door opening speeds, max HP.
- Original: incendiary/EMP/smoke IED bursts (no wiki page), steel per barrel (wiki cost / 9), door base open 45 and auto-close 120 ticks, the 12-tile crew search.

## UI
U4: `inspector/cmds_defence.go` (door Hold open, turret Hold fire, mortar Set target and Clear target, barrel/shells/target lines); `orders/mortar_aim.go` aim mode (next map click sends SetMortarTarget with the single selected colonist, right-click or Esc cancels, aim marker drawn); renderer draws the autocannon, slug turret and mortar guns from a precomputed key table (mortar aims at its target); cover overlay shows `cover.OfBuilding` percent.

## Art and sound
- A3: families `fortify` (barricade with `stone_tint` mask, turret guns, powered door) and four shell item icons; validator and `make art-check` clean.
- S3: 14 procedural sounds in `cmd/audiogen/structure_recipes.go`, those for DEF-3: autocannon and slug turret shots, mortar launch and incoming, turret rearm, IED arm, autodoor open and close; cues in `render/cue/structure_sounds.go`.

## Hysteresis
Mortar cooldown 1680 and door close delay 120 ticks keep actors from flapping; a spent barrel stays silent until rearmed.

## Tests and numbers
Journeys `TestJourneyMortarForcedTarget` (48x48 map, target 32 tiles out, burst within 10 tiles), `TestJourneyIEDTrigger`, `TestJourneyDoorHoldOpen`, `TestJourneyTurretHoldFire` (`journey_wave_d_test.go`). Coverage: game/combat 95.1 %, cover and door 100 %, weapon 92.7 %. Frame p99 grew 1.36/1.24 ms; raid 1.61/1.75 ms.
Integration (lead): the mortar aim hint moved from a raw draw to the shell's Hint channel (canvas guard; `drawAim` is now world-only outlines). BenchmarkDrawArt500, contract commit → wave D: draw calls 425 → 407 (z1), 590 → 548 (z0.5), 303 → 266 (zmin); allocations 3,801 → 3,228, 4,026 → 2,983, 2,517 → 2,223; z1 1.00 → 0.84 ms. TestGrownGolden re-pinned (wave D reason in grown_test.go).

## Gaps and follow-ups (DEF-4)
- Bullets setting off IEDs, explosion chains, sappers avoiding IEDs, raiders using mortars.
- Door forbid UI (Forbid only on sleeping places).
- Uranium and reinforced barrel items; steel stands in.
