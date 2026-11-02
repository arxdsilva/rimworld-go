# World generation and globe gap analysis (WG-0..WG-3)

Scope: the Create world and Select starting site screens (the planet, its biomes, rivers, roads, named features, tile inspector, search, filters, units, seeds) and how the globe is drawn, compared with core vanilla RimWorld 1.5 (no DLC). Load only for WG-* tasks. Phase handoffs go in `.cursor/handoffs/WG-N-*.md`. Sister document for what happens after the globe: `MULTIMAP_GAP_ANALYSIS.md`. Sources: rimworldwiki.com pages Biomes, World, World generation, Caravan, Faction base, Time zone (2026-11-02). Names and prose here are ours; the wiki fixes only numbers and which features exist.

## Decisions

| Item | Decision |
|---|---|
| Numbers | ENG-003 is extended to WG-* (approved with the plan, 2026-11-02; the `AGENTS.md` ENG-003 line is pending the user): published wiki numbers are tuning data, names and prose stay original. Each constant gets `// wiki: <page>`, or `// original tuning` where the wiki publishes nothing. |
| Original tuning (wiki silent) | Biome scoring inside the thresholds, river widths and the huge-river flow, winter movement curve shape, relief exaggeration, snowline, road class placement, feature naming. |
| Excluded | Pollution (Biotech), landmass and mountain sliders, landmarks and tile mutators (Odyssey; not on the wiki). Listed as `n/a (DLC)`. |
| Stability | New worlds set `planet.Params.Version = 1`; every generator change is gated on `Version >= 1`, so v0 worlds (old saves) rebuild identically. |
| Look | The globe look is normative here (section "How the globe is drawn") and replaces the "vertex colours only" line in `ART_DIRECTION_AND_ASSET_PIPELINE.md` section 18 and NG-1 section 7.3. The style stays original. |

## Phases

| Phase | Delivers | State |
|---|---|---|
| WG-0 | This analysis and the contract (`handoffs/WG-0-gap-analysis-and-contract.md`) | done |
| WG-1 | Domain: v1 params, biome scoring, `MoveDifficulty`, forage and disease per biome, time zone, road classes, huge river, features, search, filtered random landing, nearby bases, quadrum temperatures, planet name; globe relief, lighting, snow, soft borders, shallow band (`handoffs/WG-1-planet-v1-and-relief.md`) | done |
| WG-2 | Globe lines and labels, `worldinfo` tile panel, search, filter and legend, planet name header, temperature unit, journeys (`handoffs/WG-2-globe-lines-and-landing-ui.md`) | done |

## Items

Columns: id, vanilla behaviour (wiki page), ours today (the state before WG-1, evidence verified against the code on 2026-11-02), gap, fix (with what shipped), phase, status (current).

