# AX-0: inert contract for every vanilla Architect item

User request (2026-11-01): make every `missing` and `partial` row of `.cursor/ARCHITECT_GAP_ANALYSIS.md` real. ENG-003 is extended to AX-* (wiki numbers, original names). The inventory is in the lead's scratchpad (`ax-inventory.md`), and the wiki numbers are in `wiki-ax.md`.

## Rules for every owner
- **Append only, and everything is saved by number.** New construction kinds, item kinds, commands, orders, jobs, research projects, recreation activities and stain kinds were all appended at the end of their enums. Never reorder them.
- **Planned gate.** `construction.Kind.Planned()` (`construction/ax_gate.go`) is true for every AX kind until its owner deletes the kind's case from its own `construction/ax_<owner>.go`.
  - `game/placement.go:725` (placeFrameStuff) refuses Planned kinds and `Def.Unbuildable` kinds (urn, big screen, telescope).
  - Delete your case only when the kind works end to end: tool, art, behaviour, UI and journey.
  - Your `ax_<owner>.go` file is yours. Add your kind predicates there (for example `IsTable`, `IsWallLamp`, `Recreation()`); don't put them in construction.go.
- **Lead-owned shared files.** For changes to any of these, send the lead the exact diff:
  - construction.go (enum, defs, Def, Building, Frame, Movable), ax_gate.go, ax_hp.go, furniture.go;
  - power/power.go and power/ax.go;
  - item/item.go, value.go, category.go;
  - zone/filter.go;
  - command.go, job.go, research.go;
  - recreation/recreation.go and joykind.go, stain.go;
  - game/game.go and axview/axview.go;
  - save.go (the `case v < 11` body), solid_budget.txt.
- **Owners may edit shared code only by exact edits:**
  - your own command cases in game/tick.go `applyOne` (or host_commands.go);
  - your own lines in building_view.go `fillBuildingView`;
  - your own catalog rows (replace your placeholder in architect/planned.go with a ToolDef in catalog.go);
  - your own gap-doc and checklist rows.
  - Re-read each file before you edit it.
- **Views.** Fill your own fields of `BuildingView.AX`, `ItemView.AX`, `PawnView.AX`, `FrameView.AX` and `WorldView.AX` (`game/axview`). Each field's comment names its owner. Building's AX is in the equality check.
- **Data the contract fills in, for owners to use or correct through the lead:**
  - wiki defs, with materials /9 rounded (at least 1), work /10, and components as the wiki;
  - `Def.Extra` secondary materials;
  - hit points (ax_hp.go; for steel-only kinds, the wiki value divided by 1.4);
  - furniture comfort and beauty;
  - power specs;
  - research projects and unlock keys.
- **Def flags.** `Only` (one material), `StoneOnly`, `Unbuildable`, `Extra [3]Cost`. `Building.Tint` and `Uninstall`; `Frame.Tint` and `ExtraGot`. `item.Stack.Building` holds a crated building's construction kind.
- **Extra materials.** Nothing delivers `Def.Extra` yet. Wiring it (frame supply in haul_route.go, `Frame.ExtraGot`, `FrameView.AX.Extra/ExtraGot/Need`) needs one owner. The proposal is A4 in wave 1; the lead decides.
- **Save version.** SaveVersion is 11, with an empty `case v < 11` in save.go `migrateSave`. A2 (1x1 table to 1x2) and A3 (stone-block floors to StoneTile, entrance carpets) send the lead their step.
- **Game package budget.** The package is at 53634 lines (budget lowered by 1). Pay for every added line, and put behaviour in sub-packages behind consumer-owned hosts.

## Per owner

### A1: orders and zones
- **Commands:**
  - DesignateUninstall, Reinstall (Target = crate stack; X, Y; Facing);
  - DesignateChopWood, DesignateSlaughter, DesignateClaim, DesignateStrip, DesignateOpen;
  - DesignateDeleteZone, DesignateRemoveFloor, DesignateClearRoofArea, DesignateClearNoRoofArea.
- **Orders:** OrderUninstall, OrderSlaughter, OrderOpen, OrderRemoveFloor, OrderChopWood.
- **Jobs:** Uninstall, OpenContainer, RemoveFloor. Chopping is Cut and stripping is Strip.
- **Item and state:**
  - item.MinifiedBuilding ("crated building"), with `Stack.Building`; Stuff, Quality and Wear ride along.
  - `Building.Uninstall`.
  - Views: BuildingView.AX.Uninstall and OpenMarked; ItemView.AX.Building.
