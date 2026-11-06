# MED-13: per-archetype animal anatomy

Original part sets and tuning. This replaces MED-09's one-part animal body adapter.

## Body plans (`body/anatomy_plan.go`)
`Body` gains `Anatomy` and `Scale` (‰ of human hit points; 0 means 1000). Both are zero for humans, so human saves and hashes are unchanged (`TestHumanlikeUnchanged` pins a hash of the human table and its results).

| Anatomy | Parts | Notes |
|---|---|---|
| Humanlike | 24 | unchanged |
| Quadruped | 26 | head organs, torso organs, spine, 4 legs, 4 paws, tail |
| Bird | 23 | wings in place of front legs (they serve no capacity), beak, 2 legs and feet |
| Snake | 15 | no limbs; the vital spine moves it |
| Lizard | 23 | 4 legs, 4 feet, long tail |
| Insect | 18 | 6 legs, antennae, mandibles, carapace, nerve cord |

- Coverage sums to 1000 in every plan. Each plan has at most 32 parts.
- Part hit points scale with the species (`animalHealthScale`). Pain is measured against the scale.
- Capacities: each one averages its limb chains (leg × paw) and multiplies by `mul` parts (spine, liver).
  - A capacity no part serves stays at 100 %.
  - Losing one leg: quadruped moves at 75 %, bird at 50 %, insect at 83 %.
- Archetype → anatomy (`animal.AnatomyOf`):
  - Quadruped, Primate, Brute → Quadruped. Primates get no hands, so no animal sits on the humanlike plan.
  - Bird → Bird.
  - Snake → Snake.
  - Lizard, Tortoise → Lizard.
  - Beetle, Crawler → Insect.
- Implants, replacements, operations and aging refuse non-humanlike bodies.

## Simulation
- **Hits:** `woundAnimalBody` picks a part by coverage on `streamMedical`. A part whose loss would kill is never destroyed; the damage spills on. Limbs and eyes can be lost.
- **Moving:** an animal's speed scales with Moving. A new downed condition applies: Moving below 15 % downs it, and it stands only at 30 % (hysteresis, `TestAnimalMovingDownNoFlap`). This sits on top of MED-12's rules.
- **Body lifetime:** a body with lost parts is kept. Only bodies needing care are in the patient index.
- **Migration:** on load, the old one-part bodies (`Scale == 0 && Anatomy == Humanlike` on an animal) get their species plan. Their wounds and conditions move to the root part.

## UI
`AnimalView.Parts` and `AnimalView.MovingPct` feed the animal inspector. It shows "Moving 75%", lines like "Torso: 12/28 HP, bleeding" and "Front left leg: missing", up to 6 lines plus "+N more".

## Verify
- Tests:
  - body: `anatomy_test.go`;
  - animal: `anatomy_test.go`;
  - game: `animal_anatomy_test.go` (migration, save round-trip, hits on parts, surgery refused);
  - UI: `TestAnimalInspectorPartLines`.
- The full suite passes under `-race`.
- `TestGrownGolden` was re-pinned `0x960cb818b7a41587` → `0xf2889f4a11cbdce0`. Per-100-tick dumps match HEAD until the first wolf wound. `TestScaleGolden` is unchanged.

## Left open
- An animal that loses every leg stays down for good. Nothing euthanizes it automatically.
- Animals get no scars.
- Butchering yield ignores lost parts.
- Food poisoning, overdose and anaesthetic conditions still name the human torso slot. No animal path creates them yet.