| ID | Vanilla (wiki page) | Ours today (evidence) | Gap | Fix | Phase | Status |
|---|---|---|---|---|---|---|
| WG-01 | Globe reads as terrain: mountains and hills stand out, coasts are crisp (World) | Flat colour per tile; hills only tint toward grey (`ebiten/globe/globe.go:477-481`); one fixed light on the view-space normal (`globe.go:177-200`, `globeSun` and `Shade`); no relief, no coastline | Hills and mountains are not visible as shapes; no shore | Relief rendering, see "How the globe is drawn" | WG-1 | done |
| WG-02 | Biome picked within wiki thresholds (Biomes): rainforest 15 C and 2000 mm or more; temperate forest -10 C and 600 mm or more; desert 0-600 mm; extreme desert 4.82 C and 340 mm or less; tundra 0 C or less; ice sheet -10 C or less; sea ice -22 C or less | `Classify` (`domain/world/planet/biomes.go:104-135`): every tile with rain from 250 mm that misses the earlier rows falls through to arid shrubland (`case r >= 250`, `:125`), about 40 % of land; the boundaries (-17, -5, 5, 18 C, 550/1900 mm) are not wiki | Shrubland swallows the warm band; temperate forest and rainforest too rare | v1 branch `scoreBiome`: each candidate passes the wiki thresholds, then an original score picks the winner. Share test: arid shrubland at most 20 %, temperate forest at least 15 %, rainforest at least 6 % of land at default settings | WG-1 | done |
| WG-03 | Movement difficulty (Caravan): biome base (forest, desert, tundra, boreal, shrubland 1; ice 1.5; rainforest 2; swamps and bog 4) plus hills +0/+0.5/+1.5/+3; winter up to +2 between -17.1 and -22.5 C; roads x0.5 | `MovementDifficulty` is an invented formula in the wizard (`ebiten/wizard/screen_landing.go:200-225`); caravans ignore it: `caravan.BaseTilesPerDay` is a flat 3 tiles a day (`domain/world/caravan/caravan.go:17`, `SpeedFor` `:36-38`, `Advance` `:53`) | Tile card shows made-up numbers; travel time ignores terrain | Domain `planet.MoveDifficulty(tile, day)` with wiki values (winter curve linear, original). Used by the tile panel, route cost and caravan speed (MAP-09) | WG-1 | done |
| WG-04 | Forage per biome (Biomes): forest 100 %, swamps 75 %, shrubland 50 %, desert 25 %, extreme desert, ice sheet and sea ice 0 %, boreal 75 %, bog 50 %, tundra 50 % | Low, Medium or High from berry-bush density (`wizard/screen_landing.go:227` `ForageLevel`) | Not the wiki percentage | `biome.Def.Forage` (percent), shown as a percentage | WG-1 | done |
| WG-05 | Disease frequency per biome (Biomes): mean time between disease, 50/40/35/30/65/90/90/60/45/80/90/90 days | Not per biome: the nature storyteller rolls a disease weighted only by the difficulty's disease frequency (`application/game/incidents.go:149` `natureOdds`; `disease.go:15-20`) | No biome effect, nothing shown | `biome.Def.DiseaseMTB` shown in the tile panel; the disease incident weight uses it. Behaviour change; the golden re-pin is stated in the handoff Wired: `biome.DiseaseOdds(b)` scales the disease percent in `natureOdds` (temperate forest = 100 %), `TestNatureOddsFollowTheBiome`. | WG-1 | done |
| WG-06 | Time zone shown in the tile inspector (World) | Missing | No time zone | `planet.TimeZone(tile) = round(lon / 15)`, shown as "UTC+n" | WG-1 | done |
| WG-07 | Five road types (World generation): same speed, different look; ancient asphalt from old ruins; dirt and stone roads link bases | One road kind: `Planet.Roads [][2]int32` (`planet/planet.go:82`), built in `planet/roads.go` | No classes | `RoadClass` per segment: neolithic bases get a path or dirt road, industrial bases a stone road, a seeded ancient network (extra Kruskal pass over 6-10 random land ruin tiles) gets asphalt road and highway. Drawn by class | WG-1, WG-2 | done |
| WG-08 | Rivers: creek, river, large, huge (World generation) | Three sizes and no huge: `site.RiverSize` (`domain/world/site/site.go:23-30`); sizing from catchment in `endRivers` (`planet/rivers.go:153-200`) | No huge river | `site.HugeRiver` appended (existing values do not move) at flow 30 or more (original); mapgen width table gets a row | WG-1 | done |
| WG-09 | Named world features: biomes, lakes and oceans, mountain ranges (World generation) | None | No names on the globe | `planet.Features()`: flood-fill regions (ocean, sea, lake, mountain range, desert, forest, ice cap, bay), a seeded original name generator, centroid and size. Globe labels and search | WG-1, WG-2 | done |
| WG-10 | Search the world | Missing (UI/UX checklist section 26.2 `Search/find world features`, `[ ]`) | No search | `planet.Find(q)` over features and settlements; a search box on landing and in the world view Landing search done (`worldinfo/search.go`, journey); the in-game world view has the same box. | WG-1, WG-2 | done |
| WG-11 | Random site honours player criteria | `RandomLanding(n)`: unfiltered weighted pick (`planet/query.go:390-401`, weights built in `finish` `:376-390`) | No criteria | `RandomLandingFiltered(Filter{Biomes, TempMin/Max, Hills, River, Coast, Road, Stone, Caves}, roll)` and a filter panel | WG-1, WG-2 | done |
| WG-12 | Landing is not next to a settlement; warning within 4 tiles of a faction base (Faction base) | The adjacency rule exists (`planet/query.go:308` `ValidLanding`); no 4-tile warning | No warning, no nearby list | Warning row plus "Nearby bases" (3 nearest, with faction and hops) from `NearbyBases(i, n)` | WG-1, WG-2 | done |
| WG-13 | Inspector fields (World): biome, terrain, elevation, average temperature, rainfall, growing period, stone, time zone, travel time by season | Most present (`wizard/screen_landing.go:270` `BuildTileInfo`); missing time zone, disease, forage %, travel by season, quadrum temperatures | Rows missing | Rows added plus a 4-quadrum temperature table (`QuadrumTemps`) using the existing calendar names. `BuildTileInfo` moves to the new `worldinfo` leaf | WG-2 | done |
| WG-14 | Hilliness is readable | Only a tint | Hard to read | Relief plus a Hilliness view mode (fifth mode) Legend swatches read `globe.HillLegend/RoadLegend/RiverLegend`. | WG-1, WG-2 | done |
| WG-15 | Planet name | Missing | No name | Seeded original `Planet.Name()` shown in the Create World header and the world view | WG-1, WG-2 | done |
| WG-16 | Temperature units (Options) | Celsius only; NG-1 asked for Fahrenheit; settings are at `SettingsVersion = 6` (`platform/config/settings.go:24`) | No unit choice | Options, Gameplay, "Temperature unit" (`config.Gameplay.TempUnit`, `SettingsVersion` 6 to 7, absent means Celsius), one formatter on Env, UI literals swept Option, formatter and UI sweep done (`ebiten/units`); the domain sentences (`domain/pawn/stat/stat.go:142`, `domain/scenario/catalog.go:193`) are converted at their sinks by `units.Temp.Text` (`hud/chips.go chipsIn`, wizard scenario tips, trait effects, condition picker). `SettingsVersion` is still 6 (`TempUnit` is omitempty, absent means Celsius). | WG-2 | done |
| WG-17 | Seeds stay stable across versions | `Params.Version` (`planet/planet.go:26`) is never set by the wizard (`wizard/newgame_state.go:191`, `wizard/screen_worldgen.go:127`), so every world is v0; `Seed` xors it in (`planet/builder.go:109-112`), so any generator change would move old worlds | Unversioned generator | New worlds set `Version = 1`; generator changes are gated on `Version >= 1`. The v0 golden stays and a v1 golden is added | WG-1 | done |
| WG-18 | Journeys | No journey for globe click, modes, legend or the Factions dialog (`ebiten/journey_*_test.go` has none for the globe) | No coverage | `journey_globe_test.go` | WG-2 | done |
| WG-n/a-1 | Pollution (Biotech) | n/a | n/a (DLC) | None | n/a | n/a (DLC) |
| WG-n/a-2 | Landmass and mountain sliders (Odyssey) | n/a | n/a (DLC) | None. Our slider stops stay at 5/5/3 (the wiki publishes 3, ours is a superset) | n/a | n/a (DLC) |
| WG-n/a-3 | Landmarks and tile mutators (Odyssey; not on the wiki) | n/a | n/a (DLC) | None | n/a | n/a (DLC) |

