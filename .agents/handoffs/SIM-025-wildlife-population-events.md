# SIM-025 Wildlife population and nature events

## What changed
- **Carrying capacity** (`application/game/population.go`): per-biome capacity from wildlife density. Arrivals enter at map edges when below 70%, herds leave above 130%, and predators have a cap. The start population is sized to capacity.
- **Incident framework** (`internal/domain/incident/incident.go`, `application/game/incidents.go`): definitions with weights, cooldowns and biome limits. A nature event is considered every 2 to 5 days.
- **Events:** herd crossing, stray joins, farm animals wander in, taken a liking (self-tame), timbergnaw gnawing.
- **Threat pool:** a mad animal (single manhunter) joins the storyteller threat pool in `raids.go`.
- **Messages** carry jump links (Focus) to the event location.

## Verification
- Tests: `incident_test.go`, `population_test.go`, `incidents_test.go`, `hive_test.go`, `race_on_test.go`/`race_off_test.go`.
- A 10-day stability test keeps populations inside the band.
- Gates: `go test ./...`, `go test -race` on the game package.

## Known limitations
- Raids and threats still use the older storyteller schedule; the incident framework covers nature events only. No incident queue, point budget or category.
- No player population policy beyond hunting and slaughter.
- Incidents are Go data, not data-pack files.
