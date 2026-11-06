# CBT-3: weapons, gear and structures

Checklist 16.1 (now `[x]`), 16.3 single-use launchers, 17.8 shield belt and smokepop, 24.2 equipment by tech, turrets and traps.

## What changed
- New weapons with wiki numbers: rocket launcher, triple rocket launcher (single-use, consumed by one trigger pull), smoke launcher, charge rifle, charge lance, ikwa (melee tools). `WeaponStats` reports `MinRange` and `SingleUse`; no launcher has a wiki minimum range.
- `combat.Gear` (`game/combat/gear.go`): the equipped weapon keeps quality, stuff and wear, saved in the Turrets block (old saves load as Normal steel). Melee damage scales by stuff; accuracy and damage by quality; drop and spare keep the make. `PawnView` gains `WeaponQuality`, `WeaponStuff`, `Spare`, `ShieldPct`, `Smokepops`, `Kills`, `DamageDealt`; gun-stock blows carry the gun's name; `TraceView.Dodged`.
- Shield belt in `game/hit`, before armor: wiki energy, recharge and a 3200-tick break; EMP breaks it; the wearer cannot shoot or throw. Smokepop pack has 3 uses; raiders pop it when shot. The lead added the `PopSmokepop` command.
- Raider armor kits in `domain/combat/kit` by tech and weapon cost (wiki pawn kinds); raider views show Apparel, Worn and ApparelArmor.
- Mini-turret wiki numbers; every turret shot is a projectile (strays, friendly fire, armor); a turret explodes half the time on destruction.
- Spike trap: 5 stab hits scaled by stuff; enemies and manhunters 100 %, other wild animals 20 %.
- Rearm: slug turret wants uranium, mortar a reinforced barrel; neither is obtainable, so steel stands in.
- Kills and damage are counted; a death from wounds credits the last attacker. `craft.Slots` is now 5 (belt layer), `Outfit.Armor()`; `ArmQuality` dev hook.

## Wiki versus original
Wiki: weapon stats, shield energy and break time, mini-turret, spike trap, raider pawn kinds. Original: smokepop uses and pop rule, zone of kit costs where the wiki gives none.

## UI
See CBT-5 (gear tab, shield charge, smokepop button, drop).

## Tests
Coverage: game 91.9 %, combat 95.2 %, weapon 96.4 %, kit 100 %. Game -6 lines.

## Gaps
Uranium and reinforced barrels have no source. Explosions do not count toward records.