## Counts

| | Count |
|---|---|
| Rows WG-01 to WG-18 | 18 |
| n/a (DLC) rows | 3 |
| Missing | 0 |
| Partial | 0 |
| Done | 18 |

## How the globe is drawn

Target look (plan section 1.2). The style stays original: broad muted colour regions, light from the upper left, no specular, photo textures or sprites for terrain. Everything is CPU geometry with vertex colours, so `TestGlobeCapture` shows it and no shader fallback is needed.

- **Relief geometry.** Every hill and mountain stands up off the sphere. Each deduplicated corner gets the mean `Elev` of the 3 tiles that share it (ocean counts as 0, so coasts meet at sea level; shared corners have identical positions, so `TestGlobeHasNoHoles` holds by construction). A tile centre peaks at land elevation plus a hill lift (small hills +300 m, large +900 m, mountainous +1800 m) with plus or minus 25 % seeded jitter. Radius is `1 + 0.022 * max(0, h) / 6000` (original exaggeration), about 1.03 at most; water stays at r = 1. Positions are computed once per planet.
- **Faceted lighting.** Each fan triangle has its own vertices and face normal (18 vertices per hexagon, 15 per pentagon). Each frame the sun direction is rotated once into planet space; per face `k = 0.38 + 0.62 * max(0, n.sun)`, with the atmosphere rim kept. Sun-facing slopes are bright and ridges cast soft shade, so ranges read at every zoom. The corner bevel darkening is dropped.
- **Snow caps.** Vertices above `2600 m + 120 m * (tile mean C)` (original) blend toward snow over a 300 m band; ice sheet is always snow.
- **Soft biome borders.** Corner colour is the mean of the 3 tiles' base colours, the centre keeps the tile's own colour; the plus or minus 3 % brightness noise stays. A Grid toggle draws thin tile edges when zoomed in (Dist 2.2 or less).
- **Water.** Ocean depth ramp as today; a lighter shallow band where `CoastDist == 1` and `Elev > -200`; a pale coastline stroke along every land and ocean edge (and lake shores) at zoom 0.5 or more; sea ice gets a pale broken tint.
- **Rivers.** Widths creek 0.6, river 1.1, large 1.7, huge 2.4 (original). Creeks show at zoom 1.0 or more, larger rivers at 0.75 or more. Rivers use the lower of each corner pair so they lie in valleys.
- **Roads by class** (zoom 0.75 or more): path is a thin dotted tan line (zoom 1.0 or more only); dirt road is dashed brown over a dark casing; stone road is solid grey over a casing; ancient asphalt is solid charcoal with a pale dashed centre; ancient highway is the same at 1.6x width.
- **Labels.** Drawn in the existing exempt `drawMarks` pass. Shown only for features facing the camera (`centre . CamDir > 0.55`) inside a zoom band by kind: oceans and continent-scale features at Dist 2.4 or more; ranges, lakes and deserts at 1.6-3.2; settlements at 1.9 or less. One pixel dark shadow; ocean names lighter blue with letter spacing; a screen-rect collision pass caps labels at 24.
- **Selection and highlight** sit on the displaced surface, never inside mountains.
- **View modes.** Biome, temperature, rainfall, elevation and the new hilliness mode all keep relief and lighting; only base colours change. The legend gains hilliness swatches and a road and river key.
- **Palette.** `assets/palettes/world.yaml` gains `snow`, `shallow`, `shore`, `grid`, `road_path`, `road_dirt`, `road_stone`, `road_asphalt`, `road_centre`, `label`, `label_ocean` and `hill_flat/small/large/mountain`, mirrored in `WorldPalette` (`TestGlobePaletteMatchesYAML`). Any colour left unused is removed.
- **Budget.** `globe.View` is at its field ceiling: once-per-planet data moves into one `geom *planetGeom` field. Gate: `BenchmarkGlobeFrame` at most 2 ms at Dist 1.5 with zero steady-state allocations; if it fails, beyond Dist 3.2 facets fall back to the 7-vertex fan (an LOD switch in `Build`).
- Every consumer gets the look: the landing screen, the setup backdrop disc (`wizard/setup_backdrop.go:145`) and the in-game world view.

