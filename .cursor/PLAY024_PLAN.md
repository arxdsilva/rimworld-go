# PLAY-024: sentry turret, trade, radio console

Original names and tuning throughout (AGENTS.md: no RimWorld balancing
data; the ENG-003 wiki exception does not cover this feature). Plan of
record; the lead updates it when a contract changes (additive only).

## Phase 0 contract (done, lead)

Every agent builds against these from the first minute; they compile and
do nothing until their owner fills them.

| Area | Contract |
|---|---|
| Construction kinds (append only, saved) | `construction.SentryTurret`, `TradeBeacon`, `RadioConsole` (55 kinds in all) |
| Power | `power.ClassDefense` (appended: shed last); `SpecOf`: turret consumer 120 W firing / 40 W idle, console `bench(150)` (150 / 15 W, working while reserved), beacon not electrical. `game.powerWant` asks `turretFiring(id)` for `ClassDefense` |
| Weapon | `weapon.SentryGun` |
| Research (append only, saved) | `research.Sentries` ("Sentry guns"), `research.Radio` ("Radio relay") |
| Jobs (append only, saved) | `job.Trade` ("trade"), `job.UseComms` ("use radio console"); `phaseJobs` calls `doTrade` / `doUseComms` |
| Orders | `command.OrderTrade`, `OrderUseComms`; `forcedJob` calls `tradeJob` / `commsJob`; `OrdersAt` calls `tradeOrders` ("Trade with <name>") and `commsOrders` ("Use radio console"), refused with `ReasonNotYet` ("not yet") in Phase 0 |
| Commands | `ProposeTrade` (`Name` = `EncodeTradeOffer`, `Enabled` = gift), `CloseTrade`, `CommsCallTrader`, `CommsGift` (`Index` faction id, `Count` silver), `CommsRequestCaravan` (`Index`), `CommsRequestAid` (`Index`), `CloseComms`; routed `applyOne` → `applyHostCommand` → `cmd*` in `trade_session.go` / `comms.go` |
| Incidents (append only, saved) | `incident.TraderCaravan`, `OrbitalTrader` (Nature pool); `incidentFits` → `caravanFits` / `orbitalFits` (false in Phase 0, so goldens hold), `fireIncident` → `caravanArrive` / `orbitalArrive` |
| RNG | streams `streamTrade` ("trade"), `streamDefense` ("defense"); no new draws on `g.rng` |
| Save | `saveState.Trade *tradeSave`, `Turrets *turretSave`, nil when absent (no `SaveVersion` bump); `tradeSave.Version` versions the block |
| Offer codec | `game.EncodeTradeOffer([]trade.Delta) string` / `DecodeTradeOffer`: `"kind:qty"` pairs joined by `,`, kind = `item.Kind` id, qty signed (positive: colony buys), both decimal; `""` = empty offer. Decode refuses malformed text, unknown kinds, zero qty and repeated kinds |
| Read models | `BuildingView.AimDir int8` (0..15) and `ShotAge int` (-1 before the first shot) via `fillTurretView`; `TraceView.FromBuilding`; `BurstView.Turret`; `TradeView` / `TradeRowView`, `CommsView` / `CommsFactionView` / `CommsActionView` / `OrbitalView` (`trade_contract.go`); `Game.TradeView() (TradeView, bool)`, `Game.CommsView() (CommsView, bool)` |
| UI interfaces | ebiten `tradeSource`, `commsSource` (`trade_sources.go`) with `(*game.Game)(nil)` checks |
| Domain stubs | `domain/economy/trade` (`TraderKind`, `Side`, `Def`, `Line`, `Stock`, `Terms`, `Delta`, `Reject`, `Result`, `DefOf`, `Generate`, `Wants`, `Price`, `Settle`, `GiftValue`); `domain/faction/diplomacy.go` (`GiftGoodwill`, `CanRequestAid`, `CanRequestCaravan`, `Cooldowns`) |
| Specs | UI §28 / §29 PLAY-024 contracts; ART §38 sprite names and attachments |

