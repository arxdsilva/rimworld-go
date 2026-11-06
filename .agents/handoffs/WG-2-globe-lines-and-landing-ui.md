# WG-2: roads, rivers and labels on the globe; landing UI; units; journeys

Second half of the globe look and the UI of `.agents/gaps/WORLDGEN_GAP_ANALYSIS.md`.

## What changed
- **Globe lines and names** (`globe/labels.go`): rivers by size (creek 0.6 to huge 2.4, tapered, ending at the shore), roads by class (`road_*`), shore stroke, feature labels by zoom band and facing with a collision pass (at most 24). `worldinfo.DrawLegend` reads `globe.HillLegend/RoadLegend/RiverLegend`. Grid toggle (`land.grid`, `world.grid`).
- **`ebiten/worldinfo/` leaf:** `BuildTileInfo` (moved from the wizard) with time zone, disease, forage %, nearby bases and a quadrum temperature table; `search.go`, `filter.go`, `legend.go`.
- **Landing and Create World:** Mode cycle now includes Hilliness, Legend, Grid, Search; "Select random site" has a filter panel (`RandomLandingFiltered`); the planet name heads Create World once a world exists (before Generate there is no name to show). The landing title is centred in the space left of the top buttons, which used to cover it (`TestLandingTitleClearsTheButtons`).
- **Temperature unit (WG-16).** Options, Gameplay: `config.Gameplay.TempUnit` (`TempUnit` is `omitempty`, so absent means Celsius; `SettingsVersion` is still 6, the plan's bump to 7 was not made); `ebiten/units` formatter; the UI literals were swept (wizard, inspector, overlay, hud, architect).
- **Options step rows** write their value beside their own + button in either column; Max colonies' number used to land in the first column (`TestStepValueFollowsItsRow`).
- **Journeys** (`journey_globe_test.go`): click a tile, the view buttons, search, filtered random site, Factions row, world sliders, options unit and colonies. Also `screen_landing_wg_test.go`.

## Tests
`go test -race` on `globe`, `worldinfo`, `wizard`, the ebiten root journeys; `BenchmarkGlobeFrame`.

## Gaps
- None. The world view (F8) has the same search, legend and grid (MAP-3). Domain C sentences are converted at their sinks by `units.Temp.Text` (`hud/chips.go chipsIn`, wizard scenario tips, trait effects, condition picker), so nothing reads Celsius under Fahrenheit.
