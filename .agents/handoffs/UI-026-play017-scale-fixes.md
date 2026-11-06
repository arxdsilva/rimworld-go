# Task Handoff

Task ID: UI-026 (PLAY-017 §2–4, §5 UI, §6, §9, §11 UI, §12; plan `.cursor/PLAY017_PLAN.md`)
Status: complete

## Summary
**UI-scale bugs:**
- The colonist-bar and inspector portraits now draw through `uiDrawImage` (`portrait.go`).
- World-anchored labels go through `worldText`/`worldIcon`: zone labels, tame animal names, the chat, fight and draft icons. The wind arrow uses `uiLine`.

**Zone labels** (`zoneLabelLayout`):
- Centred on the zone cell nearest the middle, on a plate.
- Size is `0.45 × the shorter side`, from 11 px up to `zoneLabelCap(tile)`: 16 px zoomed in, rising to 40 px zoomed out.
- Drawn by `drawZoneLabels` after the objects.

**Hauling UI:**
- Orders › Haul things.
- `overlay/designation_haul` on marked stacks.
- Item panel: a "Found here" line and a Mark for hauling button; a group button too.
- New art: `ui_icon/order_haul` and `overlay/designation_haul` (overlay family).

**Chase UI:**
- Red danger messages.
- The hostile alert names who is chased.
- A red marker on every hostile animal, plus a dotted chase line.
- `onThreatPause` centres an off-screen chase.

**App:**
- Options › Gameplay "Run in background" (`background.go`).
- Double-clicking a colonist in the bar centres the camera on it; `centreOnPawn` halts smoothing.

**Inspectors:**
- Plant growth state caption and "Ripe in about N days".
- Gear tab: "(you chose this)" and "Changing into".

## Visual regression tests
- `drawTrace` (kind, text, rect, world/HUD pass) beside the unchanged `canvasTrace`.
- `TestUIDrawsThroughCanvas` is now go/ast with `file:func` exemptions:
  - raw draws only in exempt functions;
  - functions calling `worldToScreen*` use `worldText`/`worldIcon`, or convert with `uiScale()`;
  - HUD files never call world pixel helpers.
- `TestFrameInvariance` and `TestPanelsStayInTheirPanel` cover pan, scale, anchors and panels at UI scales 1–2 and device scale 1–2, in about 14 s under -race.
- `-capture-ui-scale s` takes scaled captures (captures were always scale 1, so they could not show these bugs).

## Notes
- `tracePass` is only written while a trace hook is installed, to avoid a race between parallel draw tests.
- Raw draws stay invisible to the trace; the AST guard covers them.
- Up close, a small stockpile's name still overlaps its stack counts.
