# SIM-015 Action read model, knife and spear

## What changed
- **Action read model** (`action_view.go`, `actions.go`): `PawnView.Action`
  is a canonical action per job. Attack/Hunt give `aim` (ranged weapon) or
  `melee`, in both Goto and Do. In Do:
  - construct → build, deconstruct, cut → chop, mine, sow, harvest
  - eat, horseshoes, research, tame, repair, breach
  - tend (not while fetching medicine)
  - refuel and bench bills, only while carrying the input. Bills map by
    bench: stove, campfire and drying rack → cook; tailoring bench →
    tailor; butcher table → butcher; others → craft.
  Haul while carrying → haul; rescue while carrying a patient → rescue.
  Animals: flee, bite/kick (attacking an adjacent target, by species
  weapon), graze (a non-predator standing still).
- `ActionProgress` = 1 - WorkLeft/total, with totals from content: frame
  work, deconstruct ticks, plant cut/harvest/sow work, mine work, research
  session, tend work (x1.5 self-tend), tame work, eat ticks, refuel work,
  recipe work. It is -1 when unknown (haul, rescue, attack, breach, repair).
- `AttackCycle`: aim/cooldown countdown progress, capped at 0.99. It is 1
  in the snapshot right after the strike, then resets.
- `ActTarget` and `Carrying` (item kind; "pawn" while rescuing).
- `HitTick`, `HitFromX/Y`, `HitDamage`: stamped on the victim in
  `strike()`, on hits only.
- `TraceView.Attacker`, `Target` (0 for building blows) and `Weapon`, via
  new `projectile.Trace` fields.
- **Knife and spear:**

  | Weapon | Recipe (workbench) | Damage | Cooldown | Accuracy | Raid points |
  |---|---|---|---|---|---|
  | knife | 3 stone, 200 work | 8 | 70 | 90 | 50 |
  | spear | 5 wood, 320 work | 13 | 125 | 85 | 70 |

  Both are range 1. Values: knife 10, spear 20. `kitOf` rolls rifle 50%,
  club 25%, knife 12.5% and spear 12.5%. Recipes are appended, so saved
  recipe IDs are unchanged.
- **Dev:** `Game.Arm`, `Game.RaidArmed`, `-scenario armory|chores`.

## Tests
`action_view_test.go`:
- vocabulary covers every job
- progress for chop, mine, build, sow, harvest, research, tend and tame
- the attack cycle hits 1 on the strike
- traces carry attacker, target and weapon (pawn, bite, breach)
- hits are stamped on pawns and animals
- carrying
- knife/spear stats, recipes, mapping and raid kits

Coverage for `application/game` is 85.5% (was 85.1%).

## Benchmarks
Hour 16.6 → 16.9 ms (+2%); Raid 18.9 → 20.1 ms (+6%). Re-run with the old
raid-kit logic, the per-tick cost is +1.8%; the rest is knife and spear
raiders fighting longer.

## Determinism and saves
No RNG, wall clock or map iteration in the new code. The last-hit and
last-strike stamps are saved additively (`saveState.Strikes`, `Hits`); an
identical save/load continuation required it. Older saves load. Raid
composition and seeds diverge from earlier builds.

## Notes for presentation
A trace first appears with `Age` 1 (the clock advances at the end of
`Step`), so strike frames key on the first sighting (`Age <= 1`).
`HitTick` = strike tick = `Tick() - 1` in that snapshot.

## UI
Knife and spear show up in the workbench's bill list and as wielded
weapons. Combat and work actions are presented by ART-008.
