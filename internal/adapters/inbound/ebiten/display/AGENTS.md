# internal/adapters/inbound/ebiten/display

## Purpose
The Ebitengine window and monitor calls the composition root makes, moved out of `cmd/game` (2026-11, GAR) because `internal/compose/game` imports no third-party module and the `ebiten` root is at its 60-file ceiling. Window size presets and borderless mode are OPT-029, the OS cursor OPT-028, the fps probe's floating window PERF-002.

## Owns
`Screen` (zero value ready): `Window`, `Apply`, `Cursor`, `DeviceScale`, `MonitorSize`, `Run`. `Window` is the window `Run` opens; `Game` aliases `ebiten.Game`.

## Depends on
`github.com/hajimehoshi/ebiten/v2` only.

## Must not depend on
The `ebiten` root (`internal/arch` `TestSOLID`); `Run` takes the App as a `Game`.

## Contracts
`Apply("WxH", false)` sets that size, `""` leaves it free (`TestParseSize`). `Run` sets the window title, TPS and size before `RunGame`, and with `Probe` covers the display with a floating window.
`Window.Icons` (from `LoadIcons`, `assets/generated/ui/app_icon/icon_{16,32,48,256}.png` found via `assetroot`) are passed to `ebiten.SetWindowIcon` by `Run` when non-empty. The files come from `make app-icon` (`pkg/appicon`, `cmd/iconpack`), never hand-edited.
