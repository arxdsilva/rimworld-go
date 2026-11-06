# Task Handoff

Task ID: SIM-048-social-polish (ENG-003 polish)
Status: complete (race-tested)

## Summary
Social Impact scaling, relation-specific grief, richer romance weights, guard/release animal behaviour, proper letters, face scars drawn, and the save/load Motion fix.

## Behaviour
- Social Impact scales opinion memories: `OpinionMemory.Scale` (permille, clamp 200..1500) comes from the initiator's Social skill + Talking capacity (`social.ImpactScale`). Scale 0 means 1000, so old saves read unchanged.
- `mood.Memory.Label` overrides the thought text for relation-specific grief ("my wife died").
- `InteractCtx` gains Affair and Together: breakup weight uses affairs, proposal weight uses time together.
- Letters: lovers, engagement, rejected proposal, breakup, divorce, affair, social fight, wedding, party, relative among raiders, kidnapped, recruited, bonded animal died.
- Guard / release training (`train_animals.go` masterDuty, 60-tick cadence via `nearestWhere`): a guard defends its master against hostiles within 5 tiles or the attacker of the master in the last 120 ticks; a release-trained animal attacks the drafted master's target.
- Face scar drawn in the HUD portrait and on falling frames.
- `Load` no longer adds a default Motion record (fixes the StateHash mismatch after save/load for pawns that never moved).

## Invariants / Decisions
Numbers follow the published RimWorld wiki by user decision (ENG-003 override). Authoritative state only; no wall clock, no global RNG.

## Verification
New tests in `social_polish_test.go` and the updated social/mood/train tests, all with -race. Goldens re-pinned for Social Impact scaling.
Benchmark TickGrown: about +2-3% (250 colony: 168-174 us, 500: 211-218 us).

## Determinism Impact
Seeded streams only. Golden hashes moved once, intentionally, for the Scale on memories.

## Save / Mod Impact
Additive (Scale field, grief Label); SaveVersion unchanged.

## Known Limitations
None blocking.
