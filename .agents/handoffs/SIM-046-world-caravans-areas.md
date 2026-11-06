# Task Handoff

Task ID: SIM-046-world-caravans-areas (ENG-003 M9a)
Status: complete (race-tested, committed 8a84be08)

## Summary
x

## Behaviour
Runtime world, caravans, visitors, wanderers and allowed areas.

## Invariants / Decisions
Numbers follow the published RimWorld wiki by explicit user decision (ENG-003 balance override). All mutations enter as typed commands; the UI reads Snapshot() read models only.

## Verification
Tests: new unit and no-flap tests for this milestone, run with -race.
Benchmarks: none; work is event-driven and indexed, no per-tick global scans.

## Determinism Impact
Seeded RNG only, no wall clock. Known pre-existing issue: a pawn that never moved gains a default Motion record on load, so StateHash differs after save/load for such pawns (tests work around it).

## Screen / UI Impact
Not done: caravan animals, cargo and trade; area overlay and delete; transport pods. Kidnapping and raider relatives are in progress (M9b).

## Known Limitations
- Planet rebuilt at runtime with a route; caravans: form dialog, walk to map edge, travel at slowest Moving speed, visit a friendly settlement 1 day, return; food days; in-caravan chitchat and deep talk; death on the road with grief.
- World tab (F8) with globe and caravan list.
- Visitor groups from friendly/neutral factions wander half a day, socialise and leave; wanderers join.
- Allowed areas (up to 10): paint, rename, assign via Assign tab and area manager; pathing and job filtering respect them, drafted pawns ignore them.

## Follow-ups
See ENG-003-social-contract.md progress.
