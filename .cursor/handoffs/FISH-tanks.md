# FISH: fish tanks

Fish farming, asked 2026-11-02 "the way RimWorld would do it". Vanilla has no fish farming (Odyssey has a fishing zone), so this is ours, built from vanilla parts: the building behaves like a hydroponics basin, the work like the Fish job, the item like meat. Spec `.cursor/FISH_TANK_PLAN.md` §3.

## What changed
- Domain: `domain/building/aqua` (species, growth, health); `item.Fish`, `construction.FishTank`, `research.Aquaculture`, `job.StockTank`, `job.NetTank`.
- Simulation: `game/aqua`, one `*aqua.Tanks` per map on `mapState`, stepped hourly from `burnFuel`. Feed runs through the refuel pipeline. Saved; nil while no tank exists, so goldens hold.
- `assignRefuel` skips a burner with no fuel to hand instead of ending the search, or an empty torch starved tanks behind it.
- UI: Architect card, inspector stock picker and lines, "Fish dying" alert, sprites with a powered glow and three growth stages. Read model `BuildingView.Tank`. `command.SetTankStock`.

## Numbers
The building is the wiki's hydroponics basin (ENG-003). Stocks, research cost, health rates and yields are original tuning.

## Choices
Removing or moving a tank drops its state: ripe fish land at Animals-0 yield, growing fish are lost, a moved tank has no stock. Changing stock empties it the same way.

## Tests
`domain/building/aqua`, `game/aqua`, `fish_tank_test.go` (whole cycle twice, same hash; save round trip; no-flap), `journey_fish_test.go`.

## Gaps
- No fishing zone: Odyssey content, out of scope.
- "Fish dying" has a unit test but no journey: killing a real tank needs a generator, a stocking hand and a cut.
- The whole-cycle test advances the clock through `burnFuel` for the grow time, since real days are too slow under `-race`.
