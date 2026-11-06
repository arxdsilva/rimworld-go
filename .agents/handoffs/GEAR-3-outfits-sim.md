# GEAR-3: apparel policies (simulation side)

Auto-dress now follows each colonist's apparel policy (OUT-001…005, `GEAR_GAP_ANALYSIS.md`). The state hash is unchanged while policies stay at the defaults: `bin/benchmark -hash -sizes 120,250 -speeds 1`, with fixtures both cached and rebuilt, gives grown 0x9d3f4fd296e4fc7d / 0xd7bfd4d81453422b and raid 0xfcf651463f76a937 / 0xa2d46e4ac13ff189, the same before and after.

## What changed
- **`game/outfit`** (new). `dress.go`, `index.go` and `deps.go` hold the dressing that used to live in `apparel.go`, moved first in a commit of its own with identical hashes. It works through four consumer-owned roles (`Wardrobe`, `Stock`, `Weather`, `Walks`). `policy.go` holds the policy `Book`, shaped like `diet.Book`: a `zone.Filter` of allowed kinds, a `zone.Filter` of refused materials, and a `zone.Special` for the HP range, quality range and tainted toggle. `view.go` fills `gearview.Policy` and `gearview.Pawn`.
- **Defaults** (our own contents):
  - Anything: every garment, never tainted. It is the default.
  - Worker: no armour or helmets, HP 50–100 %.
  - Soldier: everything, HP 50–100 %. Dressing still scores warmth only, so Soldier doesn't actively prefer armour. A preference would need `craft.Best` scoring.
  - Nudist: utility only (shield belt, smokepop).
  - Kinds outside the Apparel branch are never a policy's to forbid.
- **Dressing.** A candidate must pass `Policy.Takes`, which replaces the old unconditional tainted skip. The optimistic plan filters by kind. If the policy refuses a worn garment, `shed` takes off the first one per check, in an unlocked slot only, so forced slots and slots in their 2 h `Cool` are never touched. The slot then cools like any other change.
- **Commands** (`tick.go` → `applyOutfit` in `apparel.go`):
  - `SetApparelPolicy`: colonists only.
  - `EditApparelPolicy`: every `ApparelPolicy*` mode.
  - `ClearForcedApparel`: clears every forced bit on the pawn.
  - Deleting the default, or the last policy, is refused. A deleted policy's holders go back to the default.
- **Default for new colonists.** No join hook. A colonist without an assignment follows the current default. `ApparelPolicyDefault` first pins everyone dressed who is neither a raider nor a guest (keys of `g.apparel`, so pawns on other maps and on caravans are included) to the old default. Only colonists who join after the change get the new one. The book stays empty (nil `Export`) until the player edits it.
- **State.** `worldState.outfits outfit.Book` (`world_state.go`) is read through `outfitBook(g)`.
- **Save v14.** `saveState.Outfits *outfit.Save` holds the book, the default and the assignments, and is nil at the defaults. Older saves import nil, which gives the default book with everyone on Anything. The `case v < 14` migration step is comment only. `testdata/save_schema.txt` and `TestSupportedSaveVersions` are updated.
- **Views.** `PawnView.Gear.ApparelPolicy` and `ApparelPolicyName`, and `WorldView.Gear.Policies`, list allowed kinds and allowed materials by name.

## Budget
Root package: −223 lines (`apparel.go` 447 → 212; `save.go` +8, `tick.go` +2, `world_state.go` +2), and 10 fewer `Game` methods (`dressPlan`, `wearCandidates`, `dressLocked`, `dressUrgent`, `dressCooling`, `undress`, `dress`, `apparelIdx`, `apparelStock`, `apparelKinds`). No new files or fields on `Game`. The tests reach the moved functions through `outfit_shim_test.go`, which doesn't count against the budget.

## Tests
- `outfit/policy_test.go` covers the filters, edits, delete and reassign, the default for new colonists, and export/import.
- `outfit_policy_test.go` (root) covers:
  - one slot shed per check;
  - forced garments staying until cleared;
  - candidates kept to the policy;
  - no flap when the policy changes every 30 min (no slot changes twice within `dressCool`);
  - a v14 save round trip;
  - v12 fixture migration.
- `go test -race ./internal/application/game -run 'Dress|Apparel|Save|Wear|Outfit|Supported'`, `./internal/application/game/outfit`, `./internal/application/scenario -run Golden` and `./internal/arch` all pass.

## Gaps
- The Assign column, the policy editor and Clear forced in the Assign row (Wave 2) are not built. The views are filled for them.
- Soldier's armour preference is not modelled (see Defaults).
- The book is never pruned of pawns that died or left. Their entries are inert, and the export is sorted.
