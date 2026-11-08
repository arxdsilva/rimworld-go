# internal/platform/logfile

## Purpose
The run's log file (OPT-003), moved out of `cmd/game` (2026-11, GAR): `<user config dir>/rimworld-go/logs/game.log`, with the previous run's log kept as `game-prev.log`. Options › General › Open log folder shows the folder.

## Owns
`Open(dir)`, `Dir()`, and `Start()`, which tees the standard logger to stderr and the file and returns the closer.

## Contracts
Each run rotates the last `game.log` to `game-prev.log` and starts a fresh one (`TestLogFileKeepsTheLastRunAsPrev`). A missing config folder or an unwritable file never stops the game: it logs to stderr only.
