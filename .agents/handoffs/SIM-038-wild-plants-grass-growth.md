# Task Handoff

Task ID: SIM-038 (PLAY-010)
Status: complete

## Summary
Seven new wild ground plants (our names and numbers): bramble thicket (walk cost 150%), ground moss, desert rosette (agave), ball cactus, elephant ear (110%), fire lily, dandelions. They are placed per biome, browsable by grazers (not the cactus types), never block building and are cleared by it. Grass now grows: a grazed or cut tile comes back as shoots (15%) and fills out to 100% over about three days at 15 °C; snow or frost pauses it. Grazers prefer fuller grass and get a meal scaled by growth (at least 30%); tiles under 30% are left alone unless nothing else is near.

## Files
- `domain/world/plant`: defs (`WalkCost`, `Ground`), grass growth rules.
- `domain/world/biome`: plant lists. Rainforest jungle trees 95‰ and berries 25‰ so the total stays in its 140–180‰ band.
- `game/grass.go`: lazy growth layer with per-chunk lists, grown in `growChunks`; `WorldView.GrassGrowth` and `GrassVer` (a content fingerprint, so a loaded game reads the same version); saved as `GrassGrowth`.
- `animal_needs.go` grazing; `weather.go`/`water.go` plant walk cost; `plantLabel`; biome picker.

## Notes
- Bramble cost is felt in step time and in costed replans; the first path plan has no per-cell cost (adding one would move every golden), so routes don't avoid brambles until they replan.
- Five `goldenLegacy` site hashes re-pinned (plants only); `TestTemperateGenerationUnchanged` holds.
