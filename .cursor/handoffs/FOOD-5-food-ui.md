# FOOD-5: food UI

Wave 2 of the FOOD programme. Read-only views and typed commands only; no domain access from the UI. Code: `internal/adapters/inbound/ebiten` (windows, inspector, hud, architect), `internal/platform/input/defs.go`.

## What changed
- **Assign tab**: a Food column (policy cycle per colonist) and a Food policies editor window (`windows/foodpolicy.go`: list, Copy, Delete, rename, allowed-foods tree, the Foods branch of the storage filter tree).
- **Animals tab**: a Diet column (dropped in narrow windows) and an Auto-slaughter window (`windows/autoslaughter.go`: steppers per species for males, females and young).
- **Work tab**: a Plant cutting column (column 13 "Cut", skill Plants) over `command.WorkPlantCut`.
- **HUD**: a "Days of food" row atop the resource list (hidden at 0); the low-food alert counts days (FOOD-4).
- **Overlay**: Shift+F tints ground by fertility and marks growing zones (input action `overlay.fertility`); no HUD button.
- **Plant pane**: fertility, light, lifespan and blight lines, and "Cut all blight" (a `DesignateCut` per blighted plant). Hydroponics basin and sun lamp panes (`grow_lines.go`).
- **Crop picker**: hover cards with days on soil, gravel and rich soil, yield, Plants skill, hydroponics, frost and shelf life (wiki numbers from FOOD-2).
- **Inspectors**: a meal "Made with" line, a butchered-from line, pawn food policy and last meal, animal food policy, revenge chance, wildness and handling level. Rot uses the existing "Fresh for N days" lines.

## Tests
`food_ui_test.go`, `journey_food1_test.go`, `journey_food2_test.go`, `hud/food_days_test.go`.

## Open
- A rot progress bar (rot shows as "Fresh for N days").
- No search or food-category explanation in the policy editor; no per-zone yield estimate; no numbers on the zone pane's crop list; no overlay HUD button.
- Real-game proof: FOOD-6 added `scenario/year_test.go` (60 days, not a full year); see `FOOD-6-close-out.md`.
