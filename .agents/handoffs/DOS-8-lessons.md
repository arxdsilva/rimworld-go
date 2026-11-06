# DOS-8: colonist card lessons (RHUD-043 to RHUD-047)

Status: done, by the lead. The agent stopped because the lessons needed a card gate and UI signals outside its files.

## What went in

- `learn/lessons.go`: `Lesson.Needs func(config.Settings) bool`; `Frame` fields `CardRead`, `SetupOpened`, `LookRead`, `LayoutsSeen`, `SetupClosed`; lessons `dossier_hover`, `dossier_gear`, `dossier_look`, `dossier_sections`, `dossier_close`.
- `learn/deps.go`: `Sensed.CardHover`, `SetupOpen`, `SetupLook`, `SetupLayout`. `learn_compat.go` fills them from `dossierHost.pointerOnCard` and `dossier.Setup.Showing`.
- `learn/helper.go`: `applies`, `total`, gated `count` and `left`; `watchCard` asks for 45 frames on the card and 150 on the Look tab before a lesson counts as read; a modal already up when the helper starts is not an opening.

## Tests

`TestCardLessonsNeedTheCard`, `TestCardLessonsFinishOnWhatThePlayerDoes`, `TestWindowAlreadyUpAtTheBaselineIsNotAnOpening` in `learn/lessons_test.go`; `TestJourneyDossierLessons`, `TestJourneyDossierLessonsNotOfferedWithTheCardOff` in the root.
