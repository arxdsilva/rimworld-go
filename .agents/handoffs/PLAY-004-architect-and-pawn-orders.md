# Task Handoff

Task ID: PLAY-004
Status: complete

## Summary

Working build menu and pawn actions. The simulation gained a buildable
catalog (wall, door, floor, bed, table, chair, lamp) with costs, work,
layers (floor under object), movement blocking and rotation; one
placement rule (`CanPlace`) shared by the preview and the sim;
deconstruction with half refund; drafting; and player-prioritized jobs
plus an `OrdersAt` read query that lists valid actions at a tile with
reasons. The ebiten client got an Architect panel with categories, icon
cards, tooltips, green/red sprite ghosts, line/fill/single/area
placement and Q/E rotation, a Draft button (R) with bar/world badges, and
a right-click order menu. Blueprints render as translucent ghosts of the
planned object; doors open when occupied.

## Files Changed

- `internal/domain/building/construction/construction.go` (+ test): defs, layers, facing, deconstruct state
- `internal/domain/sim/command/command.go`: `DesignateBuild`, `DesignateDeconstruct`, `DraftPawn`, `Prioritize`, `Order`
- `internal/domain/ai/job/job.go`: `Deconstruct`, `Record.Dest`, `Record.Forced`
- `internal/application/game/{placement,orders}.go` (new), `tick.go`, `game.go`, `orders_test.go` (new)
- `internal/adapters/inbound/ebiten/{architect,archtools,orders}.go`, `draw_art.go`, `app.go`, `hud.go`, `inspector.go`, `renderer.go`, `art.go`, `capture.go`, tests, `AGENTS.md`
- `cmd/game/main.go` (`-capture-ui`), `Makefile`
- `assets/source/blender/artlib/families/{furniture,overlay}.py` → cot facings, card/order/draft icons, deconstruct marker; regenerated assets
- `.cursor/RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md`, `.cursor/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md`

## Public Contracts Changed

- Commands: new kinds above; `Command` gains `Build`, `Facing`, `Order`, `Layer`. Old `DesignateWall/Bed` still work.
- Read model: `PawnView.Drafted`; `FrameView.Facing/Floor`; `BuildingView.Facing/Floor/Deconstruct`; `game.Buildables()`, `Game.CanPlace`, `Game.OrdersAt`, `OrderOption`, reason constants.
- `ebiten.Simulation` adds `CanPlace` and `OrdersAt`.

## Invariants / Decisions

- Placement and order validity live in the application layer; the UI only displays them.
- One floor + one object per tile; hauls carry the exact frame id (`Dest`) so shared tiles deliver correctly.
- Drafted pawns take no automatic jobs (including eating/sleeping) until undrafted.
- Prioritized jobs are not interrupted by tiredness/hunger (exhaustion still interrupts).
- A finished blocking building pushes pawns on its tile to a fixed-order neighbour (deterministic).

## Verification

- `gofmt`, `go vet ./...`, `go test -race ./...` pass; `staticcheck` only pre-existing SA4000 in `pack_test.go` / `reservation_test.go`.
- New tests: placement reasons, floor+chair sharing a tile, door vs wall walkability + ejection, deconstruct refund, cancel clears marks, draft hold/resume, prioritize vs tiredness, `OrdersAt` reasons; UI: placement cells, rotation, drag → only valid blueprints, area command, draft button, order menu, Escape order.
- In-game: `make art-capture` → `assets/generated/ingame_{capture,architect,orders}.png`.

## Determinism Impact

All new behaviour is command-driven and tick-ordered; ejection uses a fixed neighbour order. No RNG or wall-clock added.

## Save / Mod Impact

No save format yet. Buildable defs are code; plugin-provided buildables would extend `construction` defs and tool catalog later.

## Art / Visual Impact

Blueprint ghosts, deconstruct marker, door open state, floors/furniture in game; new UI icons from the art pipeline.

## Screen / UI Impact

Architect panel rebuilt (categories, cards, tooltip, info strip); inspector Draft button; colonist bar draft badge; right-click order menu.

## Known Limitations

- 1×1 footprints only (no 1×2 bed / 2×1 workbench yet); no material choice, research locks, or drag-continuous paint.
- No multi-select/multi-draft, queued orders, forbid, hunt or blueprint/frame inspector.
- Table/chair/lamp have no gameplay effect yet (comfort, joy, light).
