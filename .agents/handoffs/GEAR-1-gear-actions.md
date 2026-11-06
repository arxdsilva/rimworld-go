# GEAR-1: Drop, Eat and Lock in the simulation

The three Gear tab commands from the GEAR-0 contract now run in `game/sense` (`sense/drop.go`). Rows: GEAR-004, GEAR-005, GEAR-007 (simulation side; the buttons are GEAR-2). A colony that never sends these commands is hash-identical: `cmd/benchmark -hash` matches across all 12 fixtures, compared against a worktree of the previous HEAD.

## Behaviour
- **DropGear** (Mode 0 pack entry ID, 1 the n-th worn garment, 2 weapon, 3 spare; the same index rules as MarkUnload).
  - A pack entry falls whole at the pawn's feet at once (`GearStock.SetDown`, as `Spill` does). The weapon and the spare drop at once too (the host's DropWeapon and DropSpare).
  - A garment comes off through a forced `job.UnloadPack` (Count `modeApparel`) of `dropWearWait` = 50 ticks (ours: the unload's pause plus the garment's), then lies at the pawn's feet (`TakeOff`).
  - A prisoner's garment comes off at once, with no job. A prisoner has no pack and no weapon, so only garments apply.
- **Queue.** `tick.go` sends a queued DropGear to `queueOrder`. `execOrder` runs it with `Gear.Drop` when its turn comes, and `orderLabel` names it "Drop gear". A plain drop happens at once and leaves the queue alone. The forced take-off job keeps later queued orders waiting.
- **ConsumeFromPack.** Food is taken out (one hand load) and eaten in place: `job.Eat` with Count 2 and Carry, forced. `doEat` eats the best portions, and what is left goes back to the pack (`BackToPack`) or down. A drug: one dose through `job.Ingest` (`doseTicks` 120 mirrors `dose.ingestTicks`), forced. If the job cannot start, the stack goes back.
- **SetApparelForced** (Index: the n-th worn garment) sets or clears the outfit's forced bit (`craft.Outfit.SetForced`, the bit `wear(..., forced)` sets). Colonists only.
- **Refusals** (our words, exported `Why*` constants):
  - Drop: not one of ours (guest, visitor, trader, hostile), down or dead, nothing there.
  - Eat: a prisoner, not one of ours, down, nothing there, too full (food at or above 90 %, `tabFullAt`), the drug policy forbids it (`dose.Allowed`), can't be eaten.
  - A diet does not refuse a player's Eat (the map's Eat order does not check it either).
  - A refused command does nothing. The read model carries the reason.
- **Read model.**
  - `PawnView.Gear.NoDrop` = `DropWhy(id, ModeAny, 0)`, one line in `game.go`.
  - `PackItemView.Gear` = `Gear.PackGear`: Grams, Use (eat or drug) and Why (`EatWhy`).
- No threshold drives any of this. Each action is one player command, so there is no flap to guard.

## Files
- `sense/drop.go` (new): `Side`, the `GearOrders` role (Up, Prisoner, Outfit, SetOutfit, MayDose), `DropWhy`, `Drop`, `EatWhy`, `Consume`, `Lock`, `PackGear`, `applyGear`. `sense.go` routes the commands; `gear.go` embeds the role.
- `sense_gear_host.go`: the five roles, `PackItemView.Gear`, and `RemoveQty` tightened (same behaviour).
- Root net: +10 lines (host +4, `order_queue.go` +5, `game.go` +1, `tick.go` 0), as allowed. No Game methods or fields.
- Tests:
  - `sense/drop_test.go`: every mode, refusals, prisoner, food, drug, lock, pack facts. The `errand` fake gains an embedded `orders`.
  - `gear_actions_test.go` (root): in the game, the queue order and its label, a plain drop keeping the queue, a prisoner drop, the read-model reasons, Eat through the real `doEat`, and the lock.

## Gaps
- The UI (GEAR-2) must show `NoDrop` and `Pack[i].Gear.Why` as disabled-button tips. The apparel-policy exemption for forced garments is W1-OUT's.
- A colonist in a mental break is not refused. `applyOne` guards only move, prioritize and draft.
- Vanilla's take-off time varies by garment. Ours is one fixed 50 ticks.