## Tuning (our numbers)

### Sentry turret

| Thing | Value |
|---|---|
| Cost | 90 steel + 3 components, 400 work, steel only |
| Footprint | 1x1, blocks movement, not rotatable |
| Hit points | 140 (base 100 × steel 140 %) |
| Cover | 30 % |
| Power | 120 W while holding a target, 40 W idle; `ClassDefense`, shed last |
| Gun (`weapon.SentryGun`) | range 10, damage 9, cooldown 60 ticks, accuracy 75 %, falloff 3 % per tile past the first (48 % at 10 tiles) |
| Target search | every 20 ticks, staggered by building id, through the creature index (`nearestWhere`) |
| Targets | standing raiders and hostile animals in line of sight; never visitors, traders, prisoners, colonists or tame animals |
| Hysteresis | locks on within 10 tiles; lets go past 12 tiles or after 20 ticks out of sight |
| Aim time | 30 ticks before the first shot at a new target |
| Unpowered | no search, no aim, no shots |
| Destroyed | blast: 12 damage to creatures within 2 tiles, 25 to buildings; may ignite (`streamDefense`); `BurstView.Turret` |
| Raider response | may pick a powered turret over the nearest colonist only when closer by `switchMargin` (no flapping) |
| Research | `Sentries` "Sentry guns", 4500, needs Gunsmithing + Electricity |

### Trade

| Thing | Value |
|---|---|
| Trader kinds | bulk hauler (caravan), quartermaster (caravan), curio merchant (orbital) |
| Prices | hundredths of a silver per unit |
| Buy markup | 45 % − 1.2 points per Social level − 0.06 points per goodwill point, floor 6 % |
| Sell cut | 50 % − 1.2 points per Social level − 0.05 points per goodwill point, floor 10 % |
| Caps | a sell price is never above 90 % of the buy price (no arbitrage) |
| Silver | always at face value (100 hundredths) |
| Specialty | a trader pays 10 % more for its specialty categories |
| Hostile | hostile factions don't trade |
| Trader silver | bulk hauler 900–1500, quartermaster 1200–2000, curio merchant 1600–2600 |
| Tech | neolithic traders carry no rifles or components |
| Caravan incident | `TraderCaravan`: weight 14, cooldown 9 days, from day 4; stays 0.75 day near `home()` |
| Caravan losses | killing or capturing the trader drops its goods forbidden and costs the faction 40 goodwill |
| Orbital incident | `OrbitalTrader`: weight 12, cooldown 7 days, from day 6; needs a powered radio console; stays 1.5 days (letter, and an alert with hours left) |
| Colony goods (caravan) | unforbidden stacks on stockpile cells |
| Colony goods (orbital) | only stacks within 6 tiles of a trade beacon |
| Deliveries | caravan purchases drop beside the negotiator; orbital ones arrive by drop pod near the beacon after 60 ticks (`streamTrade` copy of `podSpot`) |
| Trade job | walk up, wait 60 ticks, open the session; held until `CloseTrade`, the trader leaves or the negotiator goes down |
| Trade beacon | 40 steel, 120 work, 1x1, walkable, unpowered, radius 6 |

### Radio console and diplomacy

| Thing | Value |
|---|---|
| Console | 80 steel + 3 components, 300 work, 2x1, rotatable, interaction spot south of the desk (facing south), blocks movement |
| Console power | 150 W while worked, 15 W idle (`ClassBench`) |
| Research | `Radio` "Radio relay", 3000, needs Electricity; unlocks the console and the beacon |
| Gift | 1 goodwill per 40 silver, at most 15 per gift |
| Military aid | friendly factions only; costs 20 goodwill; 3-day cooldown (Phase 4) |
| Caravan request | needs goodwill ≥ 0; costs 12 goodwill; arrives in 6–12 h; 4-day cooldown |
| Orbital call | needs a trade beacon (disabled with a reason otherwise) |

## Phases and ownership

