# MED-09: animal medicine

Part of the RimWorld-style medical overhaul (`.cursor/MEDICAL_GAP_ANALYSIS.md`).

## Behaviour
- **Body adapter** (`game/animal_health.go`): a hurt animal gets the human body model with every wound on one part (Torso).
  - The body is created on the first wound and dropped once nothing needs care.
  - It reuses MED-01/02 bleeding, tend quality and `CondInfection`.
  - Bleeding scales by species size: MaxHP against a human's 100, clamped to 0.1×–10× (original tuning).
  - Animals get no scars (original tuning).
  - `Injury` stays the hit-point figure; healed wounds come off it.
- **Deaths**: blood loss and infection kill animals. There is still no downed state for animals; adding one would change behaviour across `animals.go`.
- **Hourly step**: over a sorted animal patients index (rebuilt after load, never saved). There is no scan of all animals.
- **Vet work**:
  - A tame animal with a body is tended when due: once per new wound, or once per 12 h infection window.
  - A tend stops bleeding, speeds healing and lowers infection odds; it no longer heals instantly.
  - The care level (per animal, else the Animals default) caps medicine; No care is skipped.
  - Animals without a body (old saves, starvation) keep the old behaviour.
- Wild animals now bleed and heal at the wiki rates.

## UI
- The animal inspector shows bleeding with hours to death, blood loss and stage, infection versus immunity, and "Needs tending".
- A Care command opens a five-level picker that sends `SetMedicalCare`.
- The Animals table health cell shows "62%!" while an animal bleeds or is infected.

## Save
The body is stored in `animal.State.Body`, only when present. Old saves are unchanged.

## Verify
- New tests:
  - `TestAnimalBleedsAndIsTended`
  - `TestAnimalBleedsOutUntended`
  - `TestAnimalWoundCanInfect`
  - `TestAnimalCarePolicySkipsNoCare`
  - `TestAnimalHealthSurvivesSave`
  - `TestOldSaveAnimalsLoad`
  - `TestAnimalTendDoesNotFlap`
  - `TestAnimalInspectorHealthBlock`
- Scale goldens are unchanged. The grown scenario moved (MED-09's combat hook alone moves it) and was re-pinned by the lead with MED-07/08.

## Follow-ups
- Per-species anatomy.
- A downed state for animals.
- Stain drips for animals still use the half-HP rule.
- Tame animals that die of wounds get the generic death message.
