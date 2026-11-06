# internal/application/game/herd

## Purpose
The colony's animal handling: taming, training, feeding, collecting and slaughter.

## Notes
Animal life (SIM-026, plan 4.1-4.3, `game/animal_life.go`; the rules the game package runs beside this package): growing up, old age, mating,
birth, eggs and hatching. Everything runs inside phaseAnimals, from
fields on animal.State and item.Stack, with no state of its own:

  - Young animals (younger than AdultDays) don't mate, produce, lay or
    hunt, and eat half portions: a meal fills them animal.YoungMeal
    times as far (eat, animal_needs.go). A young one follows its mother
    (the nearest grown female of its species, tameness and herd),
    wandering within animal.FollowNear tiles of her.
  - Old age: from OldAgeFrom percent of its lifespan an animal rolls an
    OldAgeChance at dawn, once a day.
  - Mating: each grown male checks once a day, at its own awake tick
    (animal.MateTick: no state, and never twice a day), for a grown
    female within MateRadius; both fed (MateFood), wandering, not
    leaving, she neither pregnant nor carrying eggs, both wild or both
    tame, and wild pairs only while the map is under its carrying
    capacity. A MateChance roll; on success he walks to her while she
    waits, MateTicks beside her, then she is pregnant (live bearers) or
    carries MatedEggsOf eggs (layers). A mating not done by MateUntil,
    or either partner leaving Wander, is called off.
  - Birth: at the due tick the litter is born on free tiles beside her:
    young, random sex, her herd and her tameness (named when tame).
    A pregnancy dies with its mother.
  - Eggs: a layer lays at its own tick of the day (animal.LayTick) when
    fed (LayFood), its species' wiki interval and clutch apart
    (animal.EggsToday, FOOD-3): fertile eggs while mated, and tame
    chickens, ducks and geese plain eggs besides (every third egg of a
    mating fertile). Wild plain layers lay only their
    fertile eggs, so the wild map is not strewn with eggs. Tame layers
    lay in their pen beside its trough when they are in one.
  - Hatching: every climate interval each fertile egg stack on the
    ground incubates while 0 C <= air <= 50 C and dies (a plain egg)
    outside it (wiki: Eggs). At HatchTicks the whole stack hatches.
