# SIM-017 Corpses, graves, funeral pyres and mourning

## What changed
- **Corpses:**
  - When a colonist or raider dies (`die()`), they drop their weapon and leave an `item.CorpseHuman`.
  - The corpse records who it was (`Stack.Of`) and when they died (`Stack.DiedAt`). `CanMerge` requires the same `Of`, so two bodies never merge.
  - The pawn stays in `Snapshot()` as dead and `Embodied`; the renderer draws the item instead.
  - Raiders no longer vanish after `corpseLinger`.
  - Human corpses rot into `item.Bones` (never spoiling) instead of being deleted.
  - Human corpses are not butcherable and are never stockpiled.
- **Grave** (`construction.Grave`): a 1×2, rotatable, dug construction (new `Def.Dug`: work 400, no materials).
  - An empty grave is walkable.
  - A filled grave blocks movement and can't be marked for deconstruction (a one-line guard in `placement.go`).
  - `graveRec{Occupant, Name, Cause, DiedDay, BuriedDay, Headstone}`. The headstone is a hash of the occupant ID, mod 4.
- **Burial job** (`job.Bury`, haul work, ahead of ordinary hauling):
  - Picks the nearest free body (corpse or bones; ties go to the lower ID), then the nearest reachable empty grave and a tile beside it.
  - Reserves both, carries the body, then fills the grave (200 work).
  - Done: the stack is removed, the record is written, the message "<Name> was laid to rest" is posted, and every living colonist remembers `LaidToRest` (+3).
  - If the target vanishes, the job fails safely and the body is put down.
- **Funeral pyre** (`construction.Pyre`, 6 wood, 2×1 bench): its default bill `recipe.Cremate` burns one body plus 2 wood.
  - The wood is taken from the nearest free stacks at completion; the bill waits until 2 are free.
  - `BuildingView.Burning` is set while it cremates. Done: "<Name> was cremated" and `LaidToRest`.
- **Thoughts:**
  - `ColonistDied`: −6 for 3 days, colonists only.
  - `UnburiedCorpse`: a state thought, −2 while a human corpse is within 8 tiles in the same room or both are outdoors. It scans the item array, not the map.
- **Views and actions:**
  - `ItemView.Of`/`OfName`; `BuildingView.Occupant`, `Epitaph`, `Headstone`.
  - Action `dig` while building a grave, `bury` while filling one, `cremate` at the pyre, `haul` while carrying a body.
- **Saves:** grave records are saved additively. On load, a dead pawn with no body and no record gets a corpse; loading twice changes nothing.
- **Dev:** `-scenario burial`, and the late game's graveyard (`burialGround`).

## Tests
`burial_test.go` covers:
- death leaves a corpse (colonist and raider)
- rot to bones
- grave build and bury, end to end
- nearest body and grave, and no grave means no burial
- the pyre bill
- all three thoughts
- filled graves blocking and not being deconstructable
- the dig/bury/cremate actions
- save round trip and old-save migration

Plus `TestHumanCorpseAndBonesNeverStockpiled`. Updated tests:
- `TestDownedRaiderBleedsOutAndDropsWeapon`: the corpse now stays.
- `TestRaidersAttackAndArmedColonistFightsBack`: its loop ends when no raider is standing.

## Benchmarks and determinism
- Within 5% (see SIM-016).
- Deterministic: ties broken by ID, no RNG or wall clock.
- Seeds diverge, because raider bodies now persist.

## UI
- Architect (Furniture): Grave, Funeral pyre.
- Grave inspector: "Here lies <name>" and the epitaph, or "Empty grave".
- Presentation is in ART-009.

## Known limitations
- No grave assignment or reservation by the player, and no sarcophagi or funerals.
- Pyre wood isn't carried to the pyre; it's drawn from stock when the cremation finishes.
