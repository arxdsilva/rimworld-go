# OPT-1 kit: tooltip delay, colour vision, larger small text, confirmations

- `uikit/access.go`: `Access` (Set, TooltipDelayMS, ColorVision, Confirm) on `Kit.Access`; `TipFrames()` (ms*60/1000, 0 = instant, unset = 24); `Palette`/`PaletteFor(name)` (status colours; variants are documented PLACEHOLDERS); `ConfirmIf(k, ...)` runs the action at once when off, else through `Kit.Confirm`.
- `BtnStyle.ColorsIn(palette, ...)`; `Colors` = normal. Kit's own buttons use the kit palette. Kit fields stay at 30 (dragY/pageDelta folded into `scrollSt`).
- `gfx`: `SetLargerSmall(bool)` (in `cur`, no new global); text of size 11 faces/measures at 13.
- Root `uikit_compat.go`: free `syncKit(a)` called from `uiHost.Frame`, copies Interface settings into the kit and canvas on change.
- Confirm: building deconstruct, inspector release, Animals-window release.
- YAML: `TestColorVisionVariantsMatchYAML` pins `ui.<variant>.{danger,good,warning}` once present; lead adds the keys and replaces the placeholder literals in `PaletteFor`.
- Other status uses of `uikit.Danger/Good/Warning` elsewhere still use the normal colours; only kit buttons follow the palette so far.
- Hash impact: none.
