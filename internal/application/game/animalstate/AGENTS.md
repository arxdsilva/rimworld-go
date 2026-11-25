# internal/application/game/animalstate

## Purpose
One owner of nine grouped live animal components and the grouped map/off-map save record boundary.

## Owns
- Registry owns the nine groups Identity, Vitals, Intent, Motion, Threat, Feeding, Handling, Reproduction and Medical. Live references remain stable across insertion and deletion of other entities.
- Record is one (entity ID, grouped animal snapshot) entry. Export follows store order. Import reconstructs all nine groups. Check, CheckSave and CheckGlobal validate identities across maps and off-map collections.
- CloneTravel, ExportWorld and ImportWorld cover world stashes, pod holds, caravans, flights and ropes without modifying live world state.

## Depends on
Domain animal snapshots and component/entity primitives; the existing game/travel save DTO only within off-map serialization.

## Must not depend on
game.Game, simulation phases, screen/UI adapters, or rendering.

## Contracts
- Ref(id) returns directly mutable authoritative *animal.Live. Hot tick/movement paths retain the stable reference and call AssignState after detached legacy algorithms; Get(id) returns a detached animal.State whose changes require Set(id,state).
- Delete clears all nine component owners. AnyLive reads the authoritative component groups without copying them; pure predicates must not retain or modify the live pointer. AnyTame filters the live Tame field before reusing a scratch State; its callback must not retain or mutate that scratch.
- New native v21 saves use AnimalFormat=1 and grouped records. Existing old-native v21 and earlier saves have format 0 and legacy flat Animals, which are imported into the same live registry. A grouped map carries no flat state.
- Save export clones world stash pointers and travel collections before replacing legacy payloads with grouped records. CheckGlobal rejects duplicate animal IDs anywhere.
- Logical StateHash projects the grouped form back to the former flat animal representation so save wire changes do not change world hash meaning.

## Tests
records_test.go, registry_test.go, domain/animal/snapshot_test.go, game/animal_save_v19_test.go, animal_save_world_test.go and animal_grouped_replay_test.go. The thousand-animal mid-save replay, duplicate ID validation and actual historical saved games are required regressions.
