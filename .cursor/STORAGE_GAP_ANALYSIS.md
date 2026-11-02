# Storage gap analysis: stacks and the dumping stockpile (STK-ST)

Scope: how many items share a tile, and the dumping stockpile zone,
against vanilla RimWorld (core, 1.4/1.5). ENG-003 applies: the wiki's
published numbers may be used; names and prose stay original. Sources
(read 2026-11-01): rimworldwiki.com *Stockpile zone*, *Dumping stockpile
zone*, *Zone/Area*, *Hopper*. The wiki's *Stack* page was unavailable
(HTTP 404); per-item limits come from each item's page (STO-2/PRD-4 history).

Columns: vanilla behaviour · our evidence (file:line after this change) ·
gap · done.

## Stacks

| # | Vanilla | Evidence | Gap | Done |
|---|---|---|---|---|
| S1 | Each item has its own stack limit (most raw resources 75, silver 500, meals 10, medicine 25...). | `internal/domain/economy/item/value.go:77` `stackLimit` | **User decision (2026-11-01)**: every stackable kind holds up to `item.StackLimit` = 1000 per tile. It replaces the STO-2/PRD-4 per-kind wiki limits on purpose. | yes |
| S2 | Weapons, apparel, corpses and body parts never stack; each has its own quality, hit points or body. | `value.go:77` (1 for `IsWeapon`, `IsContractWeapon`, `IsApparel`, `IsBelt`, `IsCorpse`, `IsImplant`, `IsOrgan`) | none | yes |
| S3 | Stone chunks: one per tile. | `value.go:77` (`IsChunk` gives 1) | none | yes |
| S4 | (no vanilla item) Bones: our item for a rotted body. | `value.go:77` | Bones were 1 (a corpse stand-in) with no per-unit state, so they move to 1000. | yes |
| S5 | A shelf cell holds several stacks (wiki: Shelf). | `internal/domain/building/zone/map.go:41` `ShelfStacks` = 3, each stack up to its limit | none | yes |
| S6 | A hopper holds one stack of raw food (wiki: Hopper; the raw-food stack is 75). A hopper is not a stockpile tile. | `zone/map.go:63` `HopperStack` = 75 (wiki), `zone/map.go:67` `Kind.StackCap`; `internal/application/game/haul_route.go:440` `cellCap`; `game/store/search.go:105` refill below half (original) | Without this cap, raw food would have filled a hopper to 1000. | yes |
| S7 | Haulers top up a partial stack of the same kind. | `game/haul/load.go:8` `Toppable` (any room), `game/tick.go:876` `planGather` (load = min(carry, room)), `game/tick.go:912` `haulRoom` | Plain kinds used to top up only when a full 100-item load fit, so a tile stalled at 901-999. Any room now takes a load sized to it. The dead `lim < item.StackLimit` branches are gone. | yes |
| S8 | A stack dropped on a full tile spills to a neighbour. | `game/tick.go:1640` `mergeOrSetFrom`, `game/tick.go:1698` `spillTile`, `haul_route.go:425` `spareTakes`, all capped by `cellCap` | none | yes |
| S9 | Traders' goods arrive split into legal stacks. | `game/trade.go:370` `dropGoods` splits by `k.StackLimit()` | Bought weapons and apparel used to land as one stack of n. | yes |
| S10 | A pawn carries by mass, about one stack. | `item/item.go:581` `CarryLimit` = 100 | Accepted difference (brief: carry limits unchanged). | n/a |

## Dumping stockpile

| # | Vanilla | Evidence | Gap | Done |
|---|---|---|---|---|
| D1 | Painted from the Architect's Zone menu, next to the stockpile. | `internal/adapters/inbound/ebiten/architect/catalog.go:107` (`dump`, `DesignateStockpile` variant 1); `game/zones.go:90` | none | yes |
| D2 | Default filter: corpses and chunks, nothing else (wiki: Dumping stockpile zone; Zone/Area: "debris and corpses"). | `zone/defaults.go:19` `NewDump` (human and animal corpses, bones, chunks) | none (bones are our corpse remains) | yes |
| D3 | Rotten/fresh and clean/tainted special filters exist; the wiki gives no default. | `NewDump` leaves `Special` zero (rotten allowed); `zone/special.go` | Original tuning: rotten is allowed, because corpses rot and the dump is where they go. A normal stockpile refuses rotten (`NewStockpile`, `zone/defaults.go:10`). | yes |
| D4 | Default priority: higher than Low (the wiki suggests lowering it to Low). The game uses Normal. | `zone/map.go:90` (zero value = Normal); `zone/sto2_test.go` asserts it | none | yes |
| D5 | Works exactly like a stockpile. Haulers fill it while it allows the item and has room, and they carry out things it refuses to a stockpile that takes them. | `game/haul_priority.go:83` `staysStored` (a refused stack is loose again); `zone/filter.go:306` (same universe as a stockpile); no other `KindDump` branch in the game | none | yes |
| D6 | Named "Dumping stockpile N". | `architect/zone_select.go:192`, `inspector/bill_config.go:128` | N is the zone id, not a per-kind counter. Accepted. | yes |
| D7 | Architect tooltip: a zone for dumping. | `catalog.go:107` Desc | It said "animal corpses" while the default takes human corpses too. Now: corpses and chunks, rotting ones too. | yes |
| D8 | Stockpile and dumping zones take colours from the storage palette, one per zone. | `render/overlay/zones_draw.go:233` `ZoneColour(z.ID)` for both | none | yes |
| D9 | The dump tool has its own icon. | `catalog.go:107` `Icon: "zone_dump"` (art by STK-AR, `overlay.py` badge family) | It used the stockpile icon. | yes (AR art) |
| D10 | One stack per tile, the item's own limit. | S1-S3: a dump tile holds one corpse or one chunk; anything else it is set to take stacks to 1000. | none | yes |

## Tests

- `item/stack_limit_test.go` TestStackLimit: every kind is 1 (per-unit identity or chunk) or 1000.
- `zone/sto2_test.go` TestNewStorageDefaults (dump at Normal, custom, one stack per cell) and TestStackCap.
- `game/stk_stack_test.go`: one tile reaches 1000 wood over 10+ hauls and never goes past; chunks stay one per tile, merged or hauled into a dump; a hopper keeps 75.
- `adapters/inbound/ebiten/journey_stk_test.go`: a dump painted from the Architect (pane: Normal, a corpse/chunk filter) gets a carcass and a chunk hauled in; a one-tile stockpile stacks 240 wood.
