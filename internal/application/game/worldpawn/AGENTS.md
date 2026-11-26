# internal/application/game/worldpawn

## Purpose
Owns off-map non-caravan human pawn health, needs, care, and transient mothball cadence.

## Contracts
The native game selects world-stashed pawns in ascending ID order and executes the world pass once per colony. Caravan medical, animals and stasis sleep have separate owners.

## Invariants
The package tests cover tending, needs, return-from-stash, mothball boundary and caravan exclusion. Mothball state is derived and never serialized.
