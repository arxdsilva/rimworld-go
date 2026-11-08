# internal/adapters/inbound/cli/simulate

## Purpose
Behind `cmd/simulate`: runs a headless colony for `-ticks` ticks from `-seed` on a `-width` x `-height` map and prints the tick, stock and each pawn. A smoke and determinism check.

## Owns
`run.go`: `Main` (flags, exit codes: 0 ok or help, 2 bad flag) and the printer.

## Depends on
`application/game`.
