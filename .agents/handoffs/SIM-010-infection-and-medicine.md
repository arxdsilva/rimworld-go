# SIM-010 Infection, herbal medicine, healroot

## What changed
- `domain/pawn/infection`: `Care` (untreated / self-tended / tended / medicine),
  `Chance(care)` (40/20/12/3%), `Step(severity, treated, care)` hourly (+4 / -5 / -9),
  `Onset` 15, `Fatal` 100, `Sick` 20, `DelayHours` 8.
- `item.HerbalMedicine`; `plant.Healroot` crop (6.5 days, 2 medicine a plant, soil
  sensitive) in the crop picker; stockpiles accept medicine.
- `health.CauseInfection`; mood `Sick` (-8) and `InPain` (-6, injury >= 40).
- `internal/application/game/infection.go`:
  - `g.wounds` (saved): a colonist's open wound (`At`, set by `hurt`) and infection.
  - `phaseInfection` (hourly): a wound untended for 8h may fester; infections grow
    unless tended (a tend lasts a day), kill at 100, end at 0 (messages for each).
  - `patientRest`: sick (>= 20) or badly hurt (>= 40) colonists with another doctor
    around lie down and wait (up to 6h past a normal wake-up).
  - Doctors tend anyone lying down (downed, in bed, asleep) who needs it, fetching
    one dose of herbal medicine first when available (`withMedicine` /
    `pickUpMedicine`). With no other doctor, a colonist tends itself (weaker).
  - Tend quality: injury healed 8 (medicine 16, self 4); closing a wound rolls
    infection by care.
- PawnView `Infection`, `Wounded`; inspector status shows "infection N%",
  "wound untended", "(tended)".
- Art: `plant/healroot_*`, `item/herbal_medicine`, icon. `-scenario medic` /
  `-capture-ui medic`.

## Tests
`infection_test.go`, `domain/pawn/infection`: untreated infection kills; untended
wounds fester sometimes; a doctor fetches medicine for a sick colonist and the
infection recedes; a lone colonist self-tends; healroot yields medicine.
