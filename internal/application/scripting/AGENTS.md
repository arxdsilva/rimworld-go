# internal/application/scripting

## Purpose
The port plugins' scripts run behind (PLG-060): the simulation owner calls a `Host` at fixed points of the tick and gets typed commands back. The Starlark runtime is `adapters/outbound/starlarkhost`; the Wasm runtime is `adapters/outbound/wasmhost`. The script-writer reference is the package comment in `doc.go`; keep it in step with any hook, ctx attribute, event or limit you change.

## Owns
- `scripting.go`: `Host`, `Event`, `Result`, the budgets (`MaxSteps`, `MaxCommands`, `MaxText`) and `PawnEvery`.
- `ext.go`: the extension-point contract: `EventKind` (quadrum, year, research_done, recipe_done, map_generated), `Craft`, `MapGen`, `CommandView`, `Panel`, `Queries` (the read-only readers), and `MaxQueryRows`, `MaxPanelLines`, `MaxActions`, `OrderEvery`.

## Must not depend on
`application/game` (the game calls in; `scripting/gameread` is the one sub-package that reads the game), any runtime library.

## Contracts
- Scripts act only through typed commands (the `Script*` commands in `domain/sim/command`) applied on the next tick; read-only hooks (`panel`, `validate_command`, `validate_defs`, `register_defs`) queue none.
- A script order is accepted at most once per pawn per `OrderEvery` cadence visits (`TestExtScriptOrderNoFlap`).
- Events are delivered in a fixed order (`doc.go`, "Events"): on_day, quadrum, year, on_hour, panels.

## Tests
`scripting_test.go`, `ext_test.go`; per-hook proofs in `starlarkhost` and `application/game/ext_*_test.go`; end to end `TestExtE2E` in `compose/game`.

## Notes
- A new extension point: capability in `pluginapi`, rule in `starlarkhost/caps.go` (`TestCapsTableComplete` fails until both agree), a seam in `application/game`, a row in `doc.go`, and a checklist box.
- Wasm has no capability enforcement and no extension hooks yet.
