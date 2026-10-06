# Campfire cooking and eating at tables

## Campfire cooking
- `construction.Campfire` is now also a bench, with its work spot just south of it.
- Recipe `recipe.CampfireMeal`, "Cook simple meal over the fire":
  - 2 raw food become 1 simple meal.
  - 180 ticks of Cook work (the stove takes 120).
- A campfire only takes bills while it has wood (`assignBill`). Otherwise the bill status reads "fire needs wood" (`BillNoFuel`).
- New campfires start with a default "until you have 10 meals" bill. Fixtures placed with BuildNow get no default bills.
- Animation: "work at campfire" stirs, with steam.

## Dining (`application/game/dining.go`)
- At the start of eating (the colonist is at the food), `carryToTable`:
  - Looks for a seat beside a table within 30 tiles of the food. A chair is worth 6 extra tiles; otherwise any free floor tile next to the table.
  - Splits off just the portions the colonist needs (`portionsFor`), carries them there and eats.
- With no table in reach, or when already at one, the colonist eats on the spot (`j.Count` = 2 marks that).
- Leftovers stay where the colonist ate.
- "Ate at a table" now needs a table right next to the colonist, diagonals included. Before, any table somewhere in the room counted.

## Not done
- The fine meal (meat plus vegetables). The bill system takes one ingredient stack per bill, so a two-ingredient recipe needs a multi-ingredient haul first.

## Tests
- `dining_test.go`:
  - A colonist carries 2 meals from the floor to the chair at the table and gets the table thought.
  - A campfire cooks only when fuelled.
