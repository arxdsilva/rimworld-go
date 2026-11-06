# FOOD-3: animals, butchering and hunting at RimWorld fidelity

Wave 1 of the FOOD programme (ENG-003 extended to food). Code: `internal/domain/animal/husbandry.go` (new), `internal/application/game/herd` (new).

## What changed
- **Butchering** (`husbandry.go`): meat 140 and leather 40 per body size (a cow gives 336 / 96), scaled by age, missing parts and x0.66 for damaged or unslaughtered corpses. Butcher efficiency is 75 % plus 2.5 % per Cooking level. The Butchering wiki page is 404, so the formula and these constants are original tuning guided by the Meat page; `HumanMeatYield` 140 and `HumanLeather` 75 likewise.
- **Meat kinds**: meat stacks carry their species (`Stack.Animal`, `ItemView.Meat`); insect meat added.
- **Human butchering**: `ButcherHuman` recipe with moods (wiki: Butcher table, -6 to all, stacking -6 to the butcher). Cold-hearted and bloodthirsty pawns are immune; there is no cannibal trait yet (FOOD-4).
- **Taming** (`herd/tame.go`): food cost, wildness and minimum skill.
- **Products**: milk and shearing per species from the wiki Milk and Wool pages; eggs per species; fertile eggs edible; omnivores eat eggs and milk.
- **Slaughter job**: Handling work, no weapon (`herd/slaughter.go`), replacing the hunt-flag route.
- **Auto-slaughter**: daily at 07:00 (`herd/auto.go`), per-species caps, `WorldView.AutoSlaughter`.
- **Feeding**: handlers feed downed animals (`herd/feed.go`).
- **Hunting**: hunters carry kills to storage.
- **Views**: revenge chance, wildness and minimum skill exposed on the animal views.
- **Tests**: `herd/herd_test.go`, `herd_food3_test.go`, `craft/butcher_food3_test.go`, `animal/husbandry_test.go`.

## UI representation
Read-model fields exist for all of the above. The animals-tab columns (food restriction, auto-slaughter, revenge, wildness), human-butcher bill listing and tooltips are FOOD-5.

## Open
- FOOD-4: cannibal trait, human and insect meat thoughts when eaten, per-animal food restriction.
- Gestation, growth stages and the Butcher table work time still use size-class values (wiki pages thin).
- Stalking while hunting is not in core.
