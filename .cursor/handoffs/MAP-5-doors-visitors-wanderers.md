# MAP-5: enemy doors, hostile visitors, new wanderers

Closes the MAP-4 leftovers the vanilla way (asked 2026-11-02). Gap rows MAP-17 to MAP-20 in `.cursor/MULTIMAP_GAP_ANALYSIS.md`; plan `.cursor/FISH_TANK_PLAN.md`.

## What changed
- Doors (MAP-17, wiki: Door): `mapsSettle.Settle` lays doors shut with `Building.Hostile`. `Doors.Shut` lets defenders through; colonists and all animals are shut out by `colonistGrid` and the region `Enemy` flag. A drafted Attack order breaks a door. `orders.Claimable` takes a hostile door once `Raiders.Defending()==0`; turrets stay refused. Defenders inside go out when the compound is hit, held by `defendHold` so they can't flap.
- Visitors (MAP-18, game behaviour): `visit/turn.go` makes visitors, traders and allies raiders already leaving, each in its own `Over` group, so no goodwill is lost. They shoot back. Trade closes. A guest in bed or being rescued stays until recovered, then leaves hostile. Downed ones stay down.
- Wanderers (MAP-19, wiki: Endings): `command.NewWanderers` (seeds in `Name`), 8 in-game hours, 7 groups. `AX.WanderersIn/Left/OK/Max/Rules`, `game.WandererPawn`, `game.WandererBio`. They walk in on map 0; an abandoned colony's map stays, wiped. The group's size limit is the scenario's starting people.
- Spent turrets (MAP-20): checked against the wiki, behaviour kept, inspector says "Out of ammo".

## Tests
`settlement_door_test.go`, `drop_rules_test.go`, `TestSpentSettlementTurretStaysSilent`, `visitors_hostile_test.go`, `wanderers_test.go`, `journey_settlement_door_test.go`, `journey_wanderers_test.go`.

## Gaps
- Old saves keep their held-open settlement doors.
- A legacy colony (no setup) gets no new-wanderers option.
- ~~No journey claims a door after a defeat in the real loop~~ closed by MAP-6 (`TestJourneyClaimAfterTheDefeat`).
- No enemy mark on a door: it shows the usual sprite.
- A recovering guest of a turned faction still earns the usual +12 goodwill.
- A caravan has no pack animals, so a turned caravan has none to take along.
- The wanderer picker is a `dialogs` modal, not the wizard page (`dialogs` can't import `wizard`). MAP-6 gave it the portrait and passion glyphs.
- The Visitors page was unreachable, so MAP-18 rests on game behaviour, not a published page. MAP-6 found the arrest and carrying rules on Events, Factions and Visitor; the turn itself still has no page.
