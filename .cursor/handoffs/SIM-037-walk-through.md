# Task Handoff

Task ID: SIM-037 (PLAY-010)
Status: complete

## Summary
Colonists and tame animals no longer block each other: a step into a cell held only by such "partners" just goes. Wild animals and raiders still block, as movers and occupants. In combat (drafted, attacking or fleeing, `inCombat` in combat_state.go) a colonist never ends a walk on another colonist's tile; it stops on the nearest free one. Outside combat, colonists may share a tile; the unstack sweep only splits pairs when one is in combat.

## Files
`occupancy.go`: partners have their own layer (`pfirst`, `pcount`, `pnear`, `pextra`) beside the blocker cells; `enter` uses `holderFor`/`combatHolder`/`stopOK`; `crowdCost` doesn't price partners for a partner mover.

## Tests
New `walkthrough_test.go` (11 tests). Tests that asserted colonist-colonist blocking now assert the new rule (blocking checks moved to wild animals): see the list in the commit. `postpone_test.go` uses a held wild animal as the doorway blocker.

## Notes
Supersedes the colonist part of SIM-014's "every standing creature owns its cell".
