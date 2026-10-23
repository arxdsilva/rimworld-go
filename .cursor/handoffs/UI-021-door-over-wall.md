# Task Handoff

Task ID: UI-021 (PLAY-010)
Status: complete

## Summary
A door or fence gate may be placed over a wall, fence, door or gate of a different kind. The old piece is marked for deconstruction; the new frame waits until it is gone, then builds normally.

## Behaviour
- `canPlace` skips Occupied/Blocked only for that pairing; same kind over same kind, walls over doors and floors/carpets on any of them stay refused.
- `construction.Frame` gains `Replaces` and `ReplaceMarked` (gob, additive).
- While the old piece stands: builders skip the frame (`constructTry`, and `completeFrame` returns early so dev hooks can't build through it), forced orders refuse with "The wall must come down first", and the frame inspector reads "Waiting for the wall to be removed". Haulers may still deliver.
- Cancelling the frame clears the mark it set; unmarking the wall cancels the frame.
- The Architect ghost stays valid and reads "replaces wall" (`ReplacesAt` on the renderer's `Simulation` interface).
- Rooms merge for the moment the cell is open, then re-split when the door is built.

## Tests
`internal/application/game/door_replace_test.go`; `TestCanPlaceReasons`, `TestCarpetNotOnDoor` unchanged and green.
