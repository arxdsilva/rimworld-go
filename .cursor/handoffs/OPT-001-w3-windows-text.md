# OPT-001 wave 3: the colony windows' text goes through the catalog

`windows` went from 165 open guard sites to 2 (both the developer-only "Debug: finish now"); `units` stays at 9 (unit names and glyphs, proposed exemptions below).

## What changed
- `windows.Shell` embeds the new `Chrome` (`Sizer + Dispatcher + Texts`), which keeps `Shell` at 7 interface members (limit 8). The root `windowsShell` already gets `T`/`Tf` from `shell`, so no root file changed.
- Every label, tip, reason, title, placeholder and empty-state text uses `w.sh.T` or `lang.Tf`. Concatenations became one format string (`"Edit %s"`, `"%s  (some)"`, `"Lowest %s"`).
- Tip constants (`assignCareTip`, `assignDrugTip`, `assignFoodTip`, `assignApparelTip`, `assignClearTip`) and the `historyEmpty`, `recordsEmpty`, `combatEmpty`, `workHint` constants became methods or inline `T` calls, so each is one catalog key.
- Exported pure helpers (`AnimalSexLetter`, `AnimalAgeText`, `AnimalPregCell`, `AnimalDoing`, `FollowCell`, `AnimalAreaCell`, `DrawGoodwillChart`) keep their English signatures for the existing tests and wrap an unexported variant that takes a `lang.Tr`.
- Work: `Work.help` and `Work.colName` translate the column cards and abbreviations; `WorkHelp` and `WorkColumns` stay as the English reference the tests read.
- Storage: `Storage.rowLabel` and `qualityNames` translate the fixed tree headings, range names and quality names; `treeRows` still returns English.
- Research: `researchState.tr` (set by `rs()`) feeds the cached panel and box texts; they are rebuilt on selection or progress change, not on a language switch.
- `windows/texts_test.go` gives `fakeShell` `T`/`Tf`. No assertion changed.

## Proposed exemptions
- `windows/research_panel.go:"Debug: finish now" # developer-mode button`
- `units/units.go:"°F" / "K" / "°C" / "1.8 °F" / "1 K" / "1 °C" # unit glyphs and step labels`
- `units/units.go:"Fahrenheit" / "Kelvin" / "Celsius" # Temp.Name feeds the Options value, which has no Shell here; options wraps it`

## Not done
- Catalog (`en.json`) regeneration is the lead's.
