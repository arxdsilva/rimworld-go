# SIM-016 Weather

## What changed
- **Model:** `internal/domain/world/weather`. It is pure and deterministic.
  - **Kinds:** clear, cloudy, rain, storm, snow, blizzard.
  - **Blocks:** weather changes in 6-hour blocks. The next kind comes from a seasonal Markov table (`Row()`), hashed from (world seed, block) with a splitmix hash. It never uses the global RNG.
  - **Seasons:** Spring and Fall weight rain and storm ×1.3, Summer ×0.7.
  - **Cold:** a day mean ≤ 0 °C turns rain into snow and storm into blizzard. Snow never falls above +2 °C.
  - **Intensity:** eases over an hour towards a jittered target, kept as `State.Goal`.
  - **Wind:** direction drifts over days and per block. Speed ranges by kind.
  - **Ground:** `SnowDepth` builds while snowing and melts above 0 °C; rain halves it once per block. `Wetness` rises while raining and dries otherwise (twice as fast in Summer).
- **Game:** `internal/application/game/weather.go`.
  - `phaseWeather` takes a new block at each 6-hour boundary and steps the state every 250 ticks.
  - The weather seed is hashed from the saved terrain, so a loaded game continues the same sky.
- **Gameplay:**
  - **Walking:** outdoor steps cost more: rain +15%, storm +25%, snow depth > 0.3 +20%, blizzard +40% (`weatherSlow` wraps pawn and animal step costs).
  - **Campfires:** an outdoor campfire is doused (no fuel use, no heat, no cooking) in rain or snow at intensity ≥ 0.3, and resumes afterwards. `heatRooms` now goes through `burning()`.
  - **Temperature:** outdoors −2 °C in snow and −5 °C in a blizzard.
  - **Thoughts:** "soaked" (−3, 6 h) out in rain or a storm; "snow day" (+2) on the first light snow outdoors each day.
- **Views:** `WorldView.Weather`, `WeatherIntensity`, `WindX`/`WindY`, `SnowDepth`, `Wetness`; `PawnView.Soaked`; `PlantView.Wilted` (frost-stressed, or not growing right now).
- **Dev:** `SetWeather(kind, intensity)`; `-scenario rain|snow`. These start just past a block boundary so the forced weather holds for about 5 hours.

## Tests
- `weather_test.go` (domain): determinism, Markov rows summing to 1, the cold and season rules, easing, snow, wetness, wind ranges, and a year of variety.
- `weather_test.go` (game): slower outdoor steps (not indoors), a doused campfire burning no fuel, the snow chill, both thoughts, `Wilted`, saves, the `SetWeather` hook, and the view fields.
- Coverage: weather 93.4%, game 86%+.

## Benchmarks
- Hour 16.6 → 17.3 ms (+4%), Raid 19.8 → 20.4 ms (+3%), including SIM-017.

## Determinism and saves
- Deterministic, but seeds diverge from earlier builds (slower walking, doused fires).
- `weather.State` is saved additively. Older saves start clear.

## Known limitations
- No fog kind; storms only darken visually.
- `Wilted` is a frost or no-growth snapshot, with no per-plant stall timer.
- No weather-controller UI. `Set weather` is still unchecked in the UI/UX checklist.
