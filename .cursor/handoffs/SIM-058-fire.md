# Task Handoff

Task ID: SIM-058 (PLAY-015 §5)
Status: complete

## Summary
**Fire domain** (`world/fire`), pure rules with our numbers:
- growth +4 per step with fuel, −6 without;
- spread from size 35 at flammability × size/100 × 0.06, ×1.5 downwind, ×0.2 into roofed cells;
- outdoor rain or snow ≥ 0.3 takes −12 per step.

**Game** (`fire.go`, every 30 ticks, per-chunk lists):
- **Ignition:** lightning, one strike per 2-hour slot in storms (stateless schedule, `WorldView.Strikes`); bursting animals; the dev "Start fire".
- **Damage:**
  - buildings 1 + size/12 per step; a building that burns down leaves no refund and no blueprint, plus a message and History; bridges use the collapse cascade;
  - plants, grass and items burn away;
  - pawns get Burn wounds (`CauseBurns`), animals Injury;
  - ash stains are left behind.
- **Heat:** a size-50 fire heats its room like a campfire; outdoors it warms 5×5.
- **Randomness:** rolls are hashed from the seed, tick and cell, so the main random stream never shifts.

**Firefighting** (`firefight.go`):
- Firefight is the first work type, on by default (priority 3);
- the FightFire job takes −10 size per 30 ticks;
- eligible fires are inside Home AND the pawn's area (Unrestricted means Home only);
- drafted pawns can be ordered to Extinguish anywhere.

**Danger** (`fire_danger.go`):
- path cost +1900% on fire, +200% beside it;
- idle pawns step out (600-tick hold, no-flap tested);
- animals flee within 4 cells;
- the alert "Fire in the colony!".

## Notes
- Scale goldens were re-pinned because only Firefight priorities changed (checked field by field on all six).
- Old saves keep Firefight off until the player turns it on.
- Benchmark: a 500-cell blaze takes 0.08–0.17 ms per step.
