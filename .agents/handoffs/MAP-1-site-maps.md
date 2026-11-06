# MAP-1: site maps, ticking maps, switching

## What changed
- `game/maps`: `Atlas`/`Meta`, `Open`/`Enter`/`Close`/`Flush`, arrival routing in `caravanArrived` (existing map, mineral site map, friendly visit, empty tile waits), `forget(id)` leak guard (`TestSiteMapOpenStepCloseNoLeak`).
- Mineral site map: lump placed, 30-day expiry kept (`travel/scanner.go`), raid at day 10 (`TestMineralSiteRaidDayTen`), 24 h pack-up then the colonists form a caravan and the map closes. Close and job-done need 3 consecutive hourly census results (`TestShouldCloseNoFlap`, `TestJobDoneNoFlap`). The abstract `Mine` is replaced.
- Renderer scenes (`ebiten/scene.go`): per-map caches; camera sidecar per map (`filesave.ViewFile.Maps`, old sidecars still load, extends VIEW-001).
- `mapnav` leaf (`]` and `[`), colonist bar groups per map and per caravan, `Jump(Spot)`, map-tagged alerts and letters. Journeys: `journey_maps_test.go` (bracket keys, frame click, bar jump, shift-click refused across maps, letter and message jump, own camera, single-map bar).
- Art: `colony_home`, `encounter`, `map_threat`, `site_camp`, `site_ruined`, `status_caravan` icons.

## Gaps
- Colonist bar has caravan groups and the status icon; per-pawn status marks beyond that are unchanged.
