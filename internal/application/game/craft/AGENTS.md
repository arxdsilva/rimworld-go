# craft package

## Package Purpose
Production: every bench's bills and the bill commands, finding and doing bill work (fetch, carry, work, product), drying racks, the tool-cabinet bonus and the electric benches. A consumer sub-package of `internal/application/game` (`.cursor/SOLID_GO.md` §10), created by PRD-0; the gap backlog is `.cursor/PRODUCTION_GAP_ANALYSIS.md`. Later phases (PRD-1…4) add multi-ingredient recipes, bill settings, quality, stuff choice, art and deterioration hooks here.

## Owned Concepts
- `Craft` (craft.go): the root, built by `New(Host)`; components as exported fields; `ElectricBench`, `ElectricPct`, `ElectricSpeedPct` (PLAY-021, also used by game's research bench). Kept small: a feature's entry points go on its component, never here.
- `Bills` (bills.go): bills per bench; `Apply` (AddBill/SetBill/RemoveBill), `Defaults` for a new bench, `Forget`, `AnyFor` (doctoring bills), `UpgradeLegacy` (older saves' butcher bills), `Line` (one bill's up-to-date copy and status for the read model) and the `Bill*` status strings; saved as the map itself.
- `Shop` (shop.go): `Assign` (the first wanted bill at the first bench in building order) and `Do` (one tick of a bill job: fetch, work, finish).
- `Drying` (drying.go): what each rack holds (`Batch`, saved), `Advance` on the climate tick, `Empty` when a rack is torn down.
- `Cabinets` (cabinet.go): the tool-cabinet bonus per bench, cached per `StructVer`, never saved.

## Allowed Dependencies
- Domain value types (`sim/entity`, `sim/command`, `ai/job`, `ai/work`, `ai/pathing`, `economy/recipe`, `economy/item`, `building/construction`, `building/power`, `pawn/skill`, `animal`).
- Nothing from `internal/application/game`: the roles in `deps.go` are the only way in.

## Forbidden Dependencies
- `internal/application/game` (the guard's `imports` rule), adapters, platform, Ebitengine, global RNG, wall clock.

## Public Contracts
- `deps.go`: `Host` embeds five value-only roles (Benches, Stock, Output, Workers, Crafters), each ≤ 8 methods. The game implements them with adapter types in `craft_host.go`; none adds a method to `game.Game`. Fuel (`fuel.go`, shared with lamps and generators) and bench work speed (`workProgressPct`, skills.go) stay in game behind `Benches.Fuel` and `Crafters.BenchWork`.
- The read model stays in game (`game.BillView`, `game.RecipeView`, `game.Recipes`, `game.Bill*` aliasing craft's strings): game builds `BillView`s from `Bills.Line`.
- Saved state keeps its gob names: `Game.Bills` (`map[EntityID][]recipe.Bill`) and `Game.Racks` (`map[EntityID]craft.Batch`, fields `Kind`, `Qty`, `Rot`, `Left`); a nil field keeps what is there, so older saves load.

## Invariants
- No randomness (a meal's poison is rolled by the host in `Output.DropMeal`), no wall clock.
- Maps are ranged only for order-free work (`AnyFor` existence check, `UpgradeLegacy` per bench); everything ordered walks the host's building or pawn id slices.

## Tests
- Unit tests run against `fakeHost` (host_test.go); coverage ≈ 99 % at PRD-0.
- Whole-game tests (production, electric benches, drying, burial, carpets, stone, save round trips) stay in `internal/application/game`. No threshold-driven behaviour here yet, so no no-flap test.

## Hot Paths
- `Shop.Assign` (every idle cook/crafter/doctor) and `Shop.Do` run on the tick path; `Drying.Advance` every climate interval. Host calls pass values only; `Benches.Bench` copies a building only when it is a bench, so the bench scan copies no other building.
