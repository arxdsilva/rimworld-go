# UI-014 Biome picker, Wildlife tab, animal inspector

## What changed
- **New game wizard** (`screen_newgame.go`, `biome_ui.go`): seed field (typed digits, Dice for a random seed) and a biome picker. Each card shows the icon, temperatures, growing days, ground, plants and top wildlife. Generate is now enabled. The top bar shows the biome.
- **Animals tab** (`animals_ui.go`): Colony/Wild toggle. The Wild list shows species, count, diet, predator flag, tame %, meat yield, with Hunt all, Tame one and Cancel. A Handle work column was added to the Work tab.
- **Animal inspector** (`animal_inspector.go`): state, food and rest bars, sex, stage, age, pregnancy %, product bar, diet, tame chance.
- **Trough panel** (`trough_ui.go`): stock and feed filter.
- **Alerts** (`alerts.go`): animals hungry, wild animals eating crops, timbergnaws eating trees (N felled), pen full.
- **Presentation:** juvenile scaling, rest pose with snore Zs, grass tint by biome. Read models only (`Snapshot()`); all orders are typed commands.

## Verification
- Tests: `biome_ui_test.go`, `animal_ui_test.go`, `app_test.go`.
- Gates: `go build ./...`, `go vet ./...`, `go test ./...`.

## Known limitations
- No world-map tile picker; the biome is picked directly, so spec sections 6 and 7 are partial.
- Wild list has no sort/filter, hunt-risk estimate or body size.
- Colony table lacks health, master, area and training columns.
- No pen capacity readout beyond the pen-full alert.
