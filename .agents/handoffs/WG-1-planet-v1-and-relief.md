# WG-1: planet v1 domain and the globe relief

Closes the domain half of `.agents/gaps/WORLDGEN_GAP_ANALYSIS.md` and the relief half of the globe look.

## What changed
- **Versioning (WG-17).** `planet.CurrentVersion = 1`; the wizard sets it (`newgame_state.go`, `screen_worldgen.go`). Every generator change is gated on `Version >= 1`; v0 worlds rebuild as before (`TestV0WorldUnchanged`), and `TestPlanetGoldenHashV1` pins v1.
- **Biomes (WG-02).** `scoreBiome` (`planet/biomes.go`): wiki thresholds first, original score second. `TestBiomeSharesV1` (shrubland at most 20 %, temperate forest at least 15 %, rainforest at least 6 %), `TestScoreBiomeInsideWikiThresholds`.
- **Movement (WG-03).** `Planet.MoveDifficulty(tile, day)` in `planet/travel.go` from `biome.Def.MoveBase/MoveWinter` plus hills and roads (`TestMoveDifficultyWiki`). The wizard's invented formula is gone.
- **Biome data.** `biome.Def.Forage`, `DiseaseMTB`, `MoveBase`, `MoveWinter` (`// wiki: Biomes`). WG-05 is wired: `biome.DiseaseOdds(b)` scales the disease percent in `natureOdds`, temperate forest = 100 % (`TestNatureOddsFollowTheBiome`). The plan's "replace today's constant" was wrong: disease was weighted only by difficulty frequency. Disease weight now moves with biome, so any golden that rolls a disease can move; the lead re-pins (not checked here).
- **Queries** (`planet/travel.go`, `features.go`, `names.go`, `roadclass.go`, `ext.go`): `TimeZone`, `QuadrumTemps`, `NearbyBases(+With scratch)`, `Features`, `FeatureAt`, `Find`, `Name`, `RandomLandingFiltered`, `RoadClassOf` (path, dirt, stone, asphalt, highway; ancient network), `HugeRiver` (appended; mapgen width row and tests). `TestRoadClassesPresent`, `TestFeaturesNamedAndFindable`, `TestRandomLandingHonoursFilter`, `TestTimeZoneAndQuadrums`, `TestNearbyBasesAndWarning`, `TestHopsWithConcurrent`.
- **Relief (WG-01, WG-14).** `globe/relief.go`, `frame.go`: per-planet `planetGeom`, corner heights, peaks, faceted lighting, snow caps and bare rock by height, soft borders, `ReliefMax`, shallow band. `mountain_tint` is gone from `world.yaml` (palette gains `snow`, `rock`, `shallow`, `shore`, `grid`, `hill_*`, `label*`). NG1 section 1.2 and ART section 18 text updated.
- **Budget.** `globe.View` fields 51 to 34; `planet.Planet` 33 to 30. `BenchmarkGlobeFrame` is the gate.

## Wiki versus original
Wiki: thresholds, movement bases and hills, forage, disease MTB, winter endpoints, roads x0.5. Original: scoring, winter curve shape, relief scale, snowline (2600 m + 120 m per C), huge-river flow 30, road class placement, feature names.

## Gaps
- Domain C sentences are converted at their sinks (see WG-2).