## Numbers

Source column: wiki (rimworldwiki.com page) or original tuning (the wiki publishes nothing).

| Number | Value | Source |
|---|---|---|
| Rainforest | 15 C or more and 2000 mm or more | wiki: Biomes |
| Temperate forest | -10 C or more and 600 mm or more | wiki: Biomes |
| Desert | 0-600 mm | wiki: Biomes |
| Extreme desert | 4.82 C or more and 340 mm or less | wiki: Biomes |
| Tundra | 0 C or less | wiki: Biomes |
| Ice sheet | -10 C or less | wiki: Biomes |
| Sea ice | -22 C or less | wiki: Biomes |
| Biome scoring inside the thresholds, arid shrubland above 22 C | scoring function | original tuning |
| Share targets (shrubland at most 20 %, temperate forest at least 15 %, rainforest at least 6 %) | test bounds | original tuning |
| Movement base | forest, desert, tundra, boreal, shrubland 1; ice 1.5; rainforest 2; swamps and bog 4 | wiki: Caravan |
| Hills movement | flat +0, small +0.5, large +1.5, mountainous +3 | wiki: Caravan |
| Winter movement | up to +2 between -17.1 and -22.5 C; linear curve | wiki: Caravan (endpoints); original tuning (linear shape) |
| Roads | x0.5 movement difficulty | wiki: Caravan |
| Forage | forest 100; swamps 75; boreal 75; shrubland 50; bog 50; tundra 50; desert 25; extreme desert, ice sheet, sea ice 0 (percent) | wiki: Biomes |
| Disease MTB per biome | 50/40/35/30/65/90/90/60/45/80/90/90 days, in the wiki's biome order | wiki: Biomes |
| Time zone | round(longitude / 15) | wiki: World (rule); formula original |
| Faction base warning distance | 4 tiles | wiki: Faction base |
| Huge river flow threshold | 30 or more | original tuning |
| River draw widths | 0.6 / 1.1 / 1.7 / 2.4 | original tuning |
| Relief exaggeration | 0.022, hill lifts 300/900/1800 m, jitter 25 % | original tuning |
| Snowline | 2600 m + 120 m per C | original tuning |
| Lighting | ambient 0.38, diffuse 0.62, sun (-0.42, 0.42, 0.80) | original tuning |
| Label limits | 24 labels, facing 0.55, zoom bands as above | original tuning |
| Ancient network | 6-10 ruin tiles | original tuning |
| Slider stops | 5 / 5 / 3 | original (wiki publishes 3 stops) |