| Phase | Agent | Model | Owns |
|---|---|---|---|
| 0 | lead (WS-D0) | opus | enums, streams, save fields, views, stubs, dispatch, specs (this file) |
| 1 | D1 | sonnet | `domain/economy/trade`, `domain/faction/diplomacy.go` and their tests |
| 1 | S1 | opus | `game/turret*.go`; exclusive edits to `combat.go`, `raids.go`, `defense.go` |
| 1 | S2 | opus | `game/trade*.go` (not `trade_contract.go`), `comms*.go`, `visitors.go` |
| 1 | ART | opus | `artlib/families/defense.py` (new), `power.py`, icons |
| 2 | R1 | sonnet | `turret_draw.go`, `draw_combat.go` muzzle, building inspector, architect tools, `alerts_trade.go` |
| 2 | U1 | sonnet | `trade_ui.go`, `comms_ui.go` |
| 3 | lead | opus | gates, raid-vs-turrets and trade captures, checklists, handoff `ENG-006-play024-trade-turrets.md` |
| 4 | aid agent | opus | military aid (allies fight raiders); `raids.go` exclusive; ships separately |

Lead-only shared files: `command.go`, `tick.go` (job switch, `applyOne`),
`host_commands.go`, `save.go`, view structs, `trade_contract.go`,
`construction.go`, `power.go`, `weapon.go`, `research.go`, `job.go`,
`orders.go`, `incident.go`, `incidents.go`, `rng_streams.go`, specs.
Contract changes go through the lead.

## Named tests

- Domain: `TestPriceBuyAboveSell`, `TestNoArbitrageAllKinds`, `TestPriceSocialMonotonic`, `TestPriceGoodwillMonotonic`, `TestSilverAtPar`, `TestGenerateStockDeterministic`, `TestGenerateStockPerKind`, `TestSettleRejectsUnaffordable`, `TestSettleRejectsOverStock`, `TestSettleRejectsUnwanted`, `TestSettleGiftNoSilver`, `TestGiftGoodwillCapped`, `TestAidNeedsFriendly`, `TestRequestCooldown`.
- Turret: `TestTurretFiresOnlyWhenPowered`, `TestTurretNeedsLineOfSight`, `TestTurretIgnoresVisitorsPrisoners`, `TestTurretAcquireCadenceStaggered`, `TestTurretTargetNoFlap`, `TestRaiderTurretSwitchNoFlap`, `TestTurretDestroyedExplodes`, `TestRaiderTargetsTurret`, `TestTurretShedLast`, `TestTurretSaveRoundTrip`, `TestTurretSameSeed`, `BenchmarkTurretsRaid`.
- Trade sim: `TestTraderCaravanArrivesStaysLeaves`, `TestTradeJobOpensSession`, `TestSessionClosesWhenTraderLeaves`, `TestProposeTradeMovesGoods`, `TestProposeTradeRejectsStaleCounts`, `TestOrbitalNeedsPoweredConsole`, `TestOrbitalGoodsOnlyNearBeacon`, `TestOrbitalPurchaseDropsPod`, `TestCommsGiftRaisesGoodwill`, `TestCommsRequestCaravan`, `TestTradeSaveRoundTrip`, `TestTradeNilBlockKeepsHash` (Phase 0), `TestTradeSameSeed`, `TestEncodeTradeOfferRoundTrip` (Phase 0).
- UI: `TestTradeDialogTotals`, `TestTradeDialogStepperClamps`, `TestTradeDialogEmitsProposeTrade`, `TestCommsDisabledReasons`, `TestTurretDrawAimFrame`, `TestTurretArtCoversRenderer`.

## Risks

- Enums are append only and the blocks are nil when absent: no `SaveVersion` bump.
- New randomness only on the new streams.
- Goldens of set-up colonies may move once traders join the incident pool (Phase 1): re-pin with a note.
- Turret search is cadenced and indexed; benchmark it.
- The sim runs while a dialog is open, so `Settle` revalidates against live counts.
- Embrasures and pack animals are deferred follow-ups.
