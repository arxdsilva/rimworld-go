# MED-11: food poisoning, disease on caravans

Part of the RimWorld-style medical follow-ups (original tuning).

## Food poisoning
- `item.Stack.Poison` (‰, zero-safe, saved) is set on cooked meals by `doBill`:
  - base chance 3 + 47·(20−Cooking)²/400 ‰ (50 at level 0, about 15 at 10, 3 at 20);
  - × the kitchen's cleanliness (clean ×0.5, slightly dirty ×1, dirty ×1.8, filthy ×3, outdoors ×1.5);
  - campfire meals at least 15 ‰; clamped to 1–1000.
- Merged and split stacks keep a quantity-weighted chance (`MergePoison`; `splitLoad` copies it).
- Eating rolls it on `streamMedical`, and only when `Poison > 0`.
- A hit adds `CondFoodPoisoning`, a 24 h time course:
  - initial for 4 h, then major until 16 h, then recovering until 24 h;
  - at major: consciousness −200, moving and manipulation −100, pain +100;
  - vomiting, the "sick" thought;
  - bed rest from major until recovering;
  - never lethal.
- UI: "Poison chance N%" on meals; the Health-tab condition row. Its tooltip no longer claims immunity builds for race-free conditions (lead fix).

## Disease on caravans
- Outbreaks can pick caravan members; the letter names the caravan.
- Members keep stepping through the patients index.
- Each hour, the member with the best Medicine skill (at least 1) tends what is due: no bed, no medicine, self-tends at ×0.7, care levels respected.
- Every medical death goes through `g.dieMedical`, which uses `dieAway` for pawns on a caravan. Call sites: disease, bleed, infection, organs, aging, drugs.
- Caravan rows list sick or hurt members, with each member's health in the tooltip.

## Verify
- New tests:
  - `TestPoisonChanceFallsWithSkill`
  - `TestFilthyKitchenPoisonsMore`
  - `TestFoodPoisoningRunsItsCourse`
  - `TestPoisonSurvivesStackMerge`
  - `TestLegacyStacksNotPoisoned`
  - `TestFoodPoisoningBedRestNoFlap`
  - `TestCookedMealCarriesPoison`
  - `TestCaravanMemberCatchesFlu`
  - `TestCaravanMemberDiesAwayCleanly`
  - `TestCaravanMedicTends`
  - UI tests
- Goldens are unchanged; same-seed runs are identical.

## Left open
Meals put through a drying rack lose their poison chance.
