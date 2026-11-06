# MAP-6: the four deferred items, second pass

Asked 2026-11-03: a gap analysis of the items MAP-5 reported as deferred, and fix them. Detail and sources: `.agents/gaps/MULTIMAP_GAP_ANALYSIS.md`, "The four deferred items, second pass"; rows MAP-21 to MAP-23. Done alone, no workers: one lead, in sequence.

## What changed
- Turrets (MAP-21): a settlement's turret is claimable once no defender stands, and `Building.OffGrid` keeps it running. `orders.ReasonDefenders` says why a Claim took nothing. `combat/turret_fuse.go` gives every turret the wiki's fuse. Autocannon and slug turret now burst (5.9 tiles) and can't be moved. A settlement's turret shoots manhunters.
- Wanderers (MAP-22): `whereEscaped` (saved enum, appended after `whereDead`). `everyoneGone` lost its launch special case. `wandererFits` admits a gone colony. The ship's screen opens first and offers New wanderers. The picker's card has a portrait and passion glyphs (`dialogs.Host` gained `DrawPortrait`, `Icon`).
- Visitors (MAP-23): `ArrestRoll` sets goodwill to at most `arrestGoodwill` on reaching a visitor, whatever the roll, and `visit.notice` turns the rest. `MakePrisoner` lost its `arrested` argument. `visit/carry.go` has leavers carry wounded friends home; `Rec.Fetch` and `Rec.NoCarry` hold the no-flap rule. `visit.Host` groups `Turning` and `Carrying` as `Leaving` to stay at 8.

## Decision to review
MAP-13 recorded the user's "the player can't claim them" (2026-11-02). MAP-21 reverses that half because the wiki's Faction base page says they can be claimed once the base is defeated, a line MAP-4 missed. To go back: `orders.Claimable` refuses `b.Hostile && !b.Kind.IsDoor()`, and `enemyCommands` returns no button for a turret.

## Budget
`game` lines stay at the ceiling: `fleeSearch` moved to `combat.FleeCell` to pay for this.

## Tests
`combat/turret_fuse_test.go`, `settlement_claim_test.go`, `visitors_carry_test.go`, `ax8_test.go`, `wanderers_test.go`, `hud/ending_test.go`, `hud/alerts_turned_test.go`, `dialogs/wanderers_test.go`, `inspector/enemy_door_test.go`, `journey_settlement_door_test.go`. No-flap: `TestLeaverGivesUpOnAFriendOutOfReach`, `TestTurretFuseRollsOnce`.

## Gaps
- Raiders don't run from a sparking turret; the fuse has no hiss or particle, only a message and a pane line.
- A turned visitor walks out alone (raid AI), wounded friends stay.
- Old saves: settlement doors held open before MAP-17 stay so; ships launched earlier keep passengers as stashed.
- A legacy colony still has no New wanderers. Group size and arrival gear aren't published.
- Hostile threshold -50 against the wiki's -75, and the wiki's "enemies will arrive in 4 days" after a base falls, are noted in the gap doc and not changed.
