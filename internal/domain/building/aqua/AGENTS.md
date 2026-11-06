# internal/domain/building/aqua

## Purpose
The pure rules of fish farming in tanks (FISH-*, `.agents/handoffs/FOOD.md#fish_tank_plan` §3): the stocks, how one grows, suffers and dies, and what a harvest yields.

## Owns
- `Species`, `Def`, `DefOf`: the stocks (saved by value, append only).
- `Tank`, `Step`, `Conditions`, `Outcome`, `Cause`: a tank's state and its rules over elapsed ticks.
- `FeedFull`, `FeedBelow`, `FeedItemTicks`: the feed table `game/industry` reads; `NetYield`, `StockWork`, `NetWork`.

## Depends on
- `sim/clock` (constants), `world/temperature`.

## Invariants
- No state of its own.
- Every number is original tuning: nothing in vanilla farms fish.
- The dying warning starts at `WarnBelow` and clears at `WarnClear` (hysteresis).
