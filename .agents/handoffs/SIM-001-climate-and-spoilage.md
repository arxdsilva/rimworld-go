# Climate, seasons, spoilage and frost

## Temperature (`domain/world/temperature`)
- Temperatures are integer tenths of a degree C.
- `Outdoor(t)` is the daily mean plus a day/night swing. The swing is ±6C: coldest at 05:00, warmest at 15:00.
- `DailyMean(t)` is a yearly triangle wave, 9C ± 15C:
  - It peaks on day 15 and bottoms out on day 45.
  - The game starts in mild spring (about 9C).
  - Winter nights reach about -12C.
- `Indoor(t)` is the daily mean + 3C. Indoors means any enclosed room (`roomAt >= 0`).
- `Season(t)` is centred on the curve:
  - Spring is days 53-7.
  - Summer is days 8-22.
  - Fall is days 23-37.
  - Winter is days 38-52.

## Spoilage (`item`)
- `Stack.Rot` accumulates rot ticks. `Kind.ShelfTicks()` is the shelf life at 10C or warmer:

  | Item | Shelf life |
  |---|---|
  | Meat | 2 days |
  | Corpses | 2.5 days |
  | Meal | 4 days |
  | Lettuce | 6 days |
  | Berries | 10 days |
  | Radishes | 14 days |
  | Potatoes | 30 days |
  | Corn | 40 days |
  | Pumpkins | 50 days |
  | Wheat, beans, rations | never |

- `RotRate(temp)` is in permille:
  - 0 at 0C or below.
  - Rises linearly up to 10C.
  - Full speed (1000) above 10C.
- Merged stacks take the quantity-weighted average of their rot. Splits copy it.
- Rotten and fresh corpses never merge.

## Game (`application/game/climate.go`)
- `phaseClimate` runs every 250 ticks.
- `rotItems`:
  - Spoiled food is deleted, with the message "N meat rotted away".
  - A rotten corpse stays but can't be butchered (`ingredientFor` skips it). After twice its shelf life it decays away.
  - Carried or reserved stacks keep rotting but are only removed once free.
- `frostCrops` kills crops below their `FrostKill`:
  - Corn, beans and pumpkin die below -1C.
  - Potato dies below -2C.
  - Lettuce dies below -3C.
  - Radish dies below -5C.
  - Wheat dies below -8C.
  - Wild plants never die of frost.
- Growers don't sow when the day's mean temperature at a cell is 3C or lower (`growingSeason`). Without this, crops sown into winter froze straight away.
- Growth is multiplied by `plant.TempFactor`:
  - 0 at 0C or below.
  - Ramps up to full speed at 10C and stays full to 35C.
  - Fades to 0 by 45C.
- Read model:
  - `WorldView` gains OutdoorTemp, IndoorTemp, Season and Messages (the last 12).
  - `ItemView` gains FreshDays, Rotten and RotRate.
  - `CropView` gains FrostKill and ShelfDays.

## UI
- The top bar shows season and outdoor temperature.
- The hover panel shows tile temperature (marked indoors when in a room).
- Item hover shows "spoils in N days", "(chilled)", "frozen" or "rotten".
- Messages stack top-left for about 10s.
- New alert "Food spoiling soon (N)" for food with less than a day left.
- The crop picker shows frost-kill temperature and shelf life.

## Probe
- Seed 7 survived a full year with 3 colonists alive and about 900 food left in the next spring. Frost hits around day 38, and stored food stays frozen through winter. Thawed radishes and lettuce rot in early spring.
- `RIMWORLD_PROBE=1 go test ./internal/application/game -run YearProbe -v`
- Takes about 15 min.
- Runs a full year with stockpile, harvest, hunt and a field of every crop.

## Next
- A cooler/freezer room (needs power or a passive equivalent).
- Heaters and campfires.
- Pawn cold/heat comfort.
- Weather.
