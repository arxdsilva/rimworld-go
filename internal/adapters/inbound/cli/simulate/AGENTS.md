# internal/adapters/inbound/cli/simulate

## Purpose
Behind `cmd/simulate`: runs a headless colony for `-ticks` ticks from `-seed` on a `-width` x `-height` map and prints the tick, stock and each pawn, then a line `hash=0x<16 hex>` (the final `StateHash`, ENG-010; same seed and ticks give the same line). A smoke and determinism check. `-record <path>` (with `-every <n>`, default 100) also writes a replay file through `application/replay` and prints a `replay <path> checkpoints=<n> hash=<h>` line after it.

## Owns
`run.go`: `Main` (flags, exit codes: 0 ok or help, 2 bad flag or negative `-every`, 1 replay write error) and the printer.

## Depends on
`application/game`, `application/replay`.
