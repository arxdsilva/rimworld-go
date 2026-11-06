# internal/adapters/inbound/ebiten/inputframe

## Purpose
The client's input abstraction: the raw device state polled once per `Update`, the resolved frame of fired actions and mouse buttons, and the fake source tests drive. It knows nothing of screens or the game. Moved in ARCH-003 wave 3.1 (203 lines).

## Owns
- `Raw`: the polled device state (keys, mouse position and buttons, wheel).
- `Frame` (11 methods) built by `NewFrame(raw Raw) Frame`: the one-frame view of fired actions and clicks, with `RepeatFire` and the click-taken-once bookkeeping.
- `Source` (`Poll() Raw`); `EbitenSource` (the device); `FakeSource{Cur Raw}` (tests).
- `MouseBtn` with `Left`, `Middle`, `Right`, `NumMouse`; `UIPoint`.

## Depends on
- Ebitengine: `ebiten`, `ebiten/inpututil`.
- `platform/input` (the action enum and the key bindings).

## Must not depend on
- Every feature or render package: `inputframe` sits below `uikit` and the features.

## Contracts
- `Source.Poll()` is called once per `Update`; the resulting `Frame` is the only input read during that update. The simulation never polls input itself.
- A click is taken once: after `Frame` marks it taken, later readers see no click.

## Invariants
- `FakeSource` clears one-frame state on each `Poll` (`TestFakeSourceClearsOneFrameState`).
- Key repeat is checked by `TestRepeatFire`.

## Hot paths
- `NewFrame` and the `Frame` queries run once per `Update`. Value types; not benchmarked.

## Tests
- 2 source files, 1 test file, 4 `Test*` functions (`TestRepeatFire`, `TestFrameClickTakenOnce`, `TestFrameTop`, `TestFakeSourceClearsOneFrameState`).
- Coverage 34 % (ARCH-003): the Ebitengine source needs a device and is not exercised headless.

## Notes
- The root's `input_frame.go` methods stay root; the root test type `fakeInput` embeds `FakeSource`.
