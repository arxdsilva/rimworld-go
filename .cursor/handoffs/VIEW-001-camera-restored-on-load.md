# VIEW-001: the camera is restored when a game loads

User request (2026-10-30): loading a game always centred the camera; it should come back where the player left it.

## What changed
- `camera.View{CX, CY, Zoom}` holds the world centre in tiles plus the apparent zoom, never pixel offsets, so it survives a different window size or device scale. `(Camera).View(vp)` captures it.
- `(*Camera).Resume(v)` stores a pending view, ignoring NaN, infinite or zero-zoom values. `Clamp`'s unplaced branch then uses it, clamped to the zoom limits and the map, and sets `Placed`.
- `filesave.Store.SaveView` writes `<slot>.rwview` (JSON, atomic temp file and rename) beside the save. `LoadView(savePath)` returns false for a missing, corrupt or invalid file, which keeps today's centring. `List` ignores the view file.
- `SaveHooks.SaveView`/`LoadView` are optional and nil-safe, wired in `cmd/game/main.go`.
- `optionsSaver.Save` writes the view after a successful save; a failure is only logged. `Load` resumes it after `startSession`. This covers manual save, autosave, save-and-quit and window close.
- Adapter-only: no save-format, application or domain change, and `App`/`Renderer` didn't grow.

## Tests
- camera (75 % coverage): the round trip over three viewport sizes and device scales, clamping, and bad views keeping today's centring.
- filesave (75.2 %): the round trip, an old save with no view file, and a corrupt view file.
- `TestSaveWritesViewAndLoadResumesIt` and `TestJourneyCameraRestoredOnLoad`. The view applies on the first `Update` after the load, as in the real app.

## Gaps
- The store has no delete, so a view file stays behind if its save file is removed by hand.
