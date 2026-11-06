# Task Handoff

Task ID: PLAY-005
Status: complete (phase 1 of the resources/food/production/rooms/combat plan)

## Summary

Seeded map generation (terrain + plants), terrain in the simulation,
plant species with growth and regrowth, Cut plants and Mine orders,
stone blocks, and stone as a building material for walls and floors.
Renderer draws terrain with soft transitions, natural rock walls,
growth-scaled trees, cut/mine designations; Architect gains Cut/Mine
tools and a Wood/Stone material picker; Work panel gains Mine (and wires
Cook/Hunt flags for later phases). Default map is 48x40.

## Public Contracts Changed

- domain: `terrain` package, `generation.Generate`, `plant.Plant` with
  species defs (`Harvestable()` is now a method), `item.Stone`,
  `work.Mine`, `construction.Frame/Building.Stuff`, `StuffOK`, `WorkFor`.
- commands: `DesignateCut`, `DesignateMine`, `Command.Stuff` (0 = wood),
  `OrderCut`, `OrderMine`, `WorkMine/WorkCook/WorkHunt`.
- read model: `WorldView.Terrain/TerrainW/Mines/Stone`, `PlantView.Species/Growth/Stage/Cut`,
  `FrameView/BuildingView.Stuff`, `PawnView.WorkMine/Cook/Hunt`,
  `game.TerrainNames()`, `game.StuffID()`, `Game.CanPlaceStuff`.
- `NewColony` now generates the map and centres the start kit.

## Invariants / Decisions

- Generation uses its own RNG stream derived from the seed; same seed, same map.
- Rock tiles are terrain, not buildings; mining turns them into stone ground.
- Trees block building on their tile (cut first); bushes/crops are cleared by completed buildings.

## Verification

go vet, go test -race ./... pass. New tests: terrain, generation
determinism/variety/clear start, plant growth/fertility, chop wood,
mine stone, stone wall, berry regrow, tree blocks building, OrdersAt
chop/mine, cancel clears cut/mine, UI stone material + plant sprites.
In-game: `assets/generated/ingame_world.png` (zoom 0.5).

## Known Limitations

No water/ore, no terrain movement cost, mining speed has no skill.
