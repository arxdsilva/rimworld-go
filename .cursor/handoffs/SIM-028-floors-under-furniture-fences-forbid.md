# SIM-028 Floors under furniture, fences and gates, forbid

## What changed
- **Floors under furniture:** floors may go under any furniture (bed, benches...), but not on rock, walls, fences/gates or dug graves. Builders work from an adjacent tile.
- **Fences and gates:** wood and stone fences (autotiled) and fence gates (4 facings).
  - Animals cannot cross fences; tame animals pass gates, wild ones cannot.
  - Colonists and raiders climb fences at x3 step cost.
  - Animals never pick food behind a fence.
  - Fences cast no shadow.
- **Forbid:** `Stack.Forbidden`. Haul, eat, equip, bills, refuel, troughs and burial skip forbidden stacks. Raid weapons and bodies drop forbidden. Forbid/Allow area tools, an item panel toggle, a red marker on the map, and the F key (UI-016).

## Verification
- Tests for floor placement rules, fence/gate passability by species and tameness, climb cost, and forbidden skipping in each giver.
- Gates: `go build ./...`, `go vet ./...`, `go test ./...`.

## Known limitations
- Pathing does not prefer gates over climbing fences.
- A forced Prioritize on a forbidden item is allowed.
- Stacks forbidden mid-haul are not interrupted.
- No stone gate art.
