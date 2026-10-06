# Task Handoff

Task ID: PLAY-010
Status: complete (phase 5b: closing combat/hunting gaps)

## Summary

- Doctor work (on by default; Work panel "Doc" column is live).
  - Rescue: a doctor carries an injury-downed colonist to the nearest free bed. The patient follows the carrier tile by tile and is laid in the bed, which they claim.
  - In bed, patients heal 4x faster than on the ground.
  - Tending: 300 work ticks heal 8 injury immediately, and healing runs 2x faster for a day. Patients are tended in place when no bed is free.
  - Interrupted rescues put the patient down where the rescuer stands.
- Corpses: kills leave a carcass item ("dustbuck corpse" / "ridgewolf corpse") instead of meat. Haulers store carcasses in food stockpiles.
  - A predator kill feeds the predator and leaves the carcass for scavenging.
- Butcher table: a 2x1 bench (Production menu, art in 4 facings) that is built with "Butcher dustbuck/ridgewolf, forever" bills. Butchering is Cook work: 1 carcass becomes 12 or 6 meat.
- Self-defence: an undrafted colonist that an animal is attacking within 3 tiles fights back if armed (club/rifle). Otherwise it runs about 7 tiles away and re-plans every 30 ticks.
- Alert list: a stack on the right edge, rebuilt from the read model each frame. It replaces the centre banner.
  - Red: animal attacking, "X needs rescue", "X is starving".
  - Amber: collapsed from exhaustion, low food (fewer than 4 food items per colonist), unhappy.
  - Repeated alerts are grouped ("2 colonists need rescue").
  - Clicking centres the camera on the next target in a cycle.
- Art: corpses (animal models rolled legs-up over a small pool, `item` family), the butcher table and their icons.

## Public Contracts Changed

- item `CorpseDustbuck/CorpseRidgewolf`, `IsCorpse()`; `animal.Def.Corpse`.
- construction `ButcherTable`; recipes `ButcherDustbuck/ButcherRidgewolf`.
- jobs `Rescue/Tend`; command `WorkDoctor`; `work.DefaultColonist` includes Doctor.
- PawnView `WorkDoctor/Lying/InBed/CarriedBy/Tended`.

## Verification

- `go test ./...` green.
- New tests:
  - rescue to bed + tend + recovery
  - patient moves with the rescuer
  - butcher table starts with bills and turns a carcass into 12 meat
  - unarmed colonist flees / armed one fights back
  - alert building/grouping/severity, and alert click moves the camera
- Combat tests now expect carcasses.
- Asset validator: 216 sprites, 0 errors.

## Known Limitations

- No medicine items, no infections, no self-tend; downed starving colonists are not fed.
- Carcasses never rot.
- Lying, carried and dead pawns are still drawn standing; this is next, in the animation pass.
