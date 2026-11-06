# GEAR-3: apparel policies (Assign window)

UI side of OUT-001…005. Reads `WorldView.Gear.Policies` and `PawnView.Gear.ApparelPolicy`; sends `SetApparelPolicy`, `ClearForcedApparel` and `EditApparelPolicy`. No simulation change, no hash impact.

## What changed (`ebiten/windows`)
- **Apparel column** (`assign.go`, `apparel_policy.go`): a click cycles the policy, and dragging down the column paints it (`paintOutfit`). Columns are now laid out by `newAssignCols` from the body width, shrinking together to half; the Food column keeps its fixed width. Name/Drugs/Medical got narrower to make room. Food policies and Manage areas moved to the default-care row.
- **Clear forced**: a button per row, disabled with a reason when `ApparelForced` has no true entry.
- **Editor** (`apparel_policy_editor.go`): opened from the Apparel header. Policy list with New, Copy and Delete (disabled for the default and the last policy, with a reason), rename field, "default for new colonists" check (disabled on the default), tree of Apparel kinds from the filter tree plus a materials list, HP and quality low/high sliders, tainted check. The tree is built in these files; the inspector/architect ones were left alone.
- Range sliders keep a draft and send one command when the knob is released; a sent value is held 90 frames so the view catching up does not snap it back.
- `deps.go`: `outfit *outfitState` on `Set`. No root `windows_compat.go` change was needed.

## Tests
- `windows/apparel_policy_test.go`: cycle, drag paint, clear forced, editor commands, disabled reasons, tree rows, fit at UI scales 75–200 %.
- `journey_outfit_test.go`: one journey per command kind and mode (SetApparelPolicy, ClearForcedApparel, EditApparelPolicy: create, copy, rename, delete, default, allow, stuff, tainted, HP, quality).

## Gaps
- `TestEveryInputActionHasAJourney` still fails on ConsumeFromPack, DropGear and SetApparelForced (GEAR-1 pack/inspector work, not this change).
