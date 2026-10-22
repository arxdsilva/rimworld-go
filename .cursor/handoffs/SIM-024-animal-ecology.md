# SIM-024 Animal ecology: needs, states, grazing, troughs

## What changed
- **Ecology rows** (`internal/domain/animal/ecology.go`): per species diet, core and marginal biomes, egg kind, spawn weights (our formula). Domestic species are never spawned wild; sex and age are rolled at spawn.
- **Needs and states** (`application/game/animal_needs.go`): every animal has Food and Rest. States: Wander, Feed, Rest, Hunt (shown as Attack), Flee, Leave, with hysteresis so they do not flap.
- **Feeding:**
  - Grazing on grass, browsing, crop raiding (raises an alert), eating carcasses (consumed, shared between animals).
  - Tree-eaters (timbergnaw) fell trees.
  - Starvation kills.
- **Rest:** herds bed down together; night and day sleepers.
- **Predators:** hunger-driven, give up chases; badly hurt fighters break off.
- **Troughs** (`troughs.go`, `trough_feed.go`): feeding troughs in pens with a plants/meat/any filter. Haulers fill them; pen animals eat from them.
- **Performance** (`animal_index.go`): creature bucket index, and animals make full decisions every 60 ticks (paced), so no per-tick global scans.

## Verification
- Tests: `ecology_test.go`, `animal_needs_test.go`, `handling_test.go`, `wildlife_test.go`, `bench_wildlife_test.go`.
- Animals-only benchmark 0.205 s (was 1.46 s before the index).
- ColonyHour/ColonyRaid about 30/32.8 ms against 22.3/24.9 before. The cause is the start population, now sized to carrying capacity (about 15 animals instead of 8 on the 48×40 bench map). Measured 18.7/24.6/29.5 ms for 0/8/15 animals.

## Known limitations
- Trough feed does not rot and is lost when the trough is deconstructed.
- Badly hurt predators can starve.
- Wild animals eat loose food from the start kit.
- No taming skill, wildness or food requirement; no training or obedience.
