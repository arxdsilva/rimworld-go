# SIM-019 Wildlife: 61 species

## What changed
- **Roster** (`internal/domain/animal`): 61 species, each with key, archetype, size (XS–XL ± trim), behaviour, habitat, natural weapon, herd size, speed class and flags (dimorphic, explosive, small-prey).
  - **Saves:** enum values are append-only. Deer and TimberWolf took the dustbuck and ridgewolf slots, and the old names still parse.
  - **Originals:** ten RimWorld-invented creatures are original designs: emberbuck, cinder_rat, woolhorn, glimmerstag, timbergnaw, hive_beetle, hive_brute, hive_crawler, ground_sloth, dire_wolf.
- **Numbers are our own formulas** (`buildDef`), from a size table and a behaviour table:

  | Size | HP | Step | Meat | Leather | Damage | Cooldown | Accuracy |
  |---|---|---|---|---|---|---|---|
  | XS | 12 | 16 | 2 | 0 | 2 | 100 | 80 |
  | S | 28 | 14 | 6 | 2 | 4 | 110 | 78 |
  | M | 55 | 16 | 14 | 5 | 7 | 120 | 75 |
  | L | 100 | 18 | 30 | 10 | 11 | 130 | 72 |
  | XL | 170 | 22 | 55 | 16 | 16 | 140 | 70 |

  - Fast species −4 step, slow +6. Flee step = step × 0.6.
  - Tame / revenge by behaviour: domestic 75/5, pet 70/3, grazer 40/10, skittish 30/0, docile 50/5, territorial 15/50, predators 15/45, hive 0, legendary 5/100.
  - HP multipliers: legendary ×2.5, tortoise ×1.5.
- **Behaviour:**
  - Predators hunt prey no bigger than themselves; small-prey predators hunt XS/S only.
  - Hive insects are hostile from spawn, never tameable and never flee.
  - Explosive animals burst on death: 10–20 damage within 1.5 tiles, a message, and a `WorldView.Bursts` stamp. There's no fire spread, because the `fire` package has no ignite API.
  - The legendary enrages others of its kind when hurt.
- **Spawning** (`wildlife.go`, `g.rng` only):
  - Start herds and daily migration are weighted by season and habitat: cold ×3 in winter and ×0.3 in summer, hot the reverse, domestic and pet ×0.15, no legendary or hive.
  - **Threat roll** (out of 8):

    | Event | Winter | Rest of year |
    |---|---|---|
    | Predator pack | 4 | — |
    | Manhunter pack (from day 8) | — | 2 |
    | Hive swarm (from day 12) | 1 | 1 |
    | Raid (the rest) | 3 | 5 |

    Raids were previously 1/2 in winter and 3/4 otherwise.
  - Hive swarms burst from the rock nearest the colony.
  - The glimmerstag visit: from day 20, 2% a day, at most once a year.
- **Corpses:** one `CorpseAnimal` kind with `Stack.Animal`; no merging across species. One butcher recipe yields by size. Legacy corpses and bills convert on load. A hauled split keeps the corpse's species (`splitLoad`).
- **Views:** `AnimalView.Label`, `Attack`, `Female`, `Size`; `ItemView.Animal`; `WorldView.Bursts`. Attack verbs appear in the logs ("Gored", "Pecked", …).
- **Dev:** `-scenario zoo` (`-width 80 -height 60`) puts all 61 species, tamed and named, in a pen.

## Tests
- `roster_test.go`, `corpse_test.go`, `butcher_test.go`, `wildlife_test.go`, `hive_test.go`, `split_test.go`.
- Updated existing tests are renames only. One exception: the provoked-manhunter test uses an elk, because a deer's revenge is now 0.
- Coverage: animal 100%, game 87.8%.

## Benchmarks and determinism
- ColonyHour +4.5%, Raid +0.5%; animals with no flee radius skip the threat scan.
- Deterministic. Seeds diverge from earlier builds because the starting wildlife changed.

## UI
Renderer and art are covered in ART-011 and ART-012.

## Known limitations
- No milk, wool, eggs, juveniles, nuzzling or training.
- Explosions don't start fires.
- Female sets share the male corpse sprites.