- **Files you own:** your new sub-package (for example game/orders), your journey file `journey_ax1_test.go` and your handoff. Remove-floor refund rules are shared with A3, so agree them through the lead.

### A2: structure, furniture and decor
- **Kinds:** OrnateDoor, Column, AnimalFlap (Fabric), DoubleSleepingSpot, AnimalSleepingBox, Table2x2, Table2x4, Table3x3, Couch, PlantPot, SmallShelf, WallTorch, WallLamp, FloodLight, Urn (Unbuildable), LargeStele, GrandStele (StoneOnly), EggBox.
- **Command:** SetPotPlant (Target, Build = plant species).
- **Views:** BuildingView.AX.Seats, Plant, PlantPct, WallMounted.
- **Research:** FloodLighting (Advanced lights). Upholstery now also unlocks the ornate door, couch, small shelf and both steles. Electricity unlocks the wall lamp.
- **Your table:** construction/ax_a2.go.
- **Wall materials:** plasteel, uranium and jade need item stuff rows (item/stuff.go, lead) and wall art.
- **Tables:** the table 1x2 migration goes in the v11 step.

### A3: floors
- **Kinds (all LayerFloor):** StrawMatting (Only Hay), Concrete, PavedTile, SteelTile, SilverTile (Only Silver), GoldTile (Only Gold), SterileTile (Extra Silver), StoneTile and Flagstone (StoneOnly, 4 blocks per the lead's ruling), CarpetFloor (Only Cloth).
- **Colour:** `Building.Tint` and `Frame.Tint` hold the carpet colour. DesignateBuild carries it in `Command.Index`; route that in your exact tick.go edit. The colour palette goes in your own new file (for example construction/carpet_colours.go).
- **Views:** BuildingView.AX.Colour and FrameView.AX.Colour.
- **Research:**
  - SterileSurfaces (new);
  - Stonecutting: concrete, paved tile, stone tile;
  - Metalworking: steel, silver and gold tile;
  - CarpetMaking: carpet floor.
- **Your table:** construction/ax_a3.go.
- **Also yours:** room floor beauty and cleanliness values (room/tally.go Floor), and the v11 floor and carpet migrations, through the lead.

### A4: power and industry
- **Kinds:** WaterproofConduit (LayerConduit), ChemfuelGenerator, Watermill (5x6, Extra Steel), DeepDrill (spot), GroundScanner (spot, Extra advanced components), MoisturePump, ElectricCrematorium (Bench, StoneOnly, Extra Steel).
- **Jobs:** OperateDrill, ScanGround.
- **Views:** BuildingView.AX.Cycle, Ore, Radius, Flow; WorldView.AX.DeepOreVer and DeepOre ([]axview.Deposit).
- **Research:** WaterPower, Drainage, DeepDrilling, GroundScanning. Electricity unlocks the waterproof conduit, chemfuel generator and crematorium.
- **Your table:** construction/ax_a4.go.
- **Unconfirmed values:** the deep drill's hit points are the default 60 (no wiki figure); ask the lead if you need one.

### A5: fire foam and security
- **Kinds:** FoamPopper, IEDFoam ("foam charge", Only FoamShells), FoamTurret (Extra Chemfuel), IEDWarhead ("warhead charge", Only AntimatterWarhead), RocketBattery (Extra Plasteel and Chemfuel).
- **Items:** item.FoamShells, item.AntimatterWarhead.
- **Stain:** stain.Foam.
- **Views:** BuildingView.AX.Charges, ChargeCap, Armed.
- **Research:** FoamSuppression, FoamTurrets, RocketBatteries. Charges now also unlocks both new charges.
- **Your table:** construction/ax_a5.go.
- **Watch for range checks.** isIED (combat/ied.go:24), raids.go:603 and item.IsShell are range checks, so the new kinds fall outside them. Make those checks explicit sets. The explosion kinds (combat/explosion) are yours.

### A6: recreation
- **Kinds:** RingToss, UrBoard, ChessTable, BilliardsTable, PokerTable (Extra Cloth and Wood), TubeTV, FlatscreenTV, BigScreenTV (Unbuildable), Telescope (Unbuildable, spot).
- **Jobs:** Recreate, WatchTelevision, Stargaze (all IsIdle).
- **Activities:** Play, Television, Stargazing (gain 0 until you fill the gain table). JoyKind lives in recreation/joykind.go.
- **Views:** BuildingView.AX.Joy and Users; PawnView.AX.Joy.
- **Research:** Television, FlatScreens. Upholstery now also unlocks the chess, billiards and poker tables.
- **Your table:** construction/ax_a6.go.

### A7: books and bookcases
- **Kinds:** Bookcase, SmallBookcase.
- **Items:** item.Novel, Textbook, Schematic (IsBook, one per stack, filter Items/Books).
- **Job, order and activity:** job.Read, OrderRead, Activity Reading, JoyReading.
- **Command:** SetBookcaseFilter (reserved).
- **Views:** BuildingView.AX.ReadingBonus; ItemView.AX.Title, Teaches, Power; PawnView.AX.Reading.
- **Research:** Upholstery now also unlocks both bookcases.
- **Your table:** construction/ax_a7.go.

### A8: the escape ship
- **Kinds:** ShipBeam, ShipCasket, ShipCore (Extra includes item.MindCore), ShipReactor, ShipEngine, ShipSensor. All are fixed: not Movable.
- **Commands:** StartReactor, LaunchShip.
- **Job:** StartReactor.
- **Views:** BuildingView.AX.Countdown and ShipID; WorldView.AX.ShipLaunched, Ending, LaunchIssues.
- **Research:** ShipFrames, ShipStasis, ShipMinds, ShipReactors, ShipDrives, ShipSensors. Prerequisites past ShipFrames are [MEM].
- **Your table:** construction/ax_a8.go.

### A9: caskets, pods, scanner, hitching spot
- **Kinds:** StasisCasket, TransportPod, PodLauncher, MineralScanner (spot), HitchingSpot (free).
- **Commands:**
  - EnterCasket (Pawn, Target; Worker = the downed pawn to carry), EjectCasket;
  - LoadPods (Target, Name "id" / "id:qty" list, Enabled), LaunchPods (Target, Count = tile, Mode = PodArriveDrop/Gift/Visit);
  - SetScannerTarget (Target, Stuff).
- **Order:** OrderEnterCasket.
- **Jobs:** EnterCasket, CarryToCasket, LoadPods, ScanMinerals.
- **Views:** PawnView.AX.Casket; BuildingView.AX.Sleeper, SleeperID, LoadKg, CapKg, FuelUnits, Range, ScanFor, Group; WorldView.AX.Pods and Sites.
- **Research:** Stasis, TransportPods, MineralScanning.
- **Your table:** construction/ax_a9.go.

## Names (ENG-003: original where RimWorld coined the term)

| Vanilla item | Our name |
|---|---|
| Wall torch lamp | wall torch |
| Megascreen television | big screen television |
| Hoopstone ring | ring toss |
| Game of Ur | ur board |
| Firefoam popper | foam popper |
| IED firefoam trap | foam charge |
| IED antigrain warhead trap | warhead charge |
| Antigrain warhead | antimatter warhead |
| Rocketswarm launcher | rocket battery |
| Cryptosleep casket | stasis casket |
| Ship cryptosleep casket | ship stasis casket |
| Persona core | mind core |
| Minified thing | crated building |

The carpet floor's name, "carpet floor", is distinct from the entrance "carpet".

## Verification
- `go build ./...` and `go vet ./...` are clean.
- `go test ./internal/arch/` passes.
- Targeted `-race` tests pass: the domain packages, `application/...`, ebiten windows, architect, inspector and wizard, and the outbound adapters.
- Determinism gate: every pin holds except `TestGrownGolden` (application/scenario), which moves from 0x753367ca3d4f612e to 0xef4cef40a35e4444 (stable over `-count=2`).
  - As with the FOOD-0 contract, the sandbox's CompleteResearch now also completes the 20 new projects, and new stockpile filters gain the bits of the 8 new item kinds.
  - With both of those hidden, the old pin comes back exactly.
  - The lead re-pins it.
- View equality now covers AX: `equalBuildingView`, `equalFrameView` and `sameItemView`. The reflective view tests step into nested structs, and the supported save versions run to 11.
- New tests:
  - construction/ax_contract_test.go (kinds, the Planned gate, stuff rules, hit points, Movable);
  - game/ax_contract_test.go (no blueprint while Planned, AX projects unlock only Planned kinds, zero views, v11);
  - power/ax_test.go.
- Docs: RESEARCH_GAP_ANALYSIS rows now map to the new projects. No art, Architect or checklist change: every gap row stays missing until its owner ships it.
