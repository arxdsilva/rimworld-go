# ports package

## Package Purpose
The contract between the ebiten client's shell (the root package: `App` and `Renderer`) and its feature packages. Each role is one small consumer-owned interface (SOLID_GO.md §6, HEXAGONAL §6). A feature composes the roles it calls into its own `Shell`, and the root satisfies them all. `Intent` is the closed set of cross-feature requests a feature hands back to the shell instead of calling another feature (SOLID_GO.md §4).

## Owned Concepts
- Roles (interfaces): `Commands`, `Dispatcher`, `Hinter`, `Overlays`, `Panels`, `PawnSel`, `Saves`, `Settings`, `Sizer`, `Speed`, `View`, `Widgets`, `World`. Each is at most 8 methods (guard).
- `Widgets{Kit}`: the one widget-kit role (`*uikit.Kit`).
- `Intent` (closed set): `Push`, `Back`, `StartGame`, `Quit`, `ReturnToMenu`, `Resume`, `Pause`, `CenterOnPawn`, `ClearSelection`, `CloseArchitect`, `CloseWindows`, `OpenArchitect`, `PresetBuildTool`, `SaveSlot`, `SelectPawn`, `ShowWindow`, `TogglePawnInSet`.
- `Screen` (an int type with the screen names; the root's `screenID` aliases it).
- `SaveSlot`: the plain listing record that `Saves` returns.

## Allowed Dependencies
- `uikit` (the `Widgets` role is the kit).
- `application/game` read models and the `game` command types; `domain/sim/clock`, `sim/command`, `sim/entity`.
- `platform/config`, `platform/input`; imports `time`.

## Forbidden Dependencies
- Concrete feature packages and `render/*`: a role is defined here, never implemented here.
- The ebiten root package and `cmd/`.
- New behaviour: this package is contract only (interfaces, intents, plain data). `Widgets` is the one struct, embedding the kit.

## Public Contracts
- Roles are satisfied by the root's `App` and `Renderer` through `<pkg>_compat.go` shims and the root's `Host` adapters.
- A new cross-feature request is a new `Intent` value, not a method on another feature.
- A feature that needs something no role provides declares one local interface in its own `deps.go`.

## Invariants
- Every role ≤ 8 methods (guard; the guard counts each embedded interface as one member).
- `Intent` types carry only plain fields; the root's navigator switches on them.
- No package-level mutable state.

## Hot Paths
- None in this package. Roles are called from feature code; their cost is the callee's.

## Determinism Rules
- Not applicable: no state, no clock reads. `time` appears only in type positions.

## Save/Mod Impact
- None. `SaveSlot` describes a listing; the save files themselves are written by `options`/the root through `Saves`.

## Tests
- None in the package. Each role is exercised by the feature packages' `fakeShell` tests.

## Benchmarks
- None.

## Known Design Constraints
- The `Screen` names are referenced by the root's screen table; adding one means adding it in `ports` first.
- ARCH-005 added the contract first (`Screen`, the navigation intents, `Widgets`, `Saves`); roles added later need a second caller.
