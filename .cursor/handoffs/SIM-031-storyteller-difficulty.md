# SIM-031 Storytellers, difficulty and threat points

Part of NG-1 (`.cursor/NG1_NEW_GAME_PLAN.md` §3.2, §3.3, §4.5, §5.2, §5.3). Commits: `37f2935d` (domain), `f815d545` (in play), `46d1110d` (stats follow difficulty changes).

## What changed

### Storytellers (`internal/domain/storyteller`)
Three original storytellers plus a hidden legacy profile. A `Profile` holds the pacing numbers; `Plan(profile, state, now, rng)` returns the next `Decision` (when, category, points percent, optional second threat). All randomness is the per-package `randx` stream, so plans are deterministic.

| Storyteller | Mode | First threat | Gap | Notes |
|---|---|---|---|---|
| Orla Vane, the Steady Hand (`steady`) | Cycle | day 6 | 3 to 5 days | 8 days of pressure then 5 quiet; points +4 % per completed cycle up to +40 %; a threat that would land in a quiet stretch moves to the next cycle |
| Bram Holloway, the Hearthkeeper (`hearth`) | Calm | day 10 | 9 to 15 days | after a colonist dies the pending threat slips 6 days and shrinks to 70 % (once per pending threat: `OnLoss`, `Eased`) |
| Kestrel Quay, the Gambler (`gambler`) | Dice | day 4 | daily roll at 07:00 | chance starts at 12.5 % per day and gains 2.5 points for every quiet day, so quiet spells end; 5 % of threats come as two; points vary 60 to 150 % |
| Legacy (hidden) | Legacy | day 5 | 5 to 9 days | makes exactly the pre-NG-1 draws; used by colonies without a Setup and by `TestLegacyPlanMatchesConstants` |

`storyteller.Spark(profile, days, seed)` gives the example pacing sparkline on Choose Storyteller. Portraits are `ui_portrait/<key>`.

### Difficulty (`internal/domain/difficulty`)
- **Presets:** Wanderer, Gentle, Settled, Seasoned, Grim, Merciless, plus Custom. Seasoned is the neutral row (every multiplier 100, threat scale 100). Threat scale runs 5, 35, 65, 100, 145, 200; peaceful days 0, 12, 8, 6, 4, 3; adaptation off for Wanderer, then strength 60, 80, 100, 100, 100.
- **Settings** (19 fields, each with limits for the sliders): threat scale, big threats, peaceful days, adaptation and strength, time escalation and rate, ignore wealth, crop/mine/butcher yield, research speed, mood offset, infection, disease frequency, animal revenge, predators hunt colonists, extreme weather, pain tolerance offset. `Match` finds the preset a settings value equals; `Encode`/`Decode` is a compact `key=value,...` codec (also the payload of the `SetDifficulty` command).
- **Threat points** (`ThreatPoints`): 25 per standing colonist plus a wealth curve (wealth/50 up to 20,000, then slower, then slower again past 100,000), times threat scale, times (100 + adaptation)/100. With time escalation on, times (100 + rate x years)/100; with "ignore wealth" the wealth term is replaced by 80 points per year. Floor 35.
- **Adaptation** (`Adapter.Day`): a day counter moves +1 per day without a colonist loss (cap 120) and -25 on a loss day (floor -40); the target value is `days/2 - 10` clamped to -30..+40 and scaled by strength; the live value moves at most 2 per day and only when it is 3 or more away from the target (dead band), so it cannot flap (`TestAdaptationNoFlap`).

### In play (`internal/application/game/story.go`, `weather_ng1.go`, `setup.go`)
- A colony with a Setup uses its storyteller's `Plan` for the next threat; nothing earlier than `max(first threat day, peaceful days)` fires. A colony without a Setup keeps the old scheduler in `raids.go` bit for bit.
- The threat die is filtered: a threat the difficulty or scenario rules out, or a raid with no hostile faction to send it, becomes a wolf pack, manhunters or a hive swarm by the same die. Raid points come from `ThreatPoints` times the plan's percent times the faction's multiplier; raid size is capped at 8 plus map area/40,000, at most 24.
- **Difficulty hooks:** crop, mining and butchering yield and research speed (`diffScale`); mood offset; pain tolerance (colonist stats rebuild when the difficulty changes in play); infection chance of wounds; animal revenge chance; whether predators hunt colonists; extreme weather.
- **Extreme weather:** with the setting on, each in-season day (rolled at midnight) has a 6 % chance of a heat wave (+12 °C in summer) or cold snap (-14 °C in winter) lasting 2 to 4 days; the climate curve shifts, so rooms, crops and weather follow. After one ends no new one starts for 3 days (minimum duration plus cooldown, so no flapping). A letter announces it; a chip shows it.
- **Commands** (`command.SetStoryteller`, `command.SetDifficulty`, both ignored by colonies without a Setup): the storyteller switch takes effect at the next scheduling decision (the pending threat stays); the difficulty command takes a preset key or `custom:` plus the encoded settings, and ignores invalid values. Options › Story drives both.
- **Read model:** `Simulation.Story()` returns storyteller name and title, preset, settings, commitment flag and the adaptation value.

## Verification
- Recorded at commit time; the docs pass re-ran nothing. Domain tests: `TestLegacyPlanMatchesConstants`, `TestPlanIsDeterministic`, `TestCycleRestHasNoBigThreats`, `TestCalmGapsAndLossEase`, `TestDiceProbabilityRisesWhenQuiet`, `TestSpark`, `TestPresetValues`, `TestThreatPointsFormula`, `TestAdaptationNoFlap`, `TestCodecRoundTrip`. Game level (`game/setup_test.go`): `TestStorytellerPacingInGame`, `TestPeacefulDaysRespected`, `TestRaidsAttributedToFaction`, `TestNoHostileFactionConvertsRaid`, `TestThreatPointsUsesDifficulty`, `TestDifficultyHooks`, `TestExtremeWeatherEvents`, `TestSetStorytellerCommand`, `TestLossesEaseAndAdapt`.
- Goldens did not move for this work: a legacy colony never reaches `story.go`.

## Known limitations (not done)
- **Positive-event bias:** each profile carries `PositiveBias` (20, 45, 30 percent) but nothing reads it yet; nature events keep their existing weights, so storytellers differ only in threat pacing, not in how kind the good events are.
- **Disease effects:** the Disease frequency slider has no effect (no disease events exist). Only the infection chance of wounds follows the difficulty. The slider says so in its tooltip.
- **Player faction naming** is not implemented (the colony has no editable faction name).
- Death or downing behaviour as a difficulty option is not modelled.
- Storytellers only schedule raids, packs, manhunters and hives; scheduled nature events keep the existing incident cadence.
