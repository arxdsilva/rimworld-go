# Drying rack: dried food that never rots

## Mechanics
- Build: the drying rack (`construction.DryingRack`):
  - 2x1, rotatable, 6 wood, worker spot on the south side.
  - It's a bench and starts with a "Dry food" bill set to forever.
- Recipe: `recipe.DryFood`:
  - Input `AnyDryable` takes up to `RackCapacity` (25) items from one stack. That means any perishable food not yet dried or rotten, including meals. Corpses are butchered instead, and wheat and beans are already dry.
  - A cook spends 90 ticks laying the batch out. The batch then sits on the rack: `game.racks[bid]` holds a `dryBatch`.
  - A loaded rack takes no new bill until its batch drops.
- Drying (`dryRacks`, runs with `phaseClimate` every 250 ticks):
  - Per-kind time at full speed:

    | Food | Drying time |
    |---|---|
    | Lettuce | 0.75 days |
    | Meat, meals | 1 day |
    | Radishes, corn | 1.5 days |
    | Berries, potatoes | 2 days |
    | Pumpkins | 2.5 days |

  - `item.DryRate(temp)`: full speed from 20C, slower in cool air, and a 25% freeze-dry below 0C.
  - Food doesn't rot while it's on the rack.
  - The finished batch drops on the work spot as a dried stack, with the message "25 dried meat ready". Haulers stockpile it.
- `Stack.Dried`:
  - `Shelf()` is 0 for a dried stack, so it never rots.
  - Dried and fresh stacks never merge. Splits keep the flag.
  - Nutrition is unchanged, since drying only removes water.
- Tearing down a loaded rack drops the batch back undried, keeping the rot it had.

## Read model and UI
- `BuildingView` gains DryKind, DryQty and DryPct. `ItemView` gains Dried.
- Bill status shows "drying a batch".
- The rack draws its batch on the tray, browning as it dries (`drawRackLoad`).
- Dried stacks draw browned (`driedCS`). Rotten corpses draw sickly green (`rottenCS`).
- Hover shows "Dried meat x 25, keeps forever".
- The building inspector shows "drying 25 meat, 40%".
- Architect: Production > Drying rack.
- Art: `furniture/drying_rack_<facing>` (trestles, slatted tray with straw weave, hanging strips), plus `ui_icon/drying_rack`.

## Dev
- `-scenario drying -capture-ui drying`: two racks with meat and berries near the colony.

## Fix
- `centerOn` now marks the camera as placed. Before, the first explicit jump (an alert click or a capture) was undone by the first-frame map centring.
