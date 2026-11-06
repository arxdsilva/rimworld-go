# lang package

## Package Purpose
The client's translation layer (OPT-001, `.cursor/OPTIONS_GAP_ANALYSIS.md` "Translation layer"). Keys are the English source strings, so English needs no catalog and draws its source byte for byte.

## Owned Concepts
- `Tr` (`T(src)`), `English`, `Tf(tr, src, args...)`, `Live` (the shell's current catalog behind `ports.Texts`).
- `Catalog` (a `Tr`): `Load(dir, code)`, `Codes(dir) []Choice{Code, Name}`. Files are `assets/lang/<code>.json`: a flat object source→translation plus `"_meta": {"name": "<the language's own name>"}`.
- `exempt.txt`: text the guard lets through (`path:literal # reason`).

## Allowed Dependencies
- Standard library only (tests read Go source with `go/parser` and the Noto fonts' cmap by hand).

## Forbidden Dependencies
- Ebitengine, the ebiten root, feature packages, domain and application.

## Public Contracts
- A missing key, a blank translation, or one whose fmt verbs differ from its source shows the source. English never applies its file.
- `Codes` lists English first (even without a file), then the readable catalogs by code.
- Codes are `^[a-z]{2,3}(-[A-Za-z0-9]{2,8})*$`, so a code never names a path outside the folder.

## Invariants
- No package state: the catalog is a value the shell holds in a `Live`.
- `assets/lang/en.json` is generated: `go test ./internal/adapters/inbound/ebiten/lang -run TestCatalogIsCurrent -update` after adding `T`/`Tf` calls. It fails while stale.
- `TestUITextIsTranslatable` counts literals drawn without `T`/`Tf` (sinks seeded from `gfx`/`uikit`, helpers that forward a string parameter found automatically, `Label`/`Tip`/… fields, locals, constants, drawn returns). It reports only until OPT-3 flips `guardFails`. `LANG_GUARD_LIST=<pkg|all>` lists the sites.
- `TestCatalogGlyphs`: every rune in `en.json` must be in both Noto Sans faces; other catalogs' missing runes are reported (CJK needs a font of its own).

## Hot Paths
- `Catalog.T` is one map lookup; `Live.T` adds a nil check.

## Determinism Rules
- Presentation only; never reaches the simulation.

## Save/Mod Impact
- The chosen code is a setting (W1-OPTS); catalogs ship in `assets/lang/`.
