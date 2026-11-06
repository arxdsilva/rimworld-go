# Task Handoff

Task ID: ART-002
Status: complete

## Summary

Simulation no longer steps 1:1 with render frames. Normal/fast/very-fast now wait 15/6/2 Update frames per tick (~4/10/30 TPS at 60 Hz). Idle and walk frames advance from a presentation frame counter, and walk holds for 20 frames after a tile hop so cycles stay readable.

## Files Changed

- `internal/adapters/inbound/ebiten/renderer.go`
- `internal/adapters/inbound/ebiten/renderer_test.go`
- `internal/adapters/inbound/ebiten/sprites.go`
- `.cursor/RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md`
- `.cursor/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md`
- `.cursor/ART_DIRECTION_AND_ASSET_PIPELINE.md`

## Public Contracts Changed

None. Speed keys (`Space`, `1`/`2`/`3`) are unchanged.

## Invariants / Decisions

- Animation clock lives only in the ebiten adapter. Domain tick rules and RNG are unchanged.
- Game speed still changes the rate of processed ticks, never simulation rules.
- Walk vs idle is inferred from last snapshot position; walk persists for `walkSustainFrames` after a hop.

## Verification

Tests: `go test ./internal/adapters/inbound/ebiten/` (needs a non-sandboxed run because Ebitengine initializes GLFW).

In-game window was not visually QA'd in this session. Run `go run ./cmd/game` and watch idle/walk at speed 1.

## Determinism Impact

None. Headless `Step(n)` is unchanged; only the interactive renderer paces when it calls `Step(1)`.

## Save / Mod Impact

None.

## Art / Visual Impact

Pawn cycles are watchable at normal speed. Side-view vendor pack is still a stand-in.

## Screen / UI Impact

HUD still debug text. Selected-pawn portrait now plays idle on the presentation clock.

## Known Limitations

- Pawns still hop a full tile per sim tick (no interpolation).
- Very-fast remains jumpy because hops are frequent.

## Follow-ups

- Interpolate pawn draw position between ticks when movement duration exists.
- Domain movement that takes multiple ticks per tile.
