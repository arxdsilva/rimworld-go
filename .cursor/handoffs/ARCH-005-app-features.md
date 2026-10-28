# ARCH-005: the App's features leave the root

## Why
`App` had 950 methods over 126 files and 111 fields (ARCH-001): every HUD region, panel, screen and dialog was a method on it. Each feature becomes a package with one type behind a `Shell` composed of `ports` roles (`.cursor/SOLID_GO.md` §6–§7; ARCH-002 built the seam, ARCH-003 the leaves it draws with). The root keeps `App`, the loop, the navigator, the click routes and the screen table, and satisfies every role.

## Contract additions (lead)
`ports.Screen` with its names (the root's `screenID` aliases it), the navigation intents `Push/Back/StartGame/Quit/ReturnToMenu/Resume/Pause`, `ports.Widgets{Kit}`, `ports.Saves{Slots, RefreshSlots, LoadSlot, NewestSlot}` and `ports.SaveSlot`. A feature that needs something no role has declares ONE local interface (≤ 8 methods) in its `deps.go` that a small `<pkg>Host{a *App}` adapter in its root compat file satisfies.

| wave | package | agent | status |
|---|---|---|---|
| 5.1 | `mainmenu` | A3 (sonnet) | done — `Screen` (splash, diorama, rail, credits lines, RAM notice card) behind `Shell` = `Sizer + Settings + Widgets` and an eight-method `Host` (the loaders, version, menu tick, saves present, the RAM notice, the flame); 41 % coverage; the click routes stay root for now; `menuTick`/`slotsRead` stay on App (capture and the backdrop read them) |
| 5.1 | `selection` | A1 (sonnet) | done — `Model` (`Kind`, `Cand`, `Group`, `Cycle`, `Press`; `Select`, `Clear`, `ClickSelect`, `ShiftClick`, `ToggleInSet`, `SetGroup`, `ApplyBox`, `Selected*`, `Selectables`, `HandleMapMouse`, …) behind `Shell` = `World + PawnSel + View + Sizer + Commands` and an eight-method `Host` (creature hit tests, zone labels, orders); 47.8 %. Wired as a sole agent: `App.sel`, 38 root files repointed from the old fields to `a.sel.X`, 109 `Cand` literals keyed; the group pane/commands, the box-select and group-mark draws and the renderer hit tests stay root. `App` 111 → 95 fields |
| 5.2 | `wizard` (first extracting the difficulty/setup composites into `uikit/form.go`) | | |
| 5.2 | `options` (after the wizard's composites land) | | |
| 5.3 | `windows` | | |
| 5.4 | `architect` | | |
| 5.5 | `inspector` | | |
| 5.6 | `hud` | | |
| 5.7 | `orders`, `dialogs`, `learn`, `devtools`, `perf` | | |

## Findings
- A feature whose state other root files read (`selection`: 52 files) cannot be wired by a parallel agent: the field repoint is a serial, root-wide `perl -pi` per field — a sole agent's job after the package exists.
- The wizard and the Options story tab share the difficulty widgets (`diffSlider`, `diffCheck`, `diffStepper`, `textLines`, `declareOnScreenLayer`, the form context); they are pure kit composites over wizard-only button helpers, so the wizard move starts by lifting them into `uikit/form.go`.
