# internal/adapters/inbound/ebiten/uikit

## Purpose
The immediate-mode widget kit (NG-1 §1.4): per-`Update` widget declarations with hit testing, focus, keyboard navigation, scrolling, text editing, modals, tooltips and the paint list, drawn through `gfx`. It reaches its host through one small interface (`Host`) and knows nothing of screens or the game. A screen declares its widgets once per `Update`, right after the input is polled and before the click path.

## Owns
- `Kit` (73 methods, 50 exported) with exported state; the moved `uiState` (budget `uikit.Kit 73` methods, `uikit.Kit 30` fields).
- `Host` (`host.go`, exactly 8 methods): `Frame`, `Cursor`, `Size`, `Updates`, `KeyRepeat`, `Cue`, `Hint`, `ReducedMotion`. `Kit.Bind(h)` attaches it lazily; `Bound()` and `Host()` report it.
- `Owner` (`form.go`): `Kit() *Kit`; `Form` (8 methods) and its free functions `Row`, `Gap`, `Content`, `SectionTitle`, `Slider`, `Check`, `Stepper`, `Choice`, `Picker`, `Segmented`, `OnScreenLayer`, `WrapCols`, `StyledButton`.
- `Button`, `Modal`, `SliderSpec` (the slider's `Tick` is set by the root's step sound), paint and spec types, colour tokens, `Shade`, `Fade`, `LerpRGBA`, the dialog frame.

## Depends on
- Leaves: `gfx`, `inputframe`.
- `pkg/mathx` (`Abs`).

## Must not depend on
- Feature packages, every `render/*` package, `application/game`, `domain`, `sim`.
- `ports`: uikit is below ports, since `ports.Widgets` embeds the kit.

## Contracts
- `Host` is the only way the kit learns about the app; the root binds `uiHost{a}` on first use.
- Widgets return their state for the frame (clicked, hovered, focused); the caller decides what a click does.
- Input comes from `inputframe` through `Host.Frame`; `Updates` is the frame counter, the only time source in widget logic.

## Invariants
- The kit's method count is budgeted at 73 (`internal/arch/solid_budget.txt`); the seams to split next are text editing (14), modals (9), table (4), tooltip (3). Adding a method means splitting one of them first.
- Form rows declare their widgets (`TestFormRowsDeclareTheirWidgets`); rows scroll and clip (`TestFormRowsScrollAndClip`).
- The dialog frame lays out deterministically (`TestDialogFrameLayout`).
- Menu chrome colours match `assets/palettes/ui.yaml` (`TestUIPaletteMatchesYAML`).

## Hot paths
- Widget declaration runs once per `Update` per widget; the paint list is built during the update and drawn per frame. Text measurement goes through `gfx`.

## Tests
- 16 source files, 3 test files, 15 `Test*` functions; `fakeHost` (8 methods).
- Coverage 82.5 % (ARCH-003).

## Notes
- Root keeps `uiBegin`'s screen logic, the screen table dispatch, `ui_modal.go`'s `modal` type (41 call sites declare `Draw func(*App, rect)`; it wraps into `uikit.Modal`) and the window-kit coupling.
- The root reaches the kit through `App.uiSt()` and `ports.Widgets.Kit()`; `uikit_compat.go` keeps only the root's widget-kit host adapter.
- The difficulty widgets shared with `options` and `wizard` were lifted into `form.go` (ARCH-005 wave 5.2).
