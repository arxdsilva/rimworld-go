# SIM-023 Biomes: twelve biomes, new ground and wild plants

## What changed
- **Contract** (`internal/domain/world/biome/biome.go`): 12 biomes (temperate forest, boreal forest, tundra, cold bog, ice sheet, sea ice, temperate swamp, tropical swamp, tropical rainforest, arid shrubland, desert, extreme desert). Each row has key, name, blurb, difficulty label, `temperature.Climate`, wetness, ground mix, wild plant densities, grass density and regrowth, and wildlife density.
- **Terrain** (`world/terrain`): new sand, marsh and ice. Walk cost: marsh ×2, sand ×1.2, ice ×1.3.
- **Plants** (`world/plant`): pine, jungle tree, cactus, shrub, reeds. They die out of their climate band (`plant/grass.go` grass density and regrowth are per biome, regrowth by heap).
- **Generation** (`world/generation/generation.go`, `application/game/world.go`, `biome_info.go`): the world is built from seed and biome. Temperate forest keeps the original generator, locked by `biome_golden_test.go`.
- **Climate and weather** (`application/game/climate.go`, `weather`): each biome supplies its own temperature profile and wetness.
- **Entry points:** `game.Biomes()` lists `BiomeInfo` for the UI; dev flag `-biome`; `AppConfig.DefaultBiome`.
- **Saves:** the biome is saved with the world. Older saves load as temperate forest.

## Rules
- Numbers are our own tuning, not copied from any source.
- Biome choice is direct. There is no world map or tile choice.

## Verification
- Tests: `biome_test.go`, `generation/biome_test.go`, `plant/biome_grass_test.go`, `weather/wet_test.go`, `temperature_test.go`, application `biome_golden_test.go`, `biome_world_test.go`.
- Gates: `go build ./...`, `go vet ./...`, `go test ./...`.

## Known limitations
- No world-map tile picker; the biome is chosen in the wizard (see UI-014).
- No disease frequency, forage, topology or hilliness per biome.
- Biomes are Go data, not data-pack files.
