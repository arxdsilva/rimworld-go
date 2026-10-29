# dialogs package

## Package Purpose
The colony's trade, comms and caravan dialogs: what a trader or faction offers, the deal the player builds, and the commands that close it. `Dealings` keeps the open dialog and reaches the simulation through `Shell` and its own sources. Feature package (ARCH-005 wave 5.3, agent A10).

## Owned Concepts
- `Dealings` (18 methods, 5 exported: `SyncDealings`, `OpenFormCaravan`, `Open`, `Paused`, `HoldPause`), built by `New(sh Shell) *Dealings`.
- `Host` (4 methods: `Trade`, `Comms`, `Sites`, `HoldSpeed`).
- `TradeSource` and `CommsSource`: one-method optional capabilities (`TradeStatus`, `CommsView`) that the root asserts on `Simulation` (SOLID_GO.md §5).
- Pure draft maths and text: `TradeLines`/`TradeLine`, `CaravanTitle`, `CaravanTip`, `CaravanHealthNote`, `MemberList`, `SilverText`; the `tradeDraft` type (11 methods) holds the deal being built.
- Modal IDs: `TradeModalID`, `CommsModalID`, `FormCaravanID`.

## Allowed Dependencies
- Leaves: `gfx`, `inputframe`, `ports`, `uikit`.
- Application read models: `application/game` (`TradeView`, `CommsView`, `WorldSiteView`, `CaravanView`).
- Domain value types: `economy/item`, `economy/trade`, `faction`, `sim/clock`, `sim/command`, `sim/entity`.
- Ebitengine (`ebiten`); stdlib (`fmt`, `slices`, `strconv`, `strings`).

## Forbidden Dependencies
- The ebiten root package and `cmd/`.
- Other feature packages (`hud`, `architect`, `inspector`, `windows`, `orders`, `learn`, `devtools`, `perf`, `options`, `wizard`, `mainmenu`, `selection`).
- `render/*`.

## Public Contracts
- `Shell` = `ports.Commands + World + Speed + Widgets + Host`.
- Every deal, gift and caravan form closes through `ports.Commands` (`command.Command` values); the dialog itself mutates no domain state.
- `Dealings.Open` and `OpenFormCaravan` open the dialogs; `HoldPause` and the `Host`'s `HoldSpeed` hold the sim's pause and speed while a dialog is open.

## Invariants
- Gift and deal totals are clamped to silver in reach (`TestCommsGiftClampsToSilverInReach`).
- Caravans with sick members show the health note (`TestCaravanShowsSickMembers`).
- Dialog widgets are declared in the `Update` pass (`TestDialogsDeclareTheirWidgets`).

## Hot Paths
- Widgets are declared once per `Update`; `Draw` paints the open dialog. The package has no benchmark, so no allocation claim.

## Determinism Rules
- Presentation only; never reaches the simulation's RNG or wall clock. Trade and comms data come from read models, not from the sim directly.

## Save/Mod Impact
- None.

## Tests
- 6 source files, 4 test files, 8 `Test*` functions (`TestTradeDialogTotals`, `TestCommsGiftClampsToSilverInReach`, `TestCaravanShowsSickMembers`, `TestDialogsDeclareTheirWidgets`).
- Coverage 73.7 % (ARCH-005 row).

## Benchmarks
- None.

## Known Design Constraints
- `TradeSource`/`CommsSource` are optional: the root asserts them on `Simulation`, so a source without them is handled by the dialog's own code path.
