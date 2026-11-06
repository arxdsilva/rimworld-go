# OPT-001 architect and orders text through the catalog

- `architect`: `Tools` and `Categories` are built by `toolDefs(tr)` / `categoryDefs(tr)` (English at init). A `Tool` keeps the translated tables (`tooltext.go`, rebuilt when a probe string's translation changes) and shows labels/descriptions through `localize`.
- Pure helpers (`AreaRefLabel`, `AreaPickOpts`, `ZoneToolHint`, `ZoneDragHint`, `ZoneTipText`, `ZoneKindName`, `ZoneDefaultName`, `ZoneCropName`, `AssignAreaTip`) keep their English signatures and delegate to `tr` variants; callers inside the package pass the Shell.
- `Shell` embeds `words` (Hinter + Texts); `orders.Labels` embeds `ports.Texts`, so interface limits hold.
- Not wrapped (guard does not flag them): `TooltipLines`, `CropLines`, `StockLine`, zone pane `Lines`, `PowerLine`, power legend. Root callers `inspector_compat.go` use the English `AreaRefLabel`/`AssignAreaTip`.
- Proposed exemptions: `architect/area_manager.go:"X"` (delete glyph), `architect/area_menu.go:"  v"` (dropdown arrow glyph).
