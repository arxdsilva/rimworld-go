# Task Handoff

Task ID: SIM-042-romance-weddings (ENG-003 M5)
Status: complete (race-tested, committed 8a84be08)

## Summary
x

## Behaviour
Romance, weddings, affairs, breakups and partner beds on top of the SIM-041 social core.

## Invariants / Decisions
Numbers follow the published RimWorld wiki by explicit user decision (ENG-003 balance override). All mutations enter as typed commands; the UI reads Snapshot() read models only.

## Verification
Tests: new unit and no-flap tests for this milestone, run with -race.
Benchmarks: none; work is event-driven and indexed, no per-tick global scans.

## Determinism Impact
Seeded RNG only, no wall clock. Known pre-existing issue: a pawn that never moved gains a default Motion record on load, so StateHash differs after save/load for such pawns (tests work around it).

## Screen / UI Impact
Not done: manual Romance button (Biotech, excluded by design).

## Known Limitations
- Romance attempts chosen by attraction (orientation traits, age >= 16, no blood kin) produce lovers; rebuffs leave memories and moods.
- Affairs: cheated memory -70, mood -20. Breakup: ex-lover memory -50 / mood -15. Divorce: ex-spouse -70 / mood -20.
- Proposals lead to engaged, or rejection (-30 opinion, mood -18, 10-day cooldown).
- Weddings at the marriage spot (fallback party spot, campfire, home): gathering, ceremony, reception. Save-safe, one marriage across save/load. +40 mood married, +20 attendees, surname adoption 50%.
- Parties at party spot or campfire (+8).
- Partner beds (double bed), sleeping alone -4, partner-opinion mood +-10, lovin' +8 (no pregnancy).
- Start/stop use different thresholds and cooldowns so pairs do not flap.

## Follow-ups
See ENG-003-social-contract.md progress.
