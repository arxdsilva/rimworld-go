# craft package

## Package Purpose
Production: every bench's bills and the bill commands, finding and doing bill work (fetch, carry, work, product), drying racks, the tool-cabinet bonus and the electric benches. A consumer sub-package of `internal/application/game` (`.cursor/SOLID_GO.md` §10), created by PRD-0; the gap backlog is `.cursor/PRODUCTION_GAP_ANALYSIS.md`. Later phases (PRD-1…4) add multi-ingredient recipes, bill settings, quality, stuff choice, art and deterioration hooks here.

## Owned Concepts
- `Craft` (craft.go): the root, built by `New(Host)`; components as exported fields; `ElectricBench`, `ElectricPct`, `ElectricSpeedPct` (PLAY-021, also used by game's research bench). Kept small: a feature's entry points go on its component, never here.
- `Bills` (bills.go): bills per bench; `Apply` (AddBill/SetBill/RemoveBill, and PRD-1's ConfigureBill/SetBillIngredient/ReorderBill in settings.go), `Defaults` for a new bench, `Forget`, `AnyFor` (doctoring bills), `UpgradeLegacy` (older saves' butcher bills), `Line` (one bill's up-to-date copy and status) and the `Bill*` status strings; saved as the map itself.
- `Shop` (shop.go): `Assign` (the first wanted bill at the first bench in building order, honouring the bill's worker, skill range and ingredient retry) and `Do` (one tick of a bill job: fetch, work, finish, then the store mode).
- Ingredient search (gather.go, PRD-1): `planner` takes each input from the stacks nearest the work spot (radius, forbidden kinds, reservations), several stacks per input; `IngredientRetry` is the cooldown after a failed search (`recipe.Bill.SearchAt`, saved with the bill).
- Gathering (haul.go, PRD-1): `job.HaulIngredients` carries its plan in the job record (`Route`: stack id, count pairs; `Count`; loads laid on the bench appended) and turns into a plain `job.Bill` with the last load in hand; the bill's finish deletes the laid loads too.
- Read model (view.go): `RecipeView`, `BillView`, `BillIngredients` (a comparable allow list; `NewBillIngredients` for tests), `Recipes`, `Bills.AppendViews` with the settings, current worker, progress and `NoWorker*` reasons. game aliases the types.
- `Drying` (drying.go): what each rack holds (`Batch`, saved), `Advance` on the climate tick, `Empty` when a rack is torn down.
- `Facilities` (facility.go, PRD-3): the bench facilities table (tool cabinet: +6 % each, two at most, reach 6), the bonus and the linked names per bench (`BuildingView.Linked`), cached per `StructVer`, never saved. Spots link nothing.
- `Stations` (station.go, PRD-3): bench work speed in permille (`Speed`, `BuildingView.WorkSpeed`): the bench's own factor (crafting spot 0.5, butcher spot 0.7), facilities, an electric twin's +40 %, times light (0.8 in the dark, ramp to 30 %) and the room (cleanliness bands 0.85/0.9/0.95/1, outdoors 0.8); cached `speedEvery` ticks. `View` fills a bench's links and speed and a sculpture's title (in `Epitaph`).
- `Art` (art.go, artgen.go, PRD-3): sculpting bench bills set a sculpture down beside the bench (`Workshop.CanInstall`/`Install`), quality rolled, title and description grown from a seed on the craft stream with original word lists; saved as `Game.Art` (`map[id]craft.Artwork{Seed, Maker}`). No minified items: the player moves a sculpture with the Move order.
- Forge fuel (PRD-3): `Fueled` (campfire, wood-fired forge); the forge works only with fuel and burns one log per `WorkPerWood` work (`Workshop.UseFuel`); refuelling is the game's shared fuel table (`fuel.go`).
- Fermenting barrels (ferment.go, PRD-4): the barrel's fill bill (`recipe.FillBarrel`, `Def.Ferment`) pours wort in up to `recipe.BarrelWort`; `Drying.Advance` ferments it (6 days between 7 and 32 C, paused from -1 to 7, ruined after `RuinTicks` outside -1..32; wiki) and sets the beer on the spot. A barrel takes more wort only while under `TopUpPct` fermented (no flap: `TestBarrelFillNoFlap`). The batch shares the saved rack map (`Game.Racks`: Kind wort, Left the ticks to go, Rot the ruin ticks), so `BuildingView.DryKind/DryQty/DryPct` show it.
- PRD-4 recipes: the better meals roll the cook's poison (`recipe.Cooked`); smelting apparel (`Input.SmeltWear`); shredding a mechanoid wreck for its type's salvage (`Def.Salvage`, `recipe.SalvageOf`); butcher tables get no default kibble bill.
- `Action` (action.go): the action word for bill work at a bench (game's `billAction`).
- Stack moves (stockops.go): `driedStock`, `merge`, `putDown` over the host's plain stack reads and writes.

## Allowed Dependencies
- Domain value types (`sim/entity`, `sim/command`, `ai/job`, `ai/work`, `ai/pathing`, `economy/recipe`, `economy/item`, `economy/quality`, `building/construction`, `building/power`, `pawn/skill`, `animal`).
- Nothing from `internal/application/game`: the roles in `deps.go` are the only way in.

## Forbidden Dependencies
- `internal/application/game` (the guard's `imports` rule), adapters, platform, Ebitengine, global RNG, wall clock.

## Public Contracts
- `deps.go`: `Host` embeds value-only roles (Benches, Stock, Output, Workers, Crafters, Makers, Weathering, Workshop), each ≤ 8 methods (Benches, Workers and Crafters are full: a new need is a new role; PRD-3 moved DriedStock, Merge and PutDown into craft). The game implements them with adapter types in `craft_host.go`; none adds a method to `game.Game`. Fuel (`fuel.go`, shared with lamps and generators) and bench work speed (`workProgressPct`, skills.go) stay in game behind `Benches.Fuel` and `Crafters.BenchWork`.
- The read model types live here (view.go); game re-exports them as aliases (`game.BillView`, `game.BillIngredients`, `game.BillIngredientView`, `game.RecipeView`, `game.Recipes`, `game.Bill*`), and `BillView` stays comparable (building views are diffed with `slices.Equal`).
- Saved state keeps its gob names: `Game.Bills` (`map[EntityID][]recipe.Bill`), `Game.Racks` (`map[EntityID]craft.Batch`, fields `Kind`, `Qty`, `Rot`, `Left`) and `Game.Art` (`map[EntityID]craft.Artwork`, fields `Seed`, `Maker`; only standing sculptures are exported); a nil field keeps what is there, so older saves load.
- `RecipeView.Benches` lists every bench kind running a recipe (`recipe.Def.At`: Bench, Also, electric twins); `RunsAt(kind)` is the UI's check.

## Invariants
- No randomness but the host's craft stream (`Makers.Roll`: quality and art seeds; a meal's poison is rolled by the host in `Output.DropMeal`), no wall clock.
- Maps are ranged only for order-free work (`AnyFor` existence check, `UpgradeLegacy` per bench); everything ordered walks the host's building or pawn id slices.

## Tests
- Unit tests run against `fakeHost` (host_test.go); coverage ≈ 99 % at PRD-0.
- Whole-game tests (production, electric benches, drying, burial, carpets, stone, save round trips, PRD-1 `bills_settings_test.go`) stay in `internal/application/game`. No-flap: `TestNoIngredientsRetriesAfterACooldown` (a bill with nothing to gather searches once per `IngredientRetry`).

## Hot Paths
- `Shop.Assign` (every idle cook/crafter/doctor) and `Shop.Do` run on the tick path; the multi-stack search runs only when the single nearest stack won't do and reuses `Shop.buf`; `Bills.AppendViews` runs on every view build and allocates nothing for a single-input recipe; `Drying.Advance` every climate interval. Host calls pass values only; `Benches.Bench` copies a building only when it is a bench, so the bench scan copies no other building.
