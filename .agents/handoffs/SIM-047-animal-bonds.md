# Task Handoff

Task ID: SIM-047-animal-bonds (ENG-003 M10)
Status: complete (race-tested, committed 8a84be08)

## Summary
x

## Behaviour
Animal bonds, master assignment, training and veterinary care.

## Invariants / Decisions
Numbers follow the published RimWorld wiki by explicit user decision (ENG-003 balance override). All mutations enter as typed commands; the UI reads Snapshot() read models only.

## Verification
Tests: new unit and no-flap tests for this milestone, run with -race.
Benchmarks: none; work is event-driven and indexed, no per-tick global scans.

## Determinism Impact
Seeded RNG only, no wall clock. Known pre-existing issue: a pawn that never moved gains a default Motion record on load, so StateHash differs after save/load for such pawns (tests work around it).

## Screen / UI Impact
Not done: guard and release training behaviour.

## Known Limitations
- Bond chances: tame 1%, training 0.7%, tending 0.4%, proximity 0.1% per 2500 ticks.
- Training obedience, guard, release with steps, cooldown and decay; obedience makes the animal follow a drafted master (guard and release have no behaviour yet).
- Veterinary tending and natural healing for tame animals.
- Bond moods: +5 master / -3 not master; died/lost -8, given away -10, released -5; cold-hearted exempt. Master death: animal confused or 10% manhunter. Mother/offspring shown.

## Follow-ups
See ENG-003-social-contract.md progress.
