# aqua package

## Package Purpose
The pure rules of fish farming in tanks (FISH-*, `.cursor/FISH_TANK_PLAN.md` §3): the stocks, how one grows, suffers and dies, and what a harvest yields.

## Owned Concepts
- `Species`, `Def`, `DefOf`: the stocks (saved by value, append only).
- `Tank`, `Step`, `Conditions`, `Outcome`, `Cause`: a tank's state and its rules over elapsed ticks.
- `FeedFull`, `FeedBelow`, `FeedItemTicks`: the feed table `game/industry` reads; `NetYield`, `StockWork`, `NetWork`.

## Allowed Dependencies
- `sim/clock` (constants), `world/temperature`. Nothing from application or adapters.

## Invariants
- No RNG, no clock reads, no state of its own.
- Every number is original tuning: nothing in vanilla farms fish.
- The dying warning starts at `WarnBelow` and clears at `WarnClear` (hysteresis).
