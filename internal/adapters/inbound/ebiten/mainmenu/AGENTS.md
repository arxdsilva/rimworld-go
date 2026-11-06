# internal/adapters/inbound/ebiten/mainmenu

## Purpose
The main menu screen: the hero splash and the colony diorama behind it, the button rail (Continue, Load, New, Options, Credits, Quit), the credits text, and the RAM and update notice cards. It reaches the shell through a small interface and the simulation not at all. Feature package (ARCH-005 wave 5.1, agent A3).

## Owns
- `Screen` built by `New(sh Shell) *Screen`: splash, diorama, rail, credits lines, RAM and update notice cards.
- `Diorama` and `LoadDiorama(root)`: the menu scene loaded from assets.
- `NewestSlot(slots []ports.SaveSlot) int`: picks the slot Continue uses.
- `Host` (8 methods): `LoadSplash`, `LoadMenuScene`, `Version`, `MenuTick`, `HasSaves`, `LowRAMNotice`, `DismissRAMNotice`, `DrawFlame`.
- `Updates` (`update_notice.go`, 2 methods): `UpdateAvailable`, `DownloadUpdate`. `cmd/game` wires `outbound/updatecheck`. Separate from `Host`, which is at the 8-method limit.
- `VersionLine`, `UpdateCard`: the running version with any newer one, and the notice's card.
- `Shell` = `Sizer + Settings + Widgets + Host + Updates`.

## Depends on
- Leaves: `art`, `gfx`, `ids`, `ports`, `uikit`.
- `platform/assetroot` (asset path), `platform/config` (settings), `pkg/flatyaml` (the diorama's metadata), `pkg/randx` (`Mix64`).

## Must not depend on
- Other feature packages (`hud`, `architect`, `inspector`, `windows`, `orders`, `dialogs`, `learn`, `devtools`, `perf`, `options`, `wizard`, `selection`).
- Domain and application packages: the menu reads no simulation state.

## Contracts
- Navigation is requested through `ports.Intent` values (`Push`, `StartGame`, `Quit`, and the rest); the root's navigator performs the change. Click routes stay in the root.
- `Screen` declares its widgets once per `Update`, then draws.
- Reads `assets/` (splash, diorama) through `LoadSplash`/`LoadDiorama` and save slot metadata through `ports.Saves` (`HasSaves`, `NewestSlot`). Writes nothing.
- Animation time comes from `Host.MenuTick`, not the wall clock, so the menu tests are deterministic.

## Invariants
- The menu view covers the whole screen (`TestMenuViewCoversScreen`).
- The update notice sits on the left under the version line, clear of the rail and the RAM card. Later hides it for this run only; nothing is saved (`TestUpdateCardFitsAndClearsTheRail`, `TestJourneyFlowUpdateNotice`).
- Flicker and breath ranges stay within bounds (`TestMenuFlickerAndBreathRange`); motes are deterministic (`TestMenuMotesDeterministic`) and the slow-path view is deterministic (`TestMenuViewSlowAndDeterministic`).

## Hot paths
- Splash and diorama are loaded once; `Draw` composes them each frame. Not benchmarked.

## Tests
- 5 source files, 2 test files, 14 `Test*` functions.
- Coverage 41 % (ARCH-005 row).

## Notes
- The click routes stay root for now.
- `menuTick` and `slotsRead` stay on `App`: the capture and the backdrop read them.
