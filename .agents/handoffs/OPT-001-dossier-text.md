# OPT-001 dossier text through the catalog (W3-DOSS)

- `dossier.Actions` embeds `ports.Texts`; the root `dossierHost` forwards `T`/`Tf` (`dossier_compat.go`). The card and the setup modal write words with `p.a` / `s.a`.
- `dossier/sheet` text functions take a first `lang.Tr` (lines, tips, cells, catalog, defs, validate). `Lookup` and `NewNode` use English: they read flags and keys only.
- Guard: dossier 137 -> 1, sheet 157 -> 1. Proposed exemptions: `dossier/build.go:" v"` (dropdown arrow glyph), `dossier/sheet/lines_misc.go:" °C"` (unit).
- Tests: only call arguments changed (`en` helper in `sheet/tr_test.go`; `dossier/setup_tabs_test.go` holds the English tab names and reasons and the fake shell's T/Tf).
- Not translated: simulation text (labels, status, job words: Wave 4); error values.
- `assets/lang/en.json` is stale until the lead regenerates it.
