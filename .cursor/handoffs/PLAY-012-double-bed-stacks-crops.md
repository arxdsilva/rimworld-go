# Task Handoff

Task ID: PLAY-012
Status: complete

## Summary

**Double bed (2x2).**
- Two colonists sleep side by side under a shared blanket.
- Beds now have two claim slots (`Building.Claimant2`), managed through `SlotOf`, `CanClaim`, `Claim`, `Release` and `Owner`. Each sleeper lies in its own slot's head cell (`construction.SleepCell`).
- Colonists prefer the bed they already hold. Claims are what share beds, so sleep jobs no longer reserve the bed.
- Art: `furniture/bed_double_<facing>` plus a blanket overlay. Architect: Furniture, Double bed (the panel's card rows now grow to fit the category).

**Stacks.**
- `item.StackLimit` is 1000. Colonists carry at most `item.CarryLimit` = 75 per trip, and only what a frame still needs.
- Hauls top up same-kind piles in stockpiles before using empty cells.
- Merging no longer loses the overflow: anything past the limit spills to the nearest free tile, or to one with a same-kind pile that has room.
- Hover label shows pile counts ("Wood x 8").

**Farming.**
- Seven real crops replace the made-up amberroot. The amberroot item is now potatoes.
- Growth follows real days-to-harvest at about 12 real days per game day:

  | Crop | Game days | Yield / plant | Soil sensitivity |
  |---|---|---|---|
  | Radish | 2 | 3 | 40% |
  | Lettuce | 3.75 | 4 | 90% |
  | Beans | 5 | 4 | 60% |
  | Potato | 7.5 | 8 | 30% |
  | Corn | 8 | 7 | 100% |
  | Pumpkin | 8.5 | 12 | 70% |
  | Wheat | 10 | 8 | 80% |

- Plants (wild ones included) grow only in daylight, 06:00 to 20:00.
- Speed on soil: rate = 100 + (fertility - 100) x sensitivity. Potatoes barely slow on gravel; corn crawls there and races on rich soil.
- Nutrition follows real energy density. Grain and dry beans are dense; greens are light. Raw grain, beans and potatoes are low preference, so they are best cooked.
- Colonists eat several portions in one sitting until full (12 at most).
- Balance: about three tiles of potatoes per colonist.
- Each crop has its own sowing work and harvest work.
- UI:
  - The Growing zone tool has a crop picker showing days on plain, rich and gravel soil, yield, and food per item.
  - Hovering a plant shows its growth ("Corn, 62% grown" or "ready to harvest").
- Art:
  - Per-crop young/mature/harvestable sprites with a shared `crop_sown`.
  - Produce items and icons; new palette materials.

## Public Contracts Changed

- construction: `DoubleBed`, `Kind.Sleepers()`, `SleepCell`; `Building.Claimant2` and its claim methods.
- `BuildingView.Claimant2`.
- item:
  - `Amberroot` is renamed `Potatoes` ("potatoes").
  - New `Corn/Wheat/Beans/Lettuce/Radishes/Pumpkins`.
  - `StackLimit` 1000, `CarryLimit`.
  - `Raw()` means any food except rations and meals.
- plant:
  - Species `Potato/Corn/Wheat/Beans/Lettuce/Radish/Pumpkin` (Amberroot removed).
  - `Def.Sens/SowWork/Label`.
  - `Crops()`, `Rate()`, `GrowDays()`, `GrowingHour()`.
- game: `Crops() []CropView`. The growing zone defaults to potato.
- Capture: the layout sows radish/lettuce/corn/wheat; `-capture-ui grow`; `-scenario double`.

## Verification

- `go test -race ./...`, vet and staticcheck are clean.
- New tests:
  - double-bed claim slots and slot cells
  - two colonists sharing one bed
  - big stacks hauled in loads of at most 75 with no loss, topping up
  - merge overflow spills
  - crops mature in real-life order; potato 7.5 and radish 2 days
  - soil sensitivity and daylight hours
  - crop picker sends the chosen crop
  - art coverage for every crop stage, produce and icon
- 12-day probe, one 2x3 field per crop: harvests land on radish d3, lettuce d4, beans d6, potatoes d8, corn and pumpkin d9, wheat d11. Fast crops cycle. All colonists alive.
- Asset validator: 376 sprites, 0 errors.
- In-game captures: `assets/generated/ingame_farm.png`, `ingame_crop_picker.png`, `ingame_double_bed.png`.

## Known Limitations

- No seasons, temperature, irrigation, blight or crop quality yet.
- No food spoilage, so big piles keep forever.
- A zone's crop is chosen when painted (re-paint to change).
- Stockpiles don't filter by food type.
