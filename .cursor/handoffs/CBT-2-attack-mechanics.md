# CBT-2: attack mechanics

Checklist 17.5 melee `[x]`, 17.1 auto-undraft `[x]`, 17.3/17.4 cover and accuracy notes; 16.1 stays `[-]`.

## What changed
- `projectile.Shot.Hit` set at fire time and saved (old saves rebuild via `shotHit`); nothing reads the attacker's current weapon at landing.
- Melee (`combat/melee.go`, `game/skills.go`): wiki hit curve (skill 20 gives 90 %, skill-less creatures 62 %), wiki dodge (`attack.DodgeChanceOf`), damage x0.8-1.2, per-mille rolls.
- Tools: wiki tools per melee weapon; humans adjacent use a gun's stock, grip or limb; humans fist, head, teeth; animals their roster attack plus a second one.
- Euclidean ranges (max and min), turrets and mortar included; the autocannon refuses 8 tiles.
- Weather factors (unless both ends are roofed); cover checks 8 tiles, angle bands, 33/67 % at close range, combined covers.
- Downed humans are attacked only when ordered; auto-engage covers manhunters, mechs and insects; grenade orders walk into range then throw.
- Auto-undraft after `AutoUndraft` = 10,000 quiet ticks, re-checked before release (no flap).
- `DropWeapon` drops or stows into a new spare slot (`domain/pawn/inventory`, saved); the spare falls when its carrier dies.
- Combat XP per second of attack cycle: 80 on a hit, 30 on a miss. Coverage 95-100 %; -2 game lines.

## Wiki versus original
Wiki: melee hit curve, dodge, damage variation, 10,000-tick undraft, cover angles, close-range 33/67 %. Original tuning: animal second attack (60 % share, 25 % chance), combat XP 80/30.

## UI
Drop weapon button (see STO-3).

## Tests
Journeys MeleeDodge (hits and misses only), AutoUndraft, DropWeapon; `TestJourneyFireAtWillOffHoldsFire` adapted (stock melee when adjacent counts as engaging).

## Gaps
`PawnView.Spare` not exposed. A dodge cannot be told from a miss in `TraceView` or the log, so no dodge-specific assertion. Gun-stock blows need a blunt sound (wave 3). Weapons still lose quality on equip (CBT-3).
