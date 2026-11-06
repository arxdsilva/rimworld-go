# FOOD-1: nutrition and hunger at RimWorld fidelity

Wave 1 of the FOOD programme (ENG-003 extended to food: wiki numbers, original names and prose).

## What changed
- **Scale** (`economy/item/item.go`): a full bar is 1.0 = 10000. Raw food, milk, pemmican, kibble and hay are 0.05 a unit. Simple, fine, survival and paste meals are 0.9, lavish 1.0, and the legacy ration is a survival meal. `Kind.MaxEatAtOnce` lets a pawn eat one meal or up to 20 raw units in a sitting (original tuning).
- **Per-stack nutrition** (`animal/food_nutrition.go`, new): eggs by species (0.25 to 0.6, `Stack.Animal` is now set on plain eggs and kept when a fertile egg freezes) and carcasses by their meat. `EdibleCorpse` excludes mechanoid wrecks, rotten carcasses and humans. `AgeHunger` sets babies at ×0.4 and juveniles at ×0.75.
- **Hunger** (`pawn/needs`):
  - the Fed, Hungry and Ravenous falls are 4, 2 and 1 per 15 ticks (1.6, 0.8 and 0.4 a day);
  - `Pair.Band` gives the thought band with a 1 % hysteresis, and `Pair.Seeking` is the eat latch (on at 30 %, off at 80 %);
  - `DecayRated` replaces the carry loop in `skills.go`, `Portions` rounds to the nearest fill (the excess is wasted), and `FightBand` moved out of `social_fight.go`.
- **Malnutrition** (`body/cond_malnutrition.go`, using free functions because `Body` is at its method budget):
  - it starts at 0 % food with a per-pawn rate rolled on the medical stream (1.51 to 2.27 %/h), recedes at the same rate once the pawn is fed, and kills at 100 % (`stepMalnutrition`, hourly in `phaseMedical`, cause "no food");
  - its five stages carry consciousness and hunger offsets (`body.HungerMul` feeds `diseaseNeedRates`);
  - `HealHour` heals nothing while a pawn is malnourished;
  - the old down-at-0 rule and die-after-2500-ticks rule are gone for colonists, prisoners and caravans;
  - malnourished pawns also eat animal carcasses (`bestFood` and `hasEdible`).
- **Thoughts** (`rooms.go`): Hungry is −6, Ravenous −12, and Malnourished −20 to −44 by stage.
- **Sleep**: sleepers wake to eat only when ravenous (`tick.go` busy interrupt, `rest/sleep.go`); the 30 % line waits for morning.
- **Recipe counts**: simple meal 10 raw; fine 5+5, lavish 10+10, survival 6+6, pemmican 5+5 (×16), kibble 20+20. The paste dispenser takes 0.3 (3000).
- **Animals** (`ecology.go`):
  - `sizeMeal` is 500/250/136/70/34, a daily need of 0.12/0.24/0.44 (alpaca)/0.86 (cow)/1.76;
  - hay 0.05, grass 0.15;
  - the drain is scaled by age;
  - `ProductPerDay` fills at 100/50/25/0 % by saturation.
- **Save v7**: the bar scale is unchanged, so stored hunger and animal food carry over as they are. A pawn an older save has down from hunger gets up (`migrateStarvation`).
- **UI**: `PawnView.Malnutrition` (colonists and prisoners), an inspector status line "Malnutrition N%", and the Health tab's condition row.
- **Game package**: net 0 lines; no methods or files added.

## Open
- The bands' immunity effects, smokeleaf and hypothermia hunger factors, and cannibal corpse eating (FOOD-4).
- Patients and prisoners still feed at 25 % (`care.FeedHungry`).
