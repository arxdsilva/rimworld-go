# Task Handoff
Task ID: SIM-049
Status: complete

## Summary
Operating on colonists is now usable in normal play. Before this, surgery (SIM-039) was fully wired, but it only fitted implants to parts that were already missing. Queued operations that couldn't start also gave no feedback. Now:
- **Replacing healthy parts.** Implants can replace healthy parts, as in RimWorld. The operation removes the part (and the parts under it), then fits the implant.
- **New implants:** prosthetic foot (750‰), glass eye and wooden nose. The glass eye and wooden nose are cosmetic: they count as artificial and clear the missing-part thought, but disfigurement still follows the spec.
- **Health tab:** the picker lists every valid operation. Unavailable ones are greyed with the reason, and every queued operation shows what it is waiting for.

## Files Changed
- **Domain:**
  - `internal/domain/pawn/body/{implant,capacity,surgery}.go`: `CanReplace`, `InstallOptions`, `MissingUncovered`, healthy-part replacement in `Apply`, a missing-ancestor block (no wooden hand under a missing arm), the new implants.
  - New items, recipes and research unlock in `item.go`, `recipe.go` and `research.go`. The glass eye needs Prosthetics research and Crafting 4.
- **Game:**
  - `surgery.go`: `surgeryWait`/`opBlocked` reasons, which also stop patients lying down for an implant nobody can reach.
  - `infection.go`/`tick.go`: `lieDownForCare` prefers a free hospital bed.
  - `body.go`: `HealthView.Options`, `OperationView.Status`/`Blocked`, `PartView.Part`/`Implant`.
  - `dev_social.go`: `DevLosePart`, `DevSpawnImplants`.
- **UI:**
  - `health_ui.go`: the picker from `Options`, greyed reasons, status lines.
  - `building_inspector.go`: an "+ Add bill" pick list replaces the per-recipe buttons, which overflowed the panel and sat under Move/Copy.
  - `devmode.go`: "Lose a part…" and "Spawn implants".
- **Art:** `item/{prosthetic_foot,glass_eye,wooden_nose}` (`item.py`).

## Public Contracts Changed
- `game.OperationOptionView` and `HealthView.Options`.
- `OperationView.Status` and `OperationView.Blocked`.
- `PartView.Part` and `PartView.Implant`.
- Dev hooks `Game.DevLosePart` and `Game.DevSpawnImplants`, plus the matching `devTools` methods.

## Invariants / Decisions
- **Waiting reasons, in order:**
  1. "needs research" (only when none is in stock)
  2. "no X in stock"
  3. "the X can't be reached"
  4. "needs another colonist who can doctor"
  5. "waiting for the patient to lie down"
  6. "a doctor is on the way"
- `Blocked` is true for 1–4 only.
- **No flap.** A patient lies down only when its operation can start, and the 6h give-up is kept.
- **Numbers.** The implant numbers are our own tuning; the spec gives none.

## Verification
Tests:
- `go test -race ./...` is green.
- Goldens and determinism hold over `-count=3`, with and without race, and no golden moved.
- `make art-check` reports 0 errors.

End-to-end, in the social scenario: after implants are spawned, 16 replace options appear on a healthy colonist. "Replace left leg with prosthetic leg" shows "a doctor is on the way", a second colonist performs it, and Moving drops to 92%.

Benchmarks: not run, because the surgery paths only run when operations are queued.

## Determinism Impact
None for colonies without operations. The goldens are unchanged.

## Save / Mod Impact
- `surgerySave.Hosp` (the hospital bed a surgery patient took) is additive.
- The new item and recipe kinds are append-only.

## Art / Visual Impact
Three item sprites. The workbench now adds bills through a pick list.

## Screen / UI Impact
Health tab §14 picker and status lines, the workbench bill picker, and two dev menu tools.

## Known Limitations
- No bionics.
- No organ transplants.
- No surgery risk preview.
- No room-cleanliness factor.
- A bench with more than about 7 bills can still crowd the Add bill button.

## Follow-ups
- Bionics tier.
- Risk preview in the picker.
