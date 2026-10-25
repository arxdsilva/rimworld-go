# Task Handoff

Task ID: SIM-050 (PLAY-013, MAPGEN-2)
Status: complete

## Summary
New-game (landing-site) maps come from a staged, field-based generator: `internal/domain/world/mapgen`. Continuous fields come first: warped multi-scale elevation with a mountain mass leaning uphill, fertility, wetness and broad stone regions. Then landforms: rock by elevation share per hilliness, tapered branching caves, coasts, meandering rivers with banks, floodplains, canyons and fords, and lakes. Then the discrete map: terrain by precedence with biome patches and patch cleanup, ore lumps, stone debris near cliffs, A*-routed and smoothed roads, a scored landing spot, small ruins, steam vents, and clustered vegetation and grass. Validation with local repair runs last. Each step has its own named random stream (`randx.Derive`), so changing one pass never moves another. The legacy generator (`generation.Generate`/`GenerateBiome`/`GenerateSite`) is frozen and still serves legacy colonies, scenarios and pins. Plan: `.cursor/PLAY013_PLAN.md`.

## Files
- `pkg/noise2`: hash-based 2-D gradient noise, fBm, ridged, domain warp. `pkg/randx.Derive`/`Float64`.
- `mapgen`: types, `Ctx`, grid helpers.
- `mapgen/pipeline`: order, labels, `Builder`, `Generate`, profile tests and benchmarks.
- `mapgen/steps/{landform,ground,water,resources,roads,start,features,vegetation}`: one package per area.
- `site.Site`: `Uphill`/`HasUphill`, `Rain`, `CoastNear`, `Settled`, filled by `Planet.Site`.
- `cmd/mapgen`: a PNG per field, `-grid N` contact sheets, `-sweep hills`, `-tile seedText:id`, `metrics.json`.
- Game: `setup.go` runs one loading phase per step. `setup_mapgen.go` `applySite` lays terrain, geology, thin/thick roofs, ruins, vents, chunks, plants and grass. The landing spot is saved (`saveState.Landing`) and used for arrival, Home, wildlife and messages. `Setup.GenVersion = mapgen.Version` (2).
- Wildlife: `wildlife_habitat.go`. Herds start in reachable open land 25+ tiles from the landing spot; arrivals walk in from a reachable edge; no domestic or pet species spawn wild (Setup games only).
- Dev: the "Map fields" overlay (`mapfields.go`, `mapfields_overlay.go`) regenerates the fields for the colony's own generator version.

## Numbers (ours, tuned in cmd/mapgen)
- **Rock share:** Flat 4%, Small 10%, Large 22%, Mountainous 42% (±25%). Rough-stone band 4%, gravel band 6%.
- **Mountain bias weight:** 0 / 0.08 / 0.18 / 0.38.
- **Caves:** in rock bodies ≥ max(200, 450·Scale²) cells, one open tunnel per 900 rock cells (up to 5). Width 4.5–7 tapering by 0.03 per step to 1.3, up to 40° per 6 steps of turning, branches 9% per step.
- **Rivers:** meander 0.16–0.24 of their length; widths 3/7/12·√Scale; fords at least every 86 cells.
- **Coasts:** 14–40·Scale wide; shoreline wobble 14–24·Scale.
- **Debris:** chunk chance 0.003 × density; density 1.6 near rock and 0.2 in open fields.
- **Ore:** budgets as in geology; origins ≥ 6 apart; gold ≥ 5 deep.
- **Landing spot:** ≥ 70% open land within 12, nothing wet or rock within 4.
- **Reachability:** ≥ 70% of soft land from the landing spot.

## Performance
Whole pipeline 0.30 s at 500² and 0.08 s at 250² (budgets 2.0 s / 0.6 s).

## Notes
- No reference constants were copied; the techniques are generic procedural generation.
- Setup-game state hashes include the new map; legacy and scenario goldens did not move. The scenario-kit test subtracts ruin loot and debris.
- Not done: road re-routing in validation, the moss-in-damp-caves exception, sealed "ancient danger" rooms (by decision), bridges.
- The steam vent has no power use: there is no power system yet (`PowerReady` is future work).
