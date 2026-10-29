# inspector package

## Package Purpose
The selection's inspector: the pane with its tabs (pawn needs, gear, health, social, bio, log; animals, buildings, frames, items, plants, pens, troughs, caskets, groups), the command grid under it, the command producers for every selectable kind, and the marks drawn over what is selected. `Pane` reads the selection and the world through `Shell` and emits typed commands; the root plugs the Architect's tool commands in for an empty selection. Feature package (ARCH-005 wave 5.4, agent A7).

## Owned Concepts
- `Pane` (37 methods): tabs, body, layout, draw. Each kind is a type over `*Pane`: `Commands` (26), `Pawn` (19), `Tabs` (32: health, social, bio, log, prisoner, pen, trough), `Animal` (17), `Building` (36), `Thing` (11), each `struct{*Pane}`.
- `Producers map[selection.Kind]Producer{Pane, Commands}`: `New` fills the six inspector kinds; the root adds `selection.None` (the Architect's tools) and `selection.Zone`.
- `Run` and `Options` are closures; `Chip` and `Producer` are its small value types.
- `Shell` = `Sizer + Commands + World + PawnSel + Widgets + Host + Peers + Labels`.
- `Host` (8 methods, plus `Dispatcher` and `Hinter`): `Env`, `Layout`, `Pick`, `Click`, `Selection`, `Hostiles`, and others.
- `Peers` (8): the Architect's areas, move mode, buildability, research locks. `Labels` (8): text and portrait helpers. `Sim` (7 read methods: `OrdersAt`, `Health`, `Social`, `Prisoner`, `Bio`, `LearnedThisWeek`, `PlantGrowth`).
- `bedPrisonReader` (in `building_inspector.go`): one method, `BedForPrisoners`.

## Allowed Dependencies
- Layers: `render/env`, `render/overlay`, `render/sprite`, `render/world`.
- Leaves: `art`, `gfx`, `hudlayout`, `ids`, `ports`, `uikit`.
- `selection`: the selection model and kinds. Stated as fact: the inspector imports `selection`.
- `application/game` read models; domain value types (`ai/work`, `building/construction`, `economy/item`, `pawn/body`, `pawn/look`, `pawn/medcare`, `pawn/prisoner`, `pawn/skill`, `sim/clock`, `sim/command`, `sim/entity`, `world/plant`); `platform/input`.
- Ebitengine; stdlib `reflect`, `math`, `fmt`, `image/color`.

## Forbidden Dependencies
- The ebiten root package and `cmd/`.
- Other feature packages: `hud`, `architect`, `windows`, `orders`, `dialogs`, `learn`, `devtools`, `perf`, `options`, `wizard`, `mainmenu`.
- Map-side selection code: `handleMapMouse`, box select and the group marks stay in the root.

## Public Contracts
- `New(sh Shell) *Pane` and the kind types; `Producers` is the lookup from `selection.Kind` to the producer that builds commands for it.
- Every action is a `command.Command` through `ports.Commands`; the inspector mutates no domain state.

## Invariants
- Guard limits: `Pane` 37, `Building` 36 and `Tabs` 32 methods (all ≤ 40).
- Power warnings are marked (`TestPowerWarningLinesAreMarked`); pen food verdicts are deterministic (`TestPenFoodVerdict`).

## Hot Paths
- The pane reads its read models each frame and draws them.
- No benchmark in the package; no allocation claim.

## Determinism Rules
- Presentation only. The simulation is reached through `Sim` (reads) and `ports.Commands` (writes).

## Save/Mod Impact
- None.

## Tests
- 26 source files, 6 test files, 26 `Test*` functions (`TestBillsBodyNotesAnEmptyBench`, `TestPowerWarningLinesAreMarked`, `TestFmtTarget`, `TestPenFoodVerdict`).
- Coverage 11.1 % (ARCH-005 row); the inspector runs mainly through the root journey tests.

## Benchmarks
- None.

## Known Design Constraints
- The largest package by lines: 7,318 source lines in 26 files. Split by kind further before adding a seventh tab type.
- Converting shims `healthOption` and `itemVerb` remain for three root tests.
- `selection.Zone` producer (`cmds_zone.go`) stays root until it joins `architect.Zones`.
