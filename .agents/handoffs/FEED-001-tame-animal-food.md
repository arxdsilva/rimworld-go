# FEED-001: hungry tame animals reach the food the colony has

User report (2026-10-30): a tame animal stayed hungry while the stockpiles held food. The request was that animals go to the nearest food they can eat, as in RimWorld.

## Causes found
1. `animal.FeedValue` accepted only straw, meat and a short list of raw plants, so kibble, every meal, survival meals and pemmican fed no animal.
2. `findTameFeed` chose the nearest edible stack with no reachability check, since `fenceReachable` passes everything when a map has no fences. A stack behind a shut door was chosen, the walk failed, and the next decision chose it again, forever.
3. There was no fallback for a starving animal.

## What changed
- **Diet** (`domain/animal/ecology.go`), the wiki diet table:
  - kibble and pemmican for every diet but strict carnivores;
  - meals (simple, fine, lavish, survival, rations) for every diet but strict carnivores and tree eaters;
  - raw vegetables and berries for herbivores, omnivores and tree eaters;
  - straw for plant eaters; meat for meat eaters.
  - Strict carnivore is a new `Diet` value. No roster species is on the wiki's strict list (warg, ghoul, vulture, wolverine), so only the original dire wolf is strict, by our own design.
- **Feed first:** animal feed (kibble, straw, raw food for the diet, plants, grazing, carcasses) wins; meals are eaten only when no feed is reachable. User decision.
- **Reachability** (`game/forage`): a region check on the animal's path layer skips food the animal can't reach. A target the animal failed to reach is remembered for 2,500 ticks (original tuning), saved as `Wildlife.Forage`.
- **Starving fallback:** below 15 % food a tame animal searches its whole allowed area or pen (capped at the map or pen extent) until it eats or passes 25 %. The band is original tuning. User decision.
- **Troughs** accept kibble.
- **UI:**
  - the status line names the food ("eating kibble", "eating a meal (no feed in reach)", "searching far for food");
  - a new pane line, "Also eats kibble, pemmican and meals", per diet.
- Wild animals are unchanged and never take colony stores.

## Tests and numbers
- Domain diet tests: coverage domain/animal 98.4 %, forage 98.6 %.
- **Regression for the report:** a loose tame deer, stockpiled corn and kibble behind a shut door, grass 15 tiles off. It eats the grass within 1,500 ticks. With the door removed it eats from the stockpile. The test fails with the reach filter switched off.
- Feed-first, strict-carnivore, starving-band no-flap and trough tests; journey `TestJourneyTameAnimalEatsStockpiledKibble`.
- The old test asserting tame animals leave meals alone now asserts feed first.
- Package game is 37 lines smaller; budget lowered to 54,433.
- Frame p99 against HEAD: grown/250 1.48 to 1.48 ms, grown/500 1.32 to 1.40, raid/250 1.80 to 1.77, raid/500 1.83 to 2.01. HEAD's own second raid/500 run was 2.33, so that is noise; the animals phase p99 is unchanged at about 0.08 ms. A worst-case starving search costs 0.85 ms on an empty 500 map, and runs only on that animal's 60-tick decision.
- TestGrownGolden is unchanged.

## Gaps
- No per-animal food restriction policy (vanilla's food policies).
- `AnimalDietLine` itself is unchanged; the kibble and meals line is separate.
