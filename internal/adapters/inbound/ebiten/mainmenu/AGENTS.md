# mainmenu package

## Package Purpose
The main menu screen: the hero splash and the colony diorama behind it, the button rail (Continue, Load, New, Options, Credits, Quit), the credits text and the RAM notice card. It reaches the shell through a small interface and the simulation not at all. Feature package (ARCH-005 wave 5.1, agent A3).

## Owned Concepts
- `Screen` (9 methods, 8 exported) built by `New(sh Shell) *Screen`: splash, diorama, rail, credits lines, RAM notice card.
- `Diorama` and `LoadDiorama(root)`: the menu scene loaded from assets.
- `NewestSlot(slots []ports.SaveSlot) int`: picks the slot Continue uses.
- `Host` (8 methods): `LoadSplash`, `LoadMenuScene`, `Version`, `MenuTick`, `HasSaves`, `LowRAMNotice`, `DismissRAMNotice`, `DrawFlame`.
- `Shell` = `Sizer + Settings + Widgets + Host`.

## Allowed Dependencies
- Leaves: `art`, `gfx`, `ids`, `ports`, `uikit`.
- `platform/assetroot` (asset path), `platform/config` (settings).
- Ebitengine (`ebiten`, `vector`); stdlib (`bufio`, `errors`, `image`, `image/png`, `os`, `path/filepath`, `sort`, `strings`, `time`).

## Forbidden Dependencies
- The ebiten root package and `cmd/`.
- Other feature packages (`hud`, `architect`, `inspector`, `windows`, `orders`, `dialogs`, `learn`, `devtools`, `perf`, `options`, `wizard`, `selection`).
- Domain and application packages: the menu reads no simulation state.

## Public Contracts
- Navigation is requested through `ports.Intent` values (`Push`, `StartGame`, `Quit`, and the rest); the root's navigator performs the change. Click routes stay in the root.
- `Screen` declares its widgets once per `Update`, then draws.

## Invariants
- The menu view covers the whole screen (`TestMenuViewCoversScreen`).
- Flicker and breath ranges stay within bounds (`TestMenuFlickerAndBreathRange`); motes are deterministic (`TestMenuMotesDeterministic`) and the slow-path view is deterministic (`TestMenuViewSlowAndDeterministic`).

## Hot Paths
- Splash and diorama are loaded once; `Draw` composes them each frame. No benchmark in the package.

## Determinism Rules
- Presentation only. Animation time comes from `Host.MenuTick`, not the wall clock, so the menu tests are deterministic.

## Save/Mod Impact
- Reads `assets/` (splash, diorama) through `LoadSplash`/`LoadDiorama`.
- Reads save slot metadata through `ports.Saves` (`HasSaves`, `NewestSlot`). Writes nothing.

## Tests
- 4 source files, 2 test files, 12 `Test*` functions.
- Coverage 41 % (ARCH-005 row).

## Benchmarks
- None.

## Known Design Constraints
- The click routes stay root for now.
- `menuTick` and `slotsRead` stay on `App`: the capture and the backdrop read them.
