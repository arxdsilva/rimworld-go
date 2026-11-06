# ENG-006: sentry turrets, trade, radio console (PLAY-024, phases 0–3)

The user asked for RimWorld-style communications and defence devices, with full trade. All numbers are original tuning (`.cursor/PLAY024_PLAN.md`); all names are original.

## Sentry turret
- **Building and power:** construction `SentryTurret` (steel, 140 HP, cover 30 %), research Sentry guns. It draws 120 W while holding a target and 40 W idle, in a defense power class that is shed last.
- **Weapon:** `weapon.SentryGun` (range 10, damage 9, cooldown 60).
- **Targeting** (`game/turret.go`):
  - Searches every 20 ticks, staggered, through the creature index plus line of sight, for standing raiders and hostile animals. Never visitors, traders or prisoners.
  - Locks on within 10 tiles; lets go past 12 tiles, after 20 ticks out of sight, or when the target goes down.
  - Aims for 30 ticks, then fires every 60 ticks, rolled on `streamDefense` through `hurtAs(…, 0, …)`.
  - Unpowered: it holds no target and doesn't fire.
- **Destroyed:** explodes (12 to creatures within 2 tiles, 25 to buildings, may ignite).
- **Raiders:** go for a powered turret that is closer than the nearest colonist by a margin.
- **Rendering:** base, plus a gun frame by `AimDir` (16 frames, 0 = south, clockwise), recoil, muzzle flash from building traces, glow only when powered.
- **Inspector:** Target / Idle / No power, and a 10-tile ring.
- `BenchmarkTurretsRaid`: about 13 µs per tick for 20 turrets against 30 raiders.

## Trade
- **Domain** (`economy/trade`, `faction/diplomacy`):
  - prices in hundredths of a silver; the markup and cut fall with Social and goodwill; no arbitrage;
  - three trader kinds with deterministic stock;
  - `Settle` validates stock, wants and affordability;
  - gifts, aid and caravan requests with cooldowns.
- **Caravans:** a trader with 1–3 guards for 0.75 day, through the visitor code. Killing or capturing the trader drops its goods forbidden and costs 40 goodwill.
- **Orbital ship:** 1.5 days, only with a powered radio console. Tradable goods must be within 6 tiles of a trade beacon; purchases arrive by drop pod.
- **Sessions:** the "Trade with X" and "Use radio console" orders hold a session until the trader leaves, the negotiator goes down, or the console loses power. `ProposeTrade` carries the encoded offer in `Command.Name`.
- **Dialogs:** a trade dialog (two-sided table, steppers, totals, gift mode, validation) and a comms dialog (factions, gift, request caravan, call ship; military aid in ENG-007).
- **Alerts:** "Trade ship in orbit (Nh left)" and "Trader caravan visiting (Nh left)", from `Game.TradeStatus()`.
- **Save:** `Trade` and `Turrets` blocks, nil when unused. No version bump.

## Lead fixes during integration
- Trader caravans need a visiting faction instead of a full new-game setup. Legacy colonies have none, so golden runs are unchanged.
- `DevAddFactions` dev hook for capture scenarios.
- `worldSeed()` replaces nil-unsafe reads of `ng.setup.Seed`.
- Visitors no longer show in the colonist bar.
- The caravan alert no longer fires for a ship alone.
- New dev scenarios `turret` and `trade`.

## Verify
- All named tests pass (domain coverage: trade 98 %, faction 98 %).
- The full suite passes under `-race`.
- `TestGrownGolden` was re-pinned once in phase 0: complete-all-research now covers the two new projects. Verified on a copy without them.
- Captures: `-scenario turret` and `-scenario trade` (session scratchpad, `look/turret.png` and `look/trade.png`).

## Left open
- Military aid: done in ENG-007.
- Embrasures.
- Trader pack animals.
- Trade dialog search, sort and category filters.
