# MAP-3: founding, abandoning, caravan speed and encounters

## What changed
- `SettleHere`/`SettleReason`: refused at the cap or next to a settlement; converts a temporary map or opens a colony (`maps_found_test.go`). The cap is `command.SetColonyCap` (Count), sent by the App at start and load when the setting is above 1 and on every change, clamped to `maps.ColonyCapLimit` 5, never saved (exception noted in `ebiten/AGENTS.md`). Options, Gameplay: "Max colonies".
- `AbandonMap`: carried items are lost; abandoning map 0 promotes the next colony (`TestAbandonFirstColony`).
- Caravan speed (MAP-09): `caravan.Advance` over `MoveDifficulty`, 06:00-22:00, mass x2 to x0.5, mounts 1.3-2.0, `TravelEstimate` (`TestCaravanEtaMatchesTravel`). An empty caravan makes about 22 tiles a day; a caravan with nobody walking moves at full speed; hours use the colony clock.
- Encounters: hourly roll on a named stream, visibility 1 pawn 20 %, 2 pawns 36 %, 2-day cooldown (`TestEncounterCooldownNoFlap`); ambush, manhunter ambush and demand; demand letters carry Give and Refuse (`hud/letters.go letterFooter` through `LetterView.Demand`, command `EncounterAnswer`); encounter map 100x100 then `CaravanReform` to `ResumeDest`.
- World view (`ebiten/worldview`, F8, full screen; `windows/world*.go` deleted): picking (cycle, double-click enters), markers (`colony_home`, `site_camp`, `encounter`, `site_ruined`, `map_threat`), inspector with a disabled reason on every gizmo, route preview and ETA, pod range ring and landing menu, search, legend, grid. Escape order: targeting, pod menu or spot, selection, close. 14 journeys in `journey_world_test.go`, including `TestJourneyWorldDemandLetterAnswers` (stages the road roll through `game.DevDemand`).
- Fixes: Enter is disabled unless the caravan is Arrived; Abandon's Cancel no longer clicks through to the globe (the wizard's globe too); the tile card skips razed bases (`worldinfo.Opts.Live`).
- Fahrenheit: `units.Temp.Text` converts domain sentences at their sinks, so WG-16 is done; `SettingsVersion` stays 6.
- Split stays disabled ("Not yet").

## Open decision for the user
- Abandoning the LAST colony wipes colonists, prisoners and items, keeps the empty map and posts a letter. The repo has no game-over flow (the plan wrongly assumed one). Decide what the game should do then.

## Gaps
- Open for the user: last-colony abandon (no game-over flow), inert settlement turrets, held-open compound doors.
- The meeting encounter is deferred (no caravan trading).
- Max colonies: Options exposes 1-5, default 1.
