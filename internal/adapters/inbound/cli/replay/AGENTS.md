# internal/adapters/inbound/cli/replay

## Purpose
Behind `cmd/replay`: `replay verify <file>` decodes a recorded run and re-runs it on a fresh colony, checking every checkpoint hash.

## Owns
`run.go`: `Main` (exit 0 ok or help, 1 hash mismatch, 2 usage error, unreadable or invalid file) and the colony starter.

## Depends on
`application/game`, `application/replay`. Never `cli/simulate`.
