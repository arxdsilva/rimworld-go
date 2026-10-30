# CBT-5: explosions and combat UI

Checklist 17.7, UI 12.3, 13.3, 13.4, 14.1, 24.3 records.

## Simulation
- Bomb x2 on passable buildings, x4 on impassable ones and plants, x8 on walls (wiki); EMP 30 ticks per point with a 2200-tick adaptation for turrets (wiki).
- Items: bombs wear weapons and apparel and destroy them at 0 HP; burnable raw stacks are destroyed by a hit of 50 or more; stone and metal are immune (original thresholds).
- Cook-off: shells and grenades in a bomb or flame blast, or lying in fire, go off 30 ticks later through the IED fuse queue (`Fuse.Stack`, saved), at most 8 a tick, each stack once (original tuning).
- Flame ignites every cell in the radius by flammability; a creature's cell 70 % (original); the centre always.
- `WorldView.Smoke` lists live smoke clouds. `looseAt` uses the ground index; `causePhrase` became `health.CausePhrase`.
- Raid hashes match HEAD for this change alone; explosion coverage 100 %.

## UI
- Minimum range ring from `WeaponStats.MinRange`; the cover overlay includes natural cover and is direction-aware with a hovered hostile (`AngleFactor` x `RangeFactor`); a target ring on the hovered enemy or the last one shot.
- Gear tab: weapon quality and stuff, a melee and ranged damage and cooldown tooltip, spare weapon, kills and damage, shield charge bar, smokepop Use button, threat response control.
- Door pane Forbid/Allow toggle (`bldg.forbid`). History window Records tab (kills and damage per colonist). "dodged" floaters.
- Blast effects (fire 50 ticks, EMP 30, smoke 1200, fading over the last third); shield bubble while `ShieldPct` > 0; `ListenShields` feed.
- Held weapon keys for the six weapons; belt worn layers (after plate and marine, before beard); hive mound build key.

## Art and sound
Art: `arms.py`, `belt.py`, `effect.py`, `hive.py`; validator 6020 sprites, 0 errors. Sound: 10 procedural sounds (`wave3_recipes.go`: rocket launch, smoke launcher, charge rifle, charge lance, shield pop, smokepop burst, stagger thud, gun-stock hit, paste dispense, tantrum smash), cues in `render/cue/wave3_sounds.go`; audio 24.0 MB of 30 MB.

## Tests
`journey_wave3_test.go`; `blast_cbt5_test.go`.

## Gaps
Explosions do not count toward records. Mech voices blocked. Light accuracy factor still fixed.

## Integration (lead)
- `TestJourneyMechOpensDoor` now requires a real break-in: the door bashed, a room wall knocked down, or a mech inside. Stray shots chipping a wall no longer count, and mechs breaching a wall instead of the door is accepted.
- The colonist command row now has nine buttons (wave 2 Drop weapon, wave 3 smokepop and threat). On a 1280-wide window it reaches the Global panel, so `TestHUDRightClickNeverStealsADraftedMove` picks a map point above the grid. Watch the crowding; a second row or overflow menu may be needed as more commands arrive.
- PopSmokepop and SetThreatResponse commands, `PawnView.ThreatResponse`, and scenarios may now give the new kinds.
- Golden tests re-pinned (wave 3 reason); full -race suite 4 min 35 s on a quiet machine.
- BenchmarkDrawArt500, wave 2 commit → wave 3: draw calls 410 → 429 (z1), 570 → 583 (z0.5), 270 → 270 (zmin); allocations 3,457 → 3,901, 3,319 → 3,574, 2,734 → 2,361. The memory profile shows no allocation in the new render code (blast effects, cover overlay, shield feed, belt layers); the growth follows the extra draw calls of the changed scene (armored raiders draw more worn layers).
