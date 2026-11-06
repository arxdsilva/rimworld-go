# Task Handoff

Task ID: SIM-045-prisoners (ENG-003 M8)
Status: complete (race-tested, committed 8a84be08)

## Summary
x

## Behaviour
Prisoner capture, life, warden work, recruitment, release and execution.

## Invariants / Decisions
Numbers follow the published RimWorld wiki by explicit user decision (ENG-003 balance override). All mutations enter as typed commands; the UI reads Snapshot() read models only.

## Verification
Tests: new unit and no-flap tests for this milestone, run with -race.
Benchmarks: none; work is event-driven and indexed, no per-tick global scans.

## Determinism Impact
Seeded RNG only, no wall clock. Known pre-existing issue: a pawn that never moved gains a default Motion record on load, so StateHash differs after save/load for such pawns (tests work around it).

## Screen / UI Impact
Not done: prison break, warden chat history list.

## Known Limitations
- Downed raiders are captured and carried to prisoner beds; arrest has a success chance, failure triggers a resist fight.
- Prisoner life: needs, Imprisoned mood, room-bound. Doctors tend prisoners in place.
- Warden work type delivers food (hysteresis) and visits that reduce resistance by opinion and mood factors x social impact.
- Recruit keeps relations (recruited-me +20 for 120 days); release +15 goodwill; execute applies relatives' killer opinion.

## Follow-ups
See ENG-003-social-contract.md progress.
