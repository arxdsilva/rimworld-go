# Task Handoff

Task ID: PLAY-006
Status: complete (phase 2; cooking and hunting land in later phases)

## Summary

Food kinds with per-kind nutrition and eating preference (meal > ration >
amberroot = berries > meat), berry bushes yield berries and regrow,
growing zones sown with amberroot (Grow work: harvest > sow > cut), zone
crops harvested automatically and resown. Read model gains GrowZones,
FoodByKind and PawnView.LastMeal. Art: berries basket, ration pack, raw
meat, simple meal, growing-zone overlay, zone icon. Renderer: grow-zone
overlay, per-kind item sprites, seamless opaque tiles at fractional zoom.

## Food production design (the four routes)

1. Forage - wild berry bushes, 5 berries per harvest, regrow to 35%.
2. Farm - growing zones; amberroot 8 per harvest, faster on rich soil.
3. Hunt - phase 5: animals drop meat.
4. Cook - phase 3: stove turns raw food into meals (more nutrition per raw unit).

## Verification

go test -race ./... green; new tests: zone sow/grow/harvest, berries,
meal preference + nutrition, cancel zone, sow order, grow-zone tool.
In-game: assets/generated/ingame_food.png.
