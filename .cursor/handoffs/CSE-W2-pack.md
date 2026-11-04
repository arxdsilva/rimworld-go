# CSE-W2: the pack (CSE-036)

Each pawn can carry stacks off the map. Nothing fills a pack yet: CSE-W4, W5 and W7 call this API.

## API

`domain/pawn/inventory/pack.go`, values, deterministic, in the order first added:

| Call | Does |
|---|---|
| `(*Pack).Add(st) uint32` | Joins a `SameLoad` entry (rot, poison and wear averaged by `MergeFood`), else a new entry. Clears X, Y, Carrier, Forbidden, Unclaimed, HaulMarked. Qty ≤ 0 returns 0. |
| `(*Pack).Take(id, n) (Stack, bool)` | Up to n. An emptied entry goes; other entries keep ID and order; an ID is never reused in a pack. |
| `Pack.Grams() int`, `Pack.Find(k) (uint32, bool)` | Total mass; first entry of the kind. |

`game/sense/pack.go`, on `senseOf(g).Gear`:

| Call | Does |
|---|---|
| `Pack(id)`, `Has(id)` | Read. `Pack`'s slice stays Gear's; don't write to it. |
| `Capacity(id) int` | Grams: 40 kg times Manipulation (ours). |
| `Room(id) int` | Grams left; `1<<30` with `RespectCapacity` off. |
| `Stow(id, stack, n) bool` | Map to pack. Refuses, changing nothing: no such stack, fewer than n, in another pawn's hands, a corpse, more than `Room`. **The caller clears `j.Carry`** if it stowed the stack in hand. |
| `TakeOut(id, entry, n) (stack, bool)` | Pack to hands; sets `FromPack[id]`. **The caller sets `j.Carry`.** Fails whole when the pawn stands on no map. |
| `Unstow(id, kind) (stack, bool)` | `TakeOut` of the first entry of the kind, at most `item.CarryLimit`. |
| `Spill(id)` | Removes the pack and sets it down (below). Also clears `FromPack[id]`. |
| `Rot(interval)`, `Wealth()` | Colonists on the current map only. |

W5: `Mark`, `DoUnload`, `DoSpare`, `BackToPack` are still inert in `gear.go`. `FromPack` is set by `TakeOut` and cleared only by `Spill`, so `BackToPack` must clear it on landing. New needs go in a new role embedded in `GearRoles` (`GearPawns`, `GearStock` have 4 each); a composite counts one per embed. `PackRows` in `inspector/gear_rows.go` is plain text for now; the row buttons go there.

W4: `Stow` refuses corpses, so a corpse ingredient stays on `HaulIngredients`.

## Decisions

- **Colony stock:** a pack counts in `Game.Wealth` (the row's `TestPackInWealth`) and nowhere else. Stock totals, stockpile counts, hauling and search don't see it: the stack isn't in `stacks` or `itemIDs`.
- **Wealth is per map**, as the rest of `Game.Wealth` is: a pack on the road isn't counted.
- **Rot** runs on `phaseClimate`'s 250-tick pass, not staggered by id as the row says. A stagger would mean a scan every tick, and `Condition.Rot` isn't staggered either. It walks the map's `pawnIDs`, never the map of packs.
- **Packs off the map don't rot**, as a caravan's goods don't, and a casket is stasis.
- **No threshold behaviour:** losing Manipulation drops nothing, and `Stow` only refuses. No no-flap test is owed.
- An empty pack is deleted from `Packs`, so a colony that emptied its packs saves and hashes as one that never used them.

## Where a pawn leaves play

| Path | Site | Result |
|---|---|---|
| Death on a map (combat, surgery, execution, heat, disease) | `dieBy` | Beside the body |
| Death on the road | `dieAway`, through `dieBy` | The caravan's goods |
| Death in a casket or pod | `dieAway`, through `dieBy` | Where it boarded |
| Downed, then stripped | `careGear.StripGear`; `HasGear` counts a pack | Where it lies |
| Arrested or captured | `makePrisoner` | Where it stands |
| Kidnapped | `raidKidnap.Off` | At the map edge, not taken along |
| Raider or visitor leaves, prisoner released | `removeRaider` | Where it stood |
| Left aboard the ship | `shipHost.Escaped` | Where it boarded |
| Left on a closing map | `closeMap` | Lost with everything on that map |
| The last colony ends | `mapsFound.Wipe` | Deleted with the map's items |
| Caravan, casket, pod | `stashPawn` | **Keeps the pack** (it lives on `worldState`) |

Everywhere else an item stays in play. The three ways it can leave are rot, a closing map or final wipe (above), and a caravan whose last member dies: `cargoTick` forgets the cargo of a lost caravan, the pack's share with it.

`SetDown` finds the pawn on any open map, then its stash record; a closed map falls back to the first map's home. It keeps the whole stack, which `dropStackOf` doesn't (book seed, yield, hatch).

## Files

New: `domain/pawn/inventory/pack_test.go`, `game/sense/pack.go`, `game/sense/pack_test.go`, `game/sense_gear_host.go`, `game/sense_gear_test.go`, `inspector/gear_rows.go`, `inspector/gear_rows_test.go`.

Changed: `inventory/pack.go`, `sense/gear.go`, the role line in `sense_host.go`, and hook lines in `combat.go`, `raids.go`, `prisoners.go`, `raid_host.go`, `social.go`, `world_state.go`, `story.go`, `game.go`.

Outside the brief's list, one line each: `game/climate.go` (the rot call; the row has it but no site was listed) and `inspector/gear_ui.go` (`PackRows` joins the tab's lines; `GearRows` stayed as it is).

Lines added to `game`, without tests: about 113, one file.

## Tests

`-race`, targeted. `go test -run 'Pack|Unstow' ./internal/application/game/ ./internal/application/game/sense/ ./internal/domain/pawn/inventory/` passed three times over.

Row tests: `TestPackSpillsOnDeath`, `TestPackRots`, `TestPackTravelsWithPawn`, `TestPackInWealth`, `TestPackRoundTrip` (save, load, identical, hash equal, equal again after 600 ticks). Balance: `TestPackNeverVanishes` sums the kind over maps, packs and caravans before and after each path. Also `TestPackIsOffTheMap`, `TestPackLostWithClosedMap`, `TestPackGoesWithTheFinalWipe`, `TestCapacityFollowsManipulation`, `TestPackRows`.

`-hash` with no pack in use, grown 120, 20000 ticks: seed 1 `0xdf13d1b2cbfc9ceb`, seed 2 `0xb8025f043b9cfb8d`, both as required.

## Save and visuals

No format change: `sense.Save.Packs` and `FromPack` already existed. The Gear tab gains `Pack  1.5 / 20.0 kg` and one line per entry only when the pack holds something, so nothing shows today. Rows past the tab's bottom are cut, like the other lines there.

`TestSOLID` fails on `options.Shell` (9 methods), which is another agent's file.
