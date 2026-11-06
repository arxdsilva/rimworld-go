# TEMP-2: outdoor climate and events (agent W)

Wiki fidelity under ENG-003 (temperature). Gap rows: `.cursor/TEMPERATURE_GAP_ANALYSIS.md` §3.1, §3.7.

## What changed
- Biomes: `biome.c(summer, winter, swing)` reaches the wiki summer max / winter min for every biome. Temperate is 5 C +-24 with a swing of 6 (original). `biome.Fit` keeps planet sites inside their biome. `biome.Warmth` keeps the old means for animal habitat bands (this fixed `TestClimateFit` and `TestStrayJoinsTame`).
- Events (`domain/incident/climate.go`): heat wave +17 C and cold snap -20 C ramped over 12,000 ticks; volcanic winter -7 C ramped over 50,000 ticks. Lengths 1.5-3.5 days and 7.5-40 days; cooldowns 30/30/140 days; one climate event at a time. They are storyteller incidents now; the wizard's Extreme weather option triples their weights instead of gating them.
- Conditions: heat wave needs a day mean of 20 C or more; cold snap a day mean of 0-15 C (our reading of the wiki's unclear wording); volcanic winter from day 60.
- Volcanic winter: sky light, solar power and grow light at 70 %; wildlife capacity halved. Old saves migrate in the v8 block.
- Plants: `ColdThreshold(id)` gives a per-plant -10 to -18 C. Crops die; healroot and wild plants lose their leaves for 24 h and regrow. No-flap tests.
- Views: `WorldView.ClimateEvent/ClimateEventEnds`, separate from `SkyEvent`. Chip text in `hud/chips.go` reads about +17 C / about -20 C.

## Wiki vs original numbers
- **Wiki:** biome extremes, event offsets, ramps, durations, cooldowns, volcanic winter light and wildlife, plant cold range.
- **Original tuning:** daily swing (6 C for temperate), event weights 5/5/1, the cold snap condition reading, how thresholds are picked per plant id.

## Tests
`TestTemperateGenerationUnchanged` re-pinned (climate part only). The landing screen's automatic season now picks Summer for the wiki temperate climate (mid-spring 5 C is under 8 C). Event ramps, exclusion, cooldowns and plant cold death have unit and no-flap tests.

## UI still owed (TEMP-5)
The volcanic winter chip and sky veil; the event card reads the new field only for heat wave and cold snap.

## Open
- Heat wave summer condition and cold snap condition are readings of an unclear wiki line.
- Event weights and MTB are original (the wiki does not confirm them).
