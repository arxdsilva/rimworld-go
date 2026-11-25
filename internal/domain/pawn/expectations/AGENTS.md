# internal/domain/pawn/expectations

## Purpose
SIM-071 owns the shared wealth bands for mood and recreation. AX-6 provides the wealth thresholds and tolerance rates; the mood definition source tags document the expectation bonuses.

## Owns
Stable Band ordinals, inclusive wealth thresholds and Effects (mood bonus and recreation tolerance decay). ForWealth is the sole classifier; consumers use Effects instead of duplicating thresholds.

## Contracts
Wealth below 15,000 is ExtremelyLow; subsequent thresholds enter the next band. Effects is a pure lookup with a neutral result for invalid bands. Preserve serialized IDs and update both consumers' tests if the table changes. No mutable tracker or per-pawn state lives here.

## Tests
expectations_test.go verifies boundaries and effects. Mood/recreation domain tests and root thought/wealth tests cover consumers.
