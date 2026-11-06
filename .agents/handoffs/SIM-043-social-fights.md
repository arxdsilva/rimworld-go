# Task Handoff

Task ID: SIM-043-social-fights (ENG-003 M6)
Status: complete (race-tested, committed 8a84be08)

## Summary
x

## Behaviour
Social fights escalating from negative interactions.

## Invariants / Decisions
Numbers follow the published RimWorld wiki by explicit user decision (ENG-003 balance override). All mutations enter as typed commands; the UI reads Snapshot() read models only.

## Verification
Tests: new unit and no-flap tests for this milestone, run with -race.
Benchmarks: none; work is event-driven and indexed, no per-tick global scans.

## Determinism Impact
Seeded RNG only, no wall clock. Known pre-existing issue: a pawn that never moved gains a default Motion record on load, so StateHash differs after save/load for such pawns (tests work around it).

## Screen / UI Impact
Not done: insulting-spree mental break (10.1 destructive breaks stays open).

## Known Limitations
- Chance from insult 4% / slight 0.5%, multiplied by bloodthirsty, hunger, alcohol and withdrawal factors.
- Uncontrolled fists-only brawl lasting at least 420 ticks; non-violent pawns do not strike. Ends on down, death or recovery.
- Cathartic +38 / angering -22 opinion per participant, rolled independently; harmed-me memory.
- Drafting blocked during a fight; pair cooldown prevents repeat fights.

## Follow-ups
See ENG-003-social-contract.md progress.
