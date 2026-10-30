# CBT-1: damage model

Checklist 11.3 Damage types `[x]`, 17.6 Armor coverage `[x]`; UI Staggered status.

## What changed
- `damage.Hit{Weapon, Form, Amount, Pen, Stopping}`; forms Blunt, Cut, Stab, Bullet, Arrow, Scratch, Bite, Crush, Burn, Flame, Bomb, EMP, Stun. `weapon.HitOf`, `armor.Landed`, `combat.Wounds` role (`Absorb`, `HurtHit`).
- Body-part-first armor: `armor.Struck` with coverage groups, plies and `GroupOf(part)`; coverage, layers and garment HP derived from kinds (save format unchanged).
- New wound kinds Stab and Gunshot: bleed, scar, fester at 10 %; spike traps stab.
- `body.Strike` (`domain/pawn/body/hit.go`) with the wiki overkill windows: brain and heart can be destroyed and kill.
- `Outfit.HitWear`: 1 HP per 4 damage; ratty, tattered, destroyed with a message.
- Stagger: step cost x6 for 95 ticks, never extended while running. Stun: 20 ticks per point. Blunt stuns 15 % / 100 %.
- Removed: the 3-region roll, `armorRoll`, `Creatures.Armor`, `Harm.Hurt`/`HurtAs`, `woundKindBy`. New `game/hit` package; `Spread`/`Reconcile`/`TrimOpen` moved into `body`. About -60 game lines, -1 Game method.

## Wiki versus original
Wiki: overkill windows, stopping power 1/6 speed for 95 ticks, blunt stun chances (torso 15 %, brain 100 %), armor layers and coverage, EMP stun per point. Original tuning: 40-tick blunt stun, 1 HP wear per 4 damage.

## UI
`Staggered` on `PawnView` and `AnimalView`; the Health tab names stab and gunshot wounds.

## Tests
Coverage: hit 98.1 %, armor 98.9 %, damage 100 %, body 91.3 %, combat 95.1 %, craft 97.2 %. Journey RifleWoundBleeds. Golden hashes re-pinned.

## Gaps
Multi-part cut table to verify. Raiders still wear no armor and weapons lose quality on equip (CBT-3). Journey HelmetGuardsHeadOnly is skipped (game-level test covers it).
