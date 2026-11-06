# OPT-1 (lang): the translation catalog and the tools that keep it complete

OPT-001 infrastructure (`OPTIONS_GAP_ANALYSIS.md` "Translation layer"). English output is unchanged: nothing calls the catalog yet, and English never applies its file. No state hash impact (client only).

## What changed
- `ebiten/lang/catalog.go`: `Catalog` (a `Tr`; the zero value is English), `Load(dir, code)`, `Codes(dir) []Choice{Code, Name}`. Files are `assets/lang/<code>.json`, a flat source→translation object with `"_meta": {"name": "…"}`. A missing key, a blank value, or a value whose fmt verbs differ from its key shows the source. Codes are checked against `^[a-z]{2,3}(-…)*$`, so a code can't name a path outside the folder. English is always listed first.
- `assets/lang/en.json`: the generated English template, only `_meta` for now.
- `TestCatalogIsCurrent` (`extract_test.go`) reads every `x.T("…")`, `x.Tf("…", …)` and `lang.Tf(tr, "…", …)` in the client, then rewrites en.json with `-update`. It fails while the file is stale.
- `TestUITextIsTranslatable` (`guard_test.go`, `sinks_test.go`, `shown_test.go`) only reports for now: `guardFails = false`, and OPT-3 flips it.
  - Sinks start from `gfx` and `uikit`. Helpers that pass a string parameter on to a sink are found automatically (84 sinks today).
  - A literal counts when it is a sink argument, or inside a `Label`/`Tip`/`Title`/`Reason`/`Body`/`Text`/`Hint`/`Desc`/`Caption`/`Disabled` field, a local or package constant that gets drawn, or the return value of a function whose result gets drawn.
  - `LANG_GUARD_LIST=<pkg|all>` lists the sites. Rules in `lang/exempt.txt` take the form `path:literal # reason`, and each one needs a reason. Today it covers developer mode, ids and the probe's lines.
- `TestCatalogGlyphs` reads the cmap of both Noto faces by hand, with no new module. A rune missing from en.json fails the test. Runes missing from other catalogs are reported. `TestFontCoverage` pins Latin, Greek and Cyrillic as present and CJK as absent.
- `lang/AGENTS.md` added.

## Guard counts (2026-11-06)
Open per package (call / field / var / return): architect 492, inspector 349, wizard 346, dossier/sheet 157, dossier 137, windows 133, root 100, hud 87, options 84, dialogs 80, worldview 35, learn 25, mainmenu 22, worldinfo 19, orders 10, uikit 9, units 9, render/overlay 6, globe 5. Total open: 2105, plus 45 exempt. The ceiling is every letter-bearing literal in the client: 8717. The guard reads syntax only, so 2105 is a lower bound.

## Gaps
- The root `shellTexts` still holds an empty `Live`. The lead wires `Load` and the setting. W1-OPTS adds the Language row. `scripts/package_release.sh` must ship `assets/lang/`.
- Simulation text (OPT-4) needs its own extractor source. Its keys come from the domain and not from client `T` calls.
- Plurals aren't supported. A CJK catalog needs a font.
- Methods are matched by name and arity, with no type information. A false positive goes in exempt.txt.
- No HUD: this is infrastructure. The player-facing part is the Language row (W1-OPTS).
