# OPT-2 cursor: OPT-028 custom cursor takes effect

## What changed
- `internal/adapters/inbound/ebiten/pointer` (new sub-package, because the root package is at its 60-file ceiling): `Draw`, `Placement`. No `App` fields or methods, no new root file.
  - The `App.Draw` tail (inline, last) calls `pointer.Draw`. With `Camera.CustomCursor` on (and no capture) it hides the OS cursor and draws `cursor/pointer` through `gfx.DrawImage` at the mouse minus the hotspot, 32 logical px wide (hotspot scaled the same), so it follows the UI scale and shows in the gfx draw trace. Off, it is one branch.
  - `cursor/pointer_busy` draws while the New Colony loading screen runs (`a.ng.Load != nil`). Saves are not a synchronous state in the client, so none shows there.
  - Art missing: the OS cursor is kept and nothing is drawn.
  - A 3-line check in `applySettings` restores the OS cursor when the option goes off.
- `AppConfig.Cursor func(visible bool)` (one field in `app.go`, like `Window`/`Display`); `cmd/game/display.go` `applyCursor` calls `ebiten.SetCursorMode`; wired in `cmd/game/main.go`. While on, `drawCursor` re-asserts hidden each frame (cheap, idempotent).
- `assets/metadata/orphan_allow.txt`: removed the two cursor lines.
- Tests: `pointer/pointer_test.go` (hotspot maths), `journey_cursor_test.go` (toggle option, draw trace shows one extra image of the right size at the hotspot, hook sees hidden/visible).

## Gaps
- No busy pointer while saving. The text cursor and system shapes are untouched.
- Hash impact: none (presentation only).
