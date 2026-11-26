# internal/adapters/inbound/ebiten/dialogs

## Purpose
The colony's trade, comms and caravan dialogs: what a trader or faction offers, the deal the player builds, and the commands that close it. `Dealings` keeps the open dialog and reaches the simulation through `Shell` and its own sources. Feature package (ARCH-005 wave 5.3, agent A10).

## Owns
- `Dealings` (40 methods, at the `SOLID_GO.md` §8 cap: new draft code goes on `*formCaravan` or free functions, never a new `Dealings` method; 5 exported: `SyncDealings`, `OpenFormCaravan`, `Open`, `Paused`, `HoldPause`), built by `New(sh Shell) *Dealings`.
- `Host` (7 methods: `Trade`, `Comms`, `Caravans`, `HoldSpeed`, `DrawPortrait`, `Icon`, `Items`).
- `TradeSource`, `CommsSource` and `CaravanSource`: optional capabilities (`TradeView`, `CommsView`; `WorldSites` and `FormEstimate`) that the root asserts on `Simulation` (`.agents/specs/SOLID_GO.md` §5). The Form caravan dialog reads its settlements and its trip estimate through `Host.Caravans()`.
- The Form caravan draft (`formCaravan`, `caravan.go`, `caravan_pack.go`, `caravan_plan.go`): colonists, prisoners (`form.prisoner.<id>`, under their own heading, disabled when downed), tame animals, goods (people's food first under "Food", `caravan.Edible`). `Name` is colonists, prisoners, animals, `stack:qty`. The summary shows food days (`caravan.Days`) against `FormEstimate` travel days, memoized per (destination, eaters, load); a shortfall warns ("Not enough food for the trip.") and leaves Send enabled.
- Pure draft maths and text: `TradeLines`/`TradeLine`, `CaravanTitle`, `CaravanTip`, `CaravanHealthNote`, `MemberList`, `SilverText`; the `tradeDraft` type (11 methods) holds the deal being built.
- Modal IDs: `TradeModalID`, `CommsModalID`, `FormCaravanID`, `LoadPodsID` (AX-9: the Load transport pods dialog, `OpenLoadPods`).

## Depends on
- Leaves: `gfx`, `inputframe`, `ports`, `uikit`.
- Application read models: `application/game` (`TradeView`, `CommsView`, `WorldSiteView`, `CaravanView`).
- Domain value types: `animal` (an animal's body class for its mass), `economy/item`, `economy/trade`, `faction`, `sim/clock`, `sim/command`, `sim/entity`, `world/caravan` (the pods' masses, AX-9).
- `pkg/mathx` (`Abs`), `pkg/randx` (`SplitMix64`).

## Must not depend on
- Other feature packages (`hud`, `architect`, `inspector`, `windows`, `orders`, `learn`, `devtools`, `perf`, `options`, `wizard`, `mainmenu`, `selection`).
- `render/*`.

## Contracts
- `Shell` = `ports.Commands + World + Speed + Widgets + Host`.
- Every deal, gift and caravan form closes through `ports.Commands` (`command.Command` values); the dialog mutates no domain state. Trade and comms data come from read models.
- `Dealings.Open` and `OpenFormCaravan` open the dialogs; `HoldPause` and the `Host`'s `HoldSpeed` hold the sim's pause and speed while a dialog is open.

## Invariants
- Gift and deal totals are clamped to silver in reach (`TestCommsGiftClampsToSilverInReach`).
- Caravans with sick members show the health note (`TestCaravanShowsSickMembers`).
- Dialog widgets are declared in the `Update` pass (`TestDialogsDeclareTheirWidgets`).

## Hot paths
- Widgets are declared once per `Update`; `Draw` paints the open dialog. Not benchmarked.

## Tests
- 10 source files, 10 test files, 25 `Test*` functions (`TestTradeDialogTotals`, `TestCommsGiftClampsToSilverInReach`, `TestCaravanShowsSickMembers`, `TestDialogsDeclareTheirWidgets`; the Form caravan's `TestCaravanListsPrisonersApart`, `TestCaravanNameOrder`, `TestCaravanCapCountsPrisoners`, `TestCaravanFoodAgainstTravel`, `TestCaravanEstimateMemoized`).
- Coverage 77.1 % (#14).

## Notes
- `TradeSource`/`CommsSource` are optional: the root asserts them on `Simulation`, so a source without them is handled by the dialog's own code path.
- New wanderers picker (MAP-19): `wanderers.go`: `OpenWanderers` (from `ports.OpenWanderers`) opens a modal of up to `AX.WanderersMax` slots with a reroll each and the chosen person's card (`game.WandererBio`); Start sends `command.NewWanderers` (Name = seeds). Seeds come from the open tick and a local counter (UI state). Counts in `Open()`, so the display pauses while it is open. Imports `domain/pawn/pawngen` for `Rules` only.
