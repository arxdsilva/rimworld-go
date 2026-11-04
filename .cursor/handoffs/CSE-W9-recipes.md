# CSE-W9: CSE-046 recipe ingredient kinds named

Scope: `gui_extended_recipe` (CSE-046). The recipe summary names the item kinds each ingredient takes.

## Changed
- `internal/domain/economy/recipe/recipe.go`: `Input.Kinds()`, the kinds `Accepts` takes, in kind order.
- `internal/domain/economy/recipe/input_kinds_test.go`: `TestInputKindsMatchPlanner` (stateless inputs: `Kinds` equals the planner's `AcceptsStack` for every kind).
- `internal/application/game/craft/view.go`: `Recipes()` fills `RecipeView.InputKinds` from `Def.InputKinds()` (`Kind.String()`, kind order).
- `internal/adapters/inbound/ebiten/inspector/bill_config.go`: `Bills.pickTip(r)` returns "Takes: wood, steel, ..." when `FullRecipeList` is on, "" otherwise.
- `internal/adapters/inbound/ebiten/inspector/bill_config_test.go`: `TestRecipeRowNamesKinds` (default on, read model names kinds, toggle off gives no tip).

## Open
- The picker row is not wired yet. `openBillPicker` in `inspector/building_inspector.go` (outside this task's file ownership) needs `Tip: func(i int) string { return a.Bills().pickTip(recipes[i]) }` added to its `uikit.ListSpec`.

## Notes
- The planner's per-stack state (dryable, smelt yield, wreck) is not part of `Kinds`; the test skips those inputs.
- `Def.InputKinds` is unchanged.
