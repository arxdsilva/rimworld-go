# SIM-026 Animal breeding, life stages and products

## What changed
- **Life** (`internal/domain/animal/life.go`, `application/game/animal_life.go`): ageing, juveniles (smaller, half portions, follow the mother), old-age death. Mating check daily (35%), pregnancy and litters, tame young are named.
- **Eggs:** chickens, ducks and geese lay plain eggs; fertile eggs hatch at 0 to 45 C and frozen ones die.
- **Products** (`animal_products.go`): milk, wool and ember oil with a product bar. New Handle work type runs Milk and Shear jobs.
- **Taming** moved from the Hunt work type to Handle; old saves migrate.
- **Items and recipes** (`economy/item`, `economy/recipe`): milk, wool, egg, fertile egg, ember oil (campfire fuel), wool parka, wool carpet.

## Verification
- Tests: `life_test.go`, `animal_life_test.go`, `animal_products_test.go`, `egg_test.go`, `handling_test.go`.
- Gates: `go build ./...`, `go vet ./...`, `go test ./...`.

## Known limitations
- Young give no reduced (50%) meat.
- No pack animals or caravans; no training or obedience.
- Taming has no skill, wildness or food requirement.
- Job line reads "milk do" for ember oil.
- Worn wool parka reuses the leather parka layer, tinted.
