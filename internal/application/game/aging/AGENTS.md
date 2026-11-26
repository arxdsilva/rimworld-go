# internal/application/game/aging

## Purpose
Owns deterministic MED-08 chronic progression and onset for native pawns, including world pawns.

## Contracts
StepIDs operates on only the passed authoritative pawn IDs; the caller selects map patients or off-map pawns. Step retains the original hourly/daily cadence and stream draws.

## Invariants
Source-compatible old-save, replay, and MED-14 chronic tests retain their deterministic results.
