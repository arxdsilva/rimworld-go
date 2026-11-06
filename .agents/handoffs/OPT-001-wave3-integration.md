# OPT-001 Wave 3: every UI string goes through the catalog (lead integration)

Commits: inspector (W3-INSP-A 0d532ff61 9661ed518 1c1b7ae0e 8df18ed86; W3-INSP-B f7d477985 4039edf79), the other packages (see each `OPT-001-*` handoff), the callers (W3-WIRE a261ab0f7), the lead's root labels and exemptions (61368f15b), and the guard switched on (9c26fcd52).

- `TestUITextIsTranslatable` now fails on any literal the client draws without `T`/`Tf` (`guardFails = true`). An id, a unit glyph, a product name or developer text goes in `lang/exempt.txt` as `path:literal # reason`. Rules marked "Wave 4" are simulation names that Wave 4 removes.
- `assets/lang/en.json` holds 2,335 keys. The extractor now also collects `T(k)` where `k` is a string constant of the caller's package (`bgPausedLabel`, `commitTag`, `EndingTag`). A key held in a variable, or in another package's constant, never reaches the template.
- Inspector: every pane's text uses `a.shell`/`c.shell`. The English wrappers (`WrapLogRows`, `RotBar`, `PlantTitle`, `PenTabBody`, `StatusTag`, `CasketCommand`, `ventCommands`, `BuildingTitle`, `AreaRefLabel`) stay only for their tests. The scroll hint "v" is exempt as a glyph. The unused `noDataYet` constant was deleted.
- Gates on 9c26fcd52: build, gofmt, vet, arch and art-check pass. `go test -race ./...` passes, and the slowest binaries are ebiten at 177 s and game at 141 s.

Left English (no `T` yet; the guard can't see them because they are built far from where they are drawn):
- Inspector: `TankLines`, the `PrisonerModes` labels and tips, `PlantCategory`, "Made with: ", the need labels in `drawNeedsTab`.
- HUD and dialogs: `alertExplanations`, caravan reasons, trade steps, `TooltipLines`, `CropLines`, `StockLine`, `PowerLine`, the zone pane's `Lines`, `powerLegendTxt`.
- Dossier: Childhood/Adulthood, "frozen, %.1f days fresh".
- Options: the row value captions.
- Caches built once in English: the wizard's `ScenarioLines` on each `ScenEntry`, and the research text. A language switch shows them in English until they are rebuilt.
- Text drafted twice, which could drift: `WorkHelp`, and the `DisabledNote`/`DiseaseFreq` literals in `options_compat.go` and `wizard/screen_difficulty.go`.

Player-facing: no new screen. English output is byte for byte unchanged, so no capture changed.
