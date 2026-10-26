# MED-00: medical overhaul contract and save groundwork

This is part of the medical overhaul towards RimWorld's system. The gap
analysis and wiki constants are in `.cursor/MEDICAL_GAP_ANALYSIS.md`. The
ENG-003 wiki-numbers exception now also covers MED-* (AGENTS.md).

## What changed (no simulation behaviour change)
- **Body save format.**
  - `body.Body.Implants [NumParts]` becomes `Fitted []item.Kind`, behind `Implant(p)` / `SetImplant(p, k)` and `FittedFrom`.
  - Saves use `savedBody`, which keeps the legacy `Implants [20]` and reads it only when `Fitted` is nil.
  - `Body` gains `BloodLoss` and `Conds`. `Wound` gains `TendQ`, `TendAt`, `InfectAt` and `Bleed`.
  - All the new fields are zero-safe and unused so far.
- **Drug state.** `Tolerance` and `LastDose` become slices. `savedDrug` keeps the legacy arrays. Accessors: `ToleranceOf`, `LastDoseOf`.
- **Conditions** (`body/condition.go`, `cond_kinds.go`).
  - Types: `CondKind`, `Condition`, `CondDef`, `Stage`, `CapMod(s)`.
  - Registry: a fixed `[NumCondKinds]CondDef` built from per-phase `[]CondEntry` tables listed in `condTables`, with no `init()` and no maps.
  - `Capacity()` and `Pain()` already fold in stage modifiers.
  - To add a kind: append it before `NumCondKinds`, add a `cond_<phase>.go` table, and list that table in `condTables`.
- **Patients index** (`game/patients.go`).
  - A sorted ID set, kept up to date from `bodyChanged`.
  - Healed and dead pawns are cleared by the hourly sweep.
  - Rebuilt on load; not saved.
- **RNG.** New stream `streamMedical`; nothing uses it yet.
- **Commands.**
  - New: `SetMedicalCare`, `SetDefaultCare`, `SetSelfTend`, `EmergencyTend`, `ReorderOperation`.
  - Only reorder is implemented (±1 step). The others are stubs in `care.go`, for MED-03.
- **Operations.** New `OpKind`s: `HarvestOrgan`, `InstallOrgan`, `Euthanize`, `Anesthetize`, `ExciseCarcinoma`, `NumOps`. `CanOperate` rejects them until MED-06. The `cmdAddOperation` bound now uses `NumOps`.
- **Views.**
  - `HealthView`: `State`, `BloodLossPct`, `BleedPerDay`, `DeathInHours` (-1), `Conditions`, `Care`, `SelfTend`.
  - `OperationView`: `ChancePct`, `ChanceTip`.
  - `PawnView`: `Bleeding`, `CareLevel`.
- **Fix.** `value.go` now prices ProstheticFoot 90, GlassEye 40 and WoodenNose 5 (original tuning; these are original items).

## Verify
- `go test -race` for game, the pawn domain, economy and sim: all pass.
- All 6 `TestScaleGolden` pins, `TestStateHashStable` and `TestTemperateGenerationUnchanged` are unchanged. `SaveVersion` stays at 5.
- New tests:
  - `TestLegacyBodySaveKeepsImplants`
  - `TestPatientsIndexMatchesScan`
  - `TestReorderOperation`
  - `TestEveryCondKindHasDef`
  - Implant-accessor and drug legacy conversion tests
- Coverage: body 97.3 %, drug 100 %.

## UI
None yet. The view fields are contract stubs, filled from MED-01 on, where their Health-tab rows ship.
