# STK-ST: stacks of 1000 and the dumping stockpile

User request (2026-11-01): "a stockpile tile must get up to 1000 items of the same type (unless they are for bolders)" and "we must have dumping stockpile zones just like rimworld". Gap analysis: `.agents/gaps/STORAGE_GAP_ANALYSIS.md` (S1-S10 stacks, D1-D10 dump).

## Behaviour
- `item.Kind.StackLimit()` (`domain/economy/item/value.go`) is now either 1000 (`item.StackLimit`) or 1. It is 1 for kinds with per-unit identity (weapons, contract weapons, apparel, belts, corpses, implants, organs) and for stone chunks. Everything else is 1000, bones included. The STO-2/PRD-4 per-kind wiki table is gone.
- A hopper is not a stockpile tile. It keeps one vanilla raw-food stack: `zone.HopperStack` = 75 (wiki: Hopper) via `zone.Kind.StackCap`. In the game this is `cellCap(g, k, x, y)` (haul_route.go). It caps merging (`mergeOrSetFrom`), spilling (`spillTile`), delivery top-ups (`spareTakes`) and the room a load plans for (`haulRoom`). The refill-below-half rule reads the hopper cap.
- Haulers top up any room. `haul.Toppable` is `Qty < StackLimit`, and `planGather` always sizes the load as min(carry 100, cell room). The old "plain kinds need a full load of room" branch is dead and removed, so a tile no longer stalls at 901-999.
- `Ground.Room` takes the cap (`lim`) from the caller.
- `dropGoods` (trade) splits by the kind's limit, so bought weapons and apparel land one per stack.
- Dump: the defaults already matched vanilla (corpses and chunks, rotten allowed, Normal priority). Fixed: the Architect tooltip said "animal corpses" only (exact edit in architect/catalog.go), and the tool now uses its own `zone_dump` icon (art by STK-AR). Specs and checklists updated.

## Tests
- item TestStackLimit (every kind is 1 or 1000), TestFoodCatalogue, the paste meal at 1000.
- zone TestNewStorageDefaults (the dump is Normal, custom, one stack per cell) and TestStackCap.
- haul TestToppable and the ground Room cap; store TestSearch (a full hopper at 75; the shelf test at 1000).
- game stk_stack_test.go: TestStockpileTileTakes1000 (10+ hauls, never past 1000), TestChunkTileHoldsOneChunk (merge spills; a dump takes one per tile), TestHopperKeepsOneVanillaStack. Updated to the new rule: TestHaulerGathersNearbyPiles ([100 44]), TestTwoHaulersSplitTheCluster, TestGatherCapsByStackLimitAndRoom (960 + 40 = 1000), TestWaveEStackLimit (995 + 8 merges to 1000 + 3).
- journey_stk_test.go: TestJourneyDumpTakesCorpseAndChunk (dump painted from the Architect, the pane shows Normal and a corpse/chunk filter, a deer carcass and a chunk are hauled in) and TestJourneyStockpileTileStacksPastOldLimit (240 wood on one tile).
- On an isolated HEAD worktree with only this change: game/..., scenario, economy and building all pass with -race, and the journeys pass. The only exception is the golden.

## Golden
- scenario TestGrownGolden: 0x51487b30909d4220 becomes **0x7b2b894da0f0fc09** (this change alone). The lead re-pins the combined value.

## Notes
- Game package line delta: 0 (tick.go -6, haul_route.go +6, trade.go 0). No new Game methods or fields.
- HUD: the stack quantities already show in the item views and the stored readouts; no new UI was needed beyond the dump tooltip and icon.
- Failures seen in the shared tree that are not from this change: bench footprint (BN): craft TestBillStoreModes and TestIngredientSearchEdges, production, action_view, power_gen wind; save v10 (BN).
