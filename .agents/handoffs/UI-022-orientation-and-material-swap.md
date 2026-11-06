# Task Handoff

Task ID: UI-022 (PLAY-011)
Status: complete

## Summary
- Doors and fence gates face along their walls or fences by themselves. `autoFacing` counts solid neighbours (walls, doors, fences, gates, built or planned, and natural rock): more east+west → spans east–west (South), more north+south → spans north–south (East), a tie keeps the player's Q/E (new piece) or the current facing (existing piece, so no flip-flop). It runs at placement and again whenever a neighbour is built, removed, cancelled or mined (`reorientAround`, O(1)). The renderer draws the stored facing for doors, frames, ghosts and shadows (the old wall-mask guess is gone); the ghost previews each cell's facing (`AutoFacingAt`).
- Placing the same piece on the same tile in a different material swaps it in place (walls, doors, fences, floors, carpets, sandbags, beds, tables, chairs): a frame with `Swaps` set takes the new material, a builder works it while the old piece keeps standing (rooms stay sealed), and on completion the same building gets the new material at full HP and the old material is refunded. Components stay. The same material is `ReasonSame`: a neutral grey ghost, nothing queued. A building with a pending swap can't be moved (`ReasonSwapPending`) and isn't repaired separately; if the target disappears, the frame finishes as a normal build.
- Fences and gates can no longer be moved (like walls and doors).

## Files
`game/placement.go` (`autoFacing`, `reorientAround`, `swapCheck`, `AutoFacingAt`, `ReplacesAt` "wood wall → steel"), `tick.go` (`completeFrame` swap path), `frame_view.go` ("Changing to steel"), `move.go`, `defense.go`, `world.go`, `save.go` (`DoorsOriented`, migrating doors in older saves), `statehash.go` (the format flag isn't hashed); `construction.Movable`; renderer `draw_art.go` (`doorAxisEW`, per-cell ghost facing, neutral same-material cells), `shadow.go`, `architect.go`, `archtools.go`, `move_mode.go`, `building_inspector.go` ("Changing to X (N%)").

## Tests
`orient_test.go`, `swap_test.go`, `play011_ui_test.go`; deliberately updated: `TestReplaceCanPlaceRules`, `TestCanPlaceReasons`, `TestCannotMoveWallOrGrave`, `TestFenceViewInWorld` (a gate beside a fence now faces along it), `TestFenceSpriteKeys`, the stone door-key cases, `TestMoveButtonDisabledForWalls`. Goldens unchanged.

## Notes
- Removing one wall from a door's own run can flip it when the other axis now has more neighbours (that is the rule, not flapping).
- `TestAtlasCoversKeys` fails in the working tree because face-scar pawn sprites were re-rendered by the ENG-003 work without re-packing the pawn atlas; unrelated to this task.
