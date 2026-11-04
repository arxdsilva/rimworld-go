# CSE-W1: Options › Habits tab (CSE-049, CSE-023)

## Done
- `options/habits_tab.go`: the tab. 27 toggle checks in four groups (Free time, Food and drugs, Tidying, Hauling and crafting), each with a one-line tooltip; three limit steppers (0..`MaxLimit`, TidyTend 0 reads "Unlimited"); a "Back to defaults" button; a scroll area (`uikit.ScrollArea` + `Form`). Values come from `Habits.Rules()`; clicks send `SetGameplayToggle` / `SetGameplayLimit` through `Host.Send` (applied at once; the colony is paused behind Options).
- Outside a colony the tab is listed but shows one sentence saying why it is off and declares no widgets.
- `options/deps.go`: consumer-owned role `Habits{Rules() gameplay.Settings}`, added to `Shell`. Root side: `optionsHabits` in `options_compat.go` (reads `renderer.game.World().Gameplay`).
- `screen_options.go`: tab label, tab list (habits before controls, in and out of a colony), `optTabW` 150 -> 112 so 7 tabs fit, draw path as for Story.
- `story_tab.go` `UI()` dispatches to the habits tab (outside the owned list; two-line change).
- Tests: `options/habits_tab_test.go` (`TestHabitsRowsCoverEveryToggle`, tab list, unlimited text, defaults, clamp); `journey_habits_test.go` (`TestJourneyHabitsToggle`, `TestJourneyHabitsLimit`, `TestJourneyHabitsBackToDefaults`). `screen_test.go` tab-count assertion now 7 with habits after story. `SetGameplayToggle` / `SetGameplayLimit` removed from `journey_mods_owed_test.go`.

## Notes
- The journeys assert after `update`, not `tick`: the game is paused behind Options, so `tick(1)` runs 0 ticks. The command applies at once.
- `options/AGENTS.md` should list `domain/gameplay` under allowed dependencies (not mine to edit).
- Checklist rows CSE-049 / CSE-023 left to the lead.
- Seen in others' files: `TestEveryInputActionHasAJourney` fails on `journeyKindTODO` entries MarkUnload, SetBenchClean, SetName "covered now or never emitted" (not caused by this change).
