# Task Handoff

Task ID: PLAY-001
Status: complete

## Summary

Pawns have display names, follow deterministic 8-connected A* on `grid.Grid`, and run a single job FSM (Goto/Do/Complete/Fail) with entity+tile reservations. Tick phases are commands → needs → AI/jobs → movement → clock. `MovePawn` is the first typed gameplay command. Inspector and colonist bar show name + job line.

## Files Changed

- `internal/domain/ai/pathing/`
- `internal/domain/ai/job/`
- `internal/domain/ai/reservation/`
- `internal/domain/sim/command/command.go`
- `internal/application/game/game.go`
- `internal/application/game/tick.go`
- `internal/application/game/play_test.go`
- `internal/adapters/inbound/ebiten/{inspector,hud,app,renderer}.go`

## Public Contracts Changed

`command.MovePawn`. `PawnView.Name` and `PawnView.Job`. `Game.Enqueue`/`Apply`. `Simulation` includes `Enqueue`.

## Invariants / Decisions

- One sim owner; insertion-order pawns; no goroutine-per-pawn.
- A* Chebyshev, diagonal only if both orthogonal neighbors walkable; tie-break f, g, y, x.
- Random `stepOnce` walks removed; idle pawns stand unless AI assigns work.
- UI never mutates stores; right-click selected pawn enqueues `MovePawn`.

## Verification

`go test ./internal/application/game/` (`TestMovePawnFollowsPath`, `TestMovePawnBlockedIsUnreachable`)
`go test ./internal/domain/ai/pathing/ ./internal/domain/ai/job/ ./internal/domain/ai/reservation/`
`go test ./internal/adapters/inbound/ebiten/`

## Determinism Impact

Pathing and job assignment are seed- and insertion-order deterministic. No wall-clock RNG.

## Save / Mod Impact

None (no persistence).

## Art / Visual Impact

No new sprites. Vendor pawn portraits + inspector text.

## Screen / UI Impact

Inspector Character body: name, job line, tile. Colonist bar uses display names.

## Known Limitations

- No region graph, congestion, or pawn-pawn occupancy.
- Job kinds beyond this wave landed in later PLAY/SURV slices in the same change set.

## Follow-ups

PLAY-002 hunger/rest/items (done in this roadmap).
