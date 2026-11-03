# STK-AL 2: what drops on the map

Fixes the glitch reported 2026-11-02: an animal's kill was allowed by default, a colonist went for it, and the predator had already eaten it. Rows F6 and F16 to F22 in `.cursor/ALLOW_GAP_ANALYSIS.md`.

## Root cause
Three faults stacked:
- `killTaken` counted any animal killer, so a wild kill stayed allowed outside Home.
- A wild feeder held no claim. `leftFound` only filters hauling, so butcher and cremate bills, burial, food search and orders still took an Unclaimed carcass.
- `deleteItem` released claims but left walkers in Goto, so they walked to the empty spot.

## What changed
`killTaken` covers a colonist hunting that prey or a tame animal on its hunt. Wild feeders claim their carcass; pack mates share it and a colonist's existing claim stands. `stackGone` (`forbid.go`, `haul.TargetGone`) ends walks at once. Wild eggs, spare weapons, shoved stacks, settlement goods and siege meals are fixed too; settlement goods can still be packed.

## Tests
`allow_drops_test.go`, `drop_rules_test.go`, `haul/step_test.go`.

## Gaps
- F22: scattered scenario goods stay allowed (unconfirmed vanilla rule, would move a golden).
- Wild raw-food feeding holds no claim; a walker stops at once through `stackGone` when the food goes.
- The wiki's Forbidden page is missing, so these rows rest on game behaviour.
