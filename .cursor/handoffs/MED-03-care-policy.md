# MED-03: medical care policy, doctor work order, emergency tend

Part of the RimWorld-style medical overhaul (`.cursor/MEDICAL_GAP_ANALYSIS.md`).

## Behaviour
- **Care levels** (`domain/pawn/medcare`): No care, Care with no medicine, Herbal or less, Industrial or less, Best available.
  - Group defaults (wiki: Doctoring): colonists Best; prisoners, guests and animals Herbal or less.
  - `withMedicine` caps both tending and surgery medicine by the patient's level.
  - No care: the pawn is rescued but never tended. Animals set to No care are skipped by the vet.
- **Self-tend**: a per-colonist toggle, on by default as before; `selfTendDue` respects it.
- **Doctor work order**: rescue > tend the bleeding > other tends (then self-tend) > surgery > animals > bills.
  - Ties break by bleeding, then injury, then distance, then id.
  - The patient stays reserved until the tend ends, so there is no flapping between patients.
- **Emergency tend**: `job.EmergencyTend`. A drafted pawn tends the target with the best allowed medicine within 15 tiles (original tuning), or with none.
  - It is area-exempt and shown like a tend.
  - It clears if the pawn is undrafted or the target is gone.

## Commands and save
- `SetMedicalCare`, `SetDefaultCare`, `SetSelfTend` and `EmergencyTend` are now real; `ReorderOperation` came in MED-00.
- New save block `Care *careSave`, nil while everything is default. No version bump.

## UI
- Health tab header: five care buttons and a Self-tend checkbox (colonists only).
- Operations list: up and down arrows on each queued operation.
- Assign screen: a Medical column, plus a "Default medical care" row with one button per group.
- Drafted right-click: "Tend \<name>" on a wounded colonist or prisoner.
- Care icons are still to come (UI row 1019 `[-]`).

## Verify
- `go test -race`: medcare (100 %), the ai domain, the full game package and the full ebiten package pass.
- New tests:
  - `TestCarePolicyCapsMedicine`
  - `TestNoCareSkipsTend`
  - `TestNoMedsTendsWithoutMedicine`
  - `TestDoctorPriorityOrder`
  - `TestEmergencyTendWhileDrafted`
  - `TestDoctorDoesNotFlapBetweenPatients`
  - `TestCareSurvivesSave`
  - `TestDefaultCareForPrisoners`
  - `TestSelfTendToggle`
  - UI tests for the care row, the checkbox, the reorder buttons, the Assign column and default-care row, and the drafted Tend option
- `TestCareStubsChangeNothing` is retired; the stubs are now behaviour.
- Golden hashes were re-pinned together with MED-05 (see the MED-05 handoff).
