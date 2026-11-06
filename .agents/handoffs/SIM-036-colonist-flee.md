# Task Handoff

Task ID: SIM-036 (PLAY-010)
Status: complete

## Summary
An undrafted colonist with no weapon (or unable to do violence) flees from any animal coming for it (hunting, charging, manhunting or retaliating) as soon as it is within 12 tiles, keeping the threat until 16 (hysteresis). The check runs right after mental breaks and before queued orders, so it also interrupts orders, work and sleep; an interrupted forced order goes back to the front of the queue. Drafted colonists stay under the player's control; armed colonists keep the old 4/8-tile reach and fight back.

## Files
- `combat.go` `defend` (reach from `threatReach`), new `flee.go` (`fightsBack`, `threatReach`, `fleeGoal` trying straight away then ±45° then ±90°, each checked for reachability and preferring the home area; `dropForFlight`; `fillFleeView`).
- `tick.go` per-pawn loop: unarmed flee before orders.
- `game.go` snapshot calls `fillFleeView`: `PawnView.Fleeing` and the job line "Fleeing from <creature>".

## Tests
`flee_test.go` (13 tests incl. a no-flap at the 12/16 boundary); the older flee/calm tests stay green.
