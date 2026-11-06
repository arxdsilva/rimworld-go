# PRD-0: the craft package and the quality domain

Date: 2026-10-29. Programme BED/PRD/DEF (`.cursor/PRODUCTION_GAP_ANALYSIS.md`). Wave A, committed with BED-0 and DEF-0: the three extractions share `game.go`/`tick.go`/`save.go` wiring.

## What changed
- **`internal/application/game/craft`** (X1, opus) now holds bills (`Bills`), bench work (`Shop`), drying racks (`Drying`) and the tool-cabinet bonus (`Cabinets`). Its five roles (Benches, Stock, Output, Workers, Crafters) are value-only, at most 8 methods each, and adapted in `game/craft_host.go`. `production.go` and `drying.go` are gone.
- **Save:** the `Bills` gob shape is unchanged, and `Racks` keeps its field names.
- **Stays in game:** `BillView`, `RecipeView`, `Recipes()`, `stockOf`, `ingredientFor` (surgery, fuel and care use it) and `dropStackOf`.
- **`internal/domain/economy/quality`** (D1, sonnet): levels Awful…Legendary, `Roll(skill, roll, inspired)` from the wiki's 21-row skill table (inspiration = +2 levels), and quality factors per stat (market value, beauty, deterioration, armor protection, comfort, rest effectiveness and more). All values are wiki-cited. Nothing calls it yet; PRD-2 rolls quality at craft time.
- No behaviour change: the benchmark state hashes are identical on cached fixtures. Coverage: craft 98.9 %, quality 94.3 %.
