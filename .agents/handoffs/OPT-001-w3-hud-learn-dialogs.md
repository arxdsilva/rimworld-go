# OPT-001 wave 3: hud, learn and dialogs text goes through the catalog

Open guard sites: hud 88, dialogs 80, learn 25, now 0 each. English output is byte for byte the same; no test assertion changed.

## What changed
- `Shell` of each package embeds `ports.Texts`. `hud` groups it with `ports.Sizer` in a new `Screen` interface so `Shell` stays at 8 embedded roles. The root shells already promote `T`/`Tf` from `shell`, so no root file changed.
- `hud`: the pure builders (alerts, `BarStatuses`, `GatheringText`, `SeasonText`, `BiomeTooltip`, `ClockText`, `DateSeasonLine`, `FoodDaysRow`, `BuildResourceRows`, `LetterRowLabel`, `MessageLine`, `PawnLabel`, `BarTip`, `GoneAlert`) gained `...In(tr lang.Tr, ...)` forms that the chrome calls with its shell. The old names stay as English wrappers (`hud/english.go` and beside each builder) for the tests and the root helpers. Group alert counts are now whole formats (`%d colonists need beds`). The resources memo key carries one translated word, so a language change rebuilds the rows.
- `learn`: `Lesson.Body` became a method over `Lesson.Text(tr, key)`; titles and goals are English sources (`en.T`) translated where drawn. `Catalog` and `ByID` are unchanged.
- `dialogs`: labels, titles, tips and format strings use `dl.sh.T`/`Tf`; `pawnLabel` and the comms helpers take a `lang.Tr`.
- New test files `texts_test.go` give each package's fake shell English `T`/`Tf`.

## Left as is
`alertExplanations` (a literal table), the caravan/pods reasons constants, the trade step labels, and sim text (`Reason`, names, phases) are not guard sites; sim text is Wave 4. `PawnLabel` is still called in English by `inspector_compat.go` and `windows_compat.go`.
