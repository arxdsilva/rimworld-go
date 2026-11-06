# PRD-1: bill settings, multi-ingredient recipes, gathering

Date: 2026-10-29. Programme BED/PRD/DEF, wave B (committed with BED-1 and DEF-1). Backlog `.cursor/PRODUCTION_GAP_ANALYSIS.md`.

## What changed
- **Recipes and bills** (P1, opus; `internal/domain/economy/recipe`):
  - `Def.Inputs` allows several ingredients; existing recipes are unchanged.
  - `Bill` gains Store (best stockpile, drop on floor, a specific stockpile), Zone, Radius, SkillMin/SkillMax, Worker, UnpauseAt, a sorted forbidden-ingredient set, and SearchAt (the retry cooldown, 500 ticks). All gob-additive.
- **Craft** (`gather.go`, `haul.go`, `settings.go`, `view.go`):
  - ingredients are gathered across stacks within the radius by `job.HaulIngredients`;
  - skill range and specific-worker checks, store mode on completion, `ReorderBill`, and the "until you have" resume level.
  - `BillView` (now an alias of `craft.BillView`) carries the settings, the ingredient list as a comparable fixed-size `BillIngredients` (`craft.NewBillIngredients` for tests), the current worker and progress, gathering, and the no-worker reason.
- **Commands** (lead contract): `ConfigureBill`, `SetBillIngredient`, `ReorderBill`. The bill fields are on `command.Command`.
- **UI** (U1, sonnet): a bill settings dialog with store mode, stockpile, radius, skill range, worker, resume-below and ingredient toggles. Bill rows gain reorder and copy/paste, and the bills tab shows worker and progress. The inspector's `Building` type was split into a `Bills` component to stay under 40 methods.
- **Tests:**
  - no-flap `TestNoIngredientsRetriesAfterACooldown`; whole-game `bills_settings_test.go`;
  - journeys `TestJourneyBillsStoreModeAndRadius`, `…IngredientToggle`, `…Reorder`;
  - coverage: craft 98.5 %, recipe 98.9 %.
- Copy/paste is a UI-side clipboard; paste replays AddBill, ConfigureBill and SetBillIngredient. Stock counting still scans every item stack (`stockOf`); an index is a follow-up.
- The ingredient filter is flat per ingredient kind; a category tree comes with PRD-4's stockpile filter tree.
