# PRD-4: recipe catalogue, stockpile filters, shelves, barrel

Date: 2026-10-29. Wave E, agent P4. Backlog `.cursor/PRODUCTION_GAP_ANALYSIS.md` (3.2, 3.4, 3.5).

## What changed
- **Recipes** (`domain/economy/recipe/wave_e.go`, rows appended to `recipe.go`): 31 ids. Stove: fine, lavish, pemmican, survival meal. Butcher table and spot: kibble, 50 and 35. Brewing bench: wort. Barrel: fill. Drug table: ash flake, silverdust, battle tonic, reagent. Refinery: chemfuel from wood or raw food. Fabrication bench: advanced components, marine armor and helmet. Machining table: flak vest, pants and jacket, the advanced helmet, frag grenades, molotovs, four mortar shells, shred mechanoid. Smithy: plate armor, simple helmet. Smelter: smelt metal apparel.
- **Recipe fields:** new `Input` flags `AnyVeg`, `AnyMeat`, `SmeltWear` and `Wreck`. Butchering never takes a wreck. New `Def` flags `Ferment` and `Salvage`. Added `recipe.Cooked` and `SalvageOf`.
- **Barrel** (`craft/ferment.go`): fermentation happens in `Drying.Advance`. The barrel's batch shares the saved rack map (`Game.Racks`), so no save field was added and `BuildingView.Dry*` shows it. `FermentingBarrel` is now `Bench: true`, so it has a fill bill.
- **Research** (`research.go`): 8 new projects with original names: TrailRations, SealedRations, LeafRefining, TonicSynthesis, PrecisionFabrication, FlakArmor, Armoursmithing, PoweredArmor. Gates were added to Upholstery (`build:shelf`), Metalworking, RifledBarrels, Microcircuits and Artillery.
- **Stack limits** (`item.Kind.StackLimit`): the wave E kinds carry their wiki limits. Hauling takes at most that many per trip, and merging and spilling stop at the limit. The older kinds keep 1000, because shrinking them changes every stockpile and every pinned game.
- **Filter tree** (`domain/building/zone/filter.go`): new `Filter` bitset type. The tree has 8 top categories with sub-categories. Also new: `Resolve`, `Names`, `Rows`, `Special` with toggles and hit-point and quality ranges, `SetAllowed`, `PasteFrom` and `NewShelf`. `Zone` gains `Custom`, `Filter` and `Special`, all zero-safe: until the first edit, a zone goes by its old switches, so old saves and hashes are unchanged.
- **Game:** `allows` became `zone.Zone.Allows`, and the old stockCat table moved to domain. `cmdStockpile` replaces the three no-op cases in tick.go. `storageCell`, `haulDest` and `staysStored` take a stack, so they honour special filters. `ZoneView.Filter`, `Special` and `Shelf` are filled. `game.StockpileFilterRows` and `game.StockpileSpecials` are new.
- **Shelf:** a `zone.KindShelf` storage zone over the footprint, laid in `zoneBuilt` and lifted in `removeBuildingAs`.
  - A moved shelf keeps its settings.
  - The zone tools leave shelf zones alone.
  - A shelf cell holds 3 stacks (`cellFull`, `shelfLoads`).
  - Defaults are Preferred priority and general items (no chunks, no corpses).
  - Items on a shelf don't deteriorate (the `Exposed` adapter).
  - Beauty is 0.5 (`construction/furniture.go`).

## Wiki numbers versus original tuning
- **Wiki:** meal inputs, skills and yields; kibble 50/35; wort 25 to 5; barrel 25 wort, 6 days, 7..32 C, paused from -1 to 7; drug leaf counts and work; chemfuel 70 wood or 3.5 nutrition to 35; recipes for advanced components, armour, grenades and shells; the shelf's 3 stacks, 0.5 beauty and Preferred default; shred mechanoid at 300 ticks with R2's per-type salvage; stack limits.
- **Scaling:** cooking work x0.4 (matching the simple meal); bench materials /10 rounded up and work /10 (as in PRD-3); the drug table uses wiki ticks.
- **Original:** reagent recipe (4 chemfuel + 2 ashleaf), barrel ruin after 1 day, the 10 % top-up gate, fill work 30, the advanced helmet using the flak helmet's recipe, project names and costs, the tree's names.

## Hysteresis
A barrel takes wort only when empty or under 10 % fermented, then stays closed until it is emptied. `TestBarrelFillNoFlap` checks it.

## Tests
- `zone/filter_test.go` (coverage 96.6 %)
- `recipe/wave_e_test.go` (98.3 %)
- `item/stack_limit_test.go` (89.7 %)
- `research/prd4_test.go`
- `craft/prd4_test.go` (97.6 %)
- `game/stockpile_prd4_test.go`
- Kind-less test zones in `stone_test.go` and `water_test.go` now say `Kind: KindStockpile`.

## Gaps
- Bill quality and HP ranges.
- Weapon quality on equip; `Wealth()`.
- Shelves have no Architect row or art (U5, A4).
- Haulers fill the barrel as cooks; the wiki uses haulers.
- The older kinds keep the 1000 stack limit.
- A stack refused everywhere by special filters marks its kind "full" for one haul search.

## Integration notes (lead, wave E)
- Journey fixes: the stockpile journey opens Settings, then expands Materials; the shelf journey counts both shelf cells. `TestJourneyStockpileFilterEdit`, `ShelfStoresStacks` and `CraftFineMeal` pass.
- The wave E contract inertness test became `TestDraftDefaults`.
- `TestGrownGolden` re-pinned (wave E reason). Game budget lowered to 54474 lines, 180 files, 1935 `Game` methods. Full `-race` suite green.
- `BenchmarkDrawArt500` unchanged against wave D (407/548/266 draw calls, 3228/2983/2223 allocations).
- Gaps closed since the list above: the shelf now has an Architect row (furniture) and art (U5, A4); the storage window (`windows/storage.go`) edits filters, specials and copy and paste.
