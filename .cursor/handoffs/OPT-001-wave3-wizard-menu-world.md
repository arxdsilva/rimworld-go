# OPT-001 wave 3: wizard, main menu, world view, world info, globe text through the catalog

## Done
- `wizard`, `mainmenu` and `worldview` Shells embed `ports.Texts` (worldview folds it with `ports.Settings` into a local `presentation` interface to stay at 8 methods). `wizHost` and `menuHost` forward `T`/`Tf` in their root compat files.
- `worldinfo` reads the kit's catalog (`k.Access.Tr`, English when unset); `globe.Mode` gains `Label(tr)`.
- Pure helpers got `...In(tr, ...)` variants; the old signatures stay as English wrappers because tests and root code call them: `ModeCaption`, `PacingTip`, `RelationLabel`, `GoodwillTip`, `SkillTip`, `ScenarioLines`, `Draft.StartReason`, `Draft.Rows`, `WorldPhaseName`, `VersionLine`, `attackConfirmText`, `buildObjects`.
- Constants that tests read (`LandTitle`, `NoHostileWarning`, `DisabledNote`, `DiseaseFreqTip/Note`) stay; the drawing code carries the literal in a `T` call so `en.json` sees the key.

## Known gaps (English still shows)
- Root callers of the English wrappers (`options_compat.go` mode caption and pacing tip, `windows_compat.go` faction standing and tip, `inspector_compat.go` skill tip, `screen_pause.go` version line) should move to the `...In` variants.
- Scenario detail lines are cached per list entry at build time, so they stay English until the entry is rebuilt on a language change.

## Proposed exemptions
See the wave report.
