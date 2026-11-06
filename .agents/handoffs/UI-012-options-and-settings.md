# Task Handoff

Task ID: UI-012
Status: complete

## Summary

Credits now open with "Created by arxdsilva". Options has four tabs (Audio, Graphics, Gameplay, Accessibility). Every setting is applied immediately and saved to `<user config dir>/rimworld-go/settings.json`. Controls and key rebinding are deferred.

## Files Changed

- `internal/platform/config/` (new `settings.go`; doc): the `Settings` type, defaults, `Normalize`, JSON `Decode`/`Encode`, `Gain`, `Graphics.FX`
- `internal/adapters/outbound/settingsfile/` (new): atomic JSON store; a corrupt file is kept as `.bad`
- `internal/adapters/inbound/ebiten/`:
  - `settings.go` (new): `SettingsHooks`, load/apply/save
  - `screen_options.go` (rewritten): table-driven tabs, steppers, toggles, choices
  - `app.go`: `AppConfig.Settings`, `Window`, `Sound`; `settings` replaces `noThreatPause`
  - `renderer.go`, `camera_smooth.go`, `anim.go`, `environment.go`: effects detail, edge scrolling, pan speed, reduced flashing
  - `saves.go`: autosave interval
  - `menu_diorama.go`: reduced motion and effects
  - `capture.go`: `-capture-ui credits|options|options-graphics|options-gameplay|options-access`
  - `screen_menu.go`: `creditsLines`
- `cmd/game/main.go`: `settingsHooks` (none when capturing) and the window hook

## Public Contracts Changed

- `AppConfig.Settings` (zero value: defaults, nothing saved).
- `AppConfig.Window(fullscreen, vsync)` (nil: leave the window alone).

## Invariants / Decisions

- Settings are presentation-only. Captures ignore saved settings.
- The stepper is `−`/bar/`+` buttons rather than a drag slider: the debug font and hand-placed layouts, and it stays on the existing click path.
- Deferred, with reasons in MAIN UI §42:
  - UI scale and text size (need a font and layout pass)
  - resolution presets (the window resizes freely)
  - frame cap (Ebitengine only has VSync)
  - colour-vision support, tooltip delay, far-zoom detail
  - Controls

## Verification

Tests:
- `internal/platform/config`: round trip, partial JSON, normalize, decode error.
- `settingsfile`: missing file, round trip, corrupt file set aside.
- `options_test.go`:
  - layout without overlaps at 1280×720 and 960×540;
  - clamping, with one save per click;
  - effects cycling, reaching the renderer;
  - tab switching;
  - settings surviving a restart;
  - `edgePan`;
  - menu particles;
  - autosave every 3 days.
- `threat_ui_test.go`: pause on threat.

Captures: Credits and all four Options tabs checked by eye at 1280×720.

## Determinism Impact

None.

## Save / Mod Impact

None (the settings file is separate from saves).
