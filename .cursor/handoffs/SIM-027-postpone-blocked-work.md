# SIM-027 Postponing blocked and unreachable work

## What changed
- **Postponement** (`application/game/postpone.go`): work that is blocked (15 s) or unreachable (1 h; cleared when walkability changes, tracked by `grid.Version`) is put off per pawn + job + target.
- **Givers** try up to 8 nearest candidates and skip postponed ones.
- **Blocked movement:** a pawn stuck behind a blocker that is still not yielding gives up after 2 s (120 ticks) instead of 10 s. Working pawns whose job is on an adjacent cell shuffle aside.
- **Player orders:** a blocked order is retried up to 3 times, then dropped with a message; an unreachable order is dropped with a message.
- **UI:** the colonist inspector shows "put off: ...". Postponement state is saved.

## Verification
- Tests in the game and sim packages for postpone expiry, walkability reset, giver skipping and order dropping.
- Gates: `go build ./...`, `go vet ./...`, `go test ./...`.

## Known limitations
- None known.
