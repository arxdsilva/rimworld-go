# combat package

## Package Purpose
Fighting: line of sight and cover, attack resolution and the attack/hunt job, defending against attackers (fight or flee), spike traps, raiders breaching walls and doors, and the sentry turrets. A consumer sub-package of `internal/application/game` (`.cursor/SOLID_GO.md` §10), created by DEF-0; the gap backlog is `.cursor/DEFENCE_GAP_ANALYSIS.md`. Raids themselves (`raids.go`, `raid_plans.go`, `hive.go`) stay in the game until DEF-4.

## Owned Concepts
- `Combat` (combat.go): the root, built by `New(Host, Roles)`; components as exported fields and no methods of its own (a feature's entry points go on its component).
- `Sight` (combat.go): `Line` (Bresenham, endpoints excluded; rock, walls and doors block), `CanHit` (range, then sight for guns past touching distance), `Cover` (best object-layer cover beside the target, facing the shooter). `SwitchMargin`.
- `Strikes` (strike.go): `Target` (attackable: living animal, standing humanlike, sentry turret), `Strike` (main stream) / `StrikeRoll` (a brawl's social-stream roll), `Do` (Attack and Hunt jobs: close in, strike on cooldown), `AutoEngage` (an idle drafted pawn). `AimTicks(d)`: the warm-up, `Cooldown/3` (the domain's new `weapon.Def.Warmup` holds the same values; switch in DEF-1).
- `Guard` (defend.go): `Defend` (armed fight back, unarmed flee and calm down), `NearestThreat`, `ThreatTo`, `InCombat`, `Reach` and the reach/release constants.
- `Traps` and `Breach` (defense.go): `At`, `Has`, `Spring`, `Underfoot`, `Victim`; `Start` (plan through walls, doors and, for sappers, rock), `Do` (blows on cooldown; rifle butt; a sapper digs).
- `Turrets` (turret.go, PLAY-024): `Phase`, `Targetable`, `Blast` (from the game's building damage), `ForRaider`, `At`, `Firing`, `Aim`, `View`, `Export`/`Import` (saved as `TurretSave`).

## Allowed Dependencies
- Domain value types (`combat/weapon`, `combat/projectile`, `building/construction`, `animal`, `pawn/body`, `pawn/health`, `economy/item`, `ai/job`, `ai/pathing`, `sim/entity`).
- Nothing from `internal/application/game`: the roles in `deps.go` are the only way in.

## Forbidden Dependencies
- `internal/application/game`, adapters, platform, Ebitengine, global RNG, wall clock.

## Public Contracts
- `deps.go`: `Host` embeds eight value-only roles, each ≤ 8 methods: Creatures (7), Rosters (6), Ground (8), Skills (5), Jobs (5), Harm (7), Events (7), Dice (2). `Roles` carries Power (4), Flight (6), Hunts (3) and Breaches (2). The game implements them in `internal/application/game/combat_host.go`; none adds a method to `game.Game`.
- On the game, behind roles: wounds, death and the harm cascade (`hurt`, `hurtAs`), building damage, rebuild blueprints and repair (`damageBuilding`, `assignRepair`), the calm window (`g.calm`, also cleared by lying down and stashing), presentation stamps (`g.traces`, `g.lastStrike`, `g.lastHit`), the hostile-map and breach-map path queries, and hunting work (`assignHunt`).
- Saved state keeps gob field names: the save's `Turrets` field (`Version`, `Aims` of `ID, Target, Dir, ShotAt, ReadyAt, SeenAt`); nil while no turret aims.
- Numbers: original tuning (cover from `construction.CoverPct`, weapons from `combat/weapon`, turret tuning from `.cursor/PLAY024_PLAN.md`).

## Invariants
- Every roll comes from `Dice`: strikes and traps on the game's main stream, turrets on the defence stream; a brawl passes its own social-stream roll to `StrikeRoll`. No wall clock.
- Behaviour-identical to the pre-extraction game code: `cmd/benchmark -hash` pins (grown/raid × 250/500) unchanged at DEF-0.
- Hysteresis: a turret locks within `TurretLock` and lets go past `TurretRelease` or after `TurretLost` ticks out of sight; an armed colonist (or a raider choosing a turret) switches targets only for one `SwitchMargin` closer; a fleeing colonist keeps running to `FleeRelease` and holds back `FleeCalm` ticks.

## Tests
- Unit tests here run against `fakeHost` (host_test.go); coverage ≈ 98 % at DEF-0. No-flap tests: `TestDefendSwitchDoesNotFlap`, `TestTurretLetsGo`, `TestReachHysteresis`.
- Whole-game tests: `internal/application/game/turret_test.go`, `allies_test.go`, `combat_test.go`, `combat_state_test.go`, `defense_test.go`, `flee_test.go`, `fight_test.go`.

## Hot Paths
- `Guard.Defend` runs per undrafted colonist per tick: the attacking-animal scan fills a reused `[]Seen` through `Rosters.Near` and checks candidates the way `nearestWhere` does (only one that would beat the best); the raider loop is one `RaiderOn` call per raider.
- `Sight.Line` runs per tile (turret scans, allies, light, bonds, social): two value host calls per tile (`Rock`, `Object`). `Turrets.Phase` runs per turret per tick, scanning every `TurretScanEvery` ticks.
- Host calls pass values only (§7); `go build -gcflags=-m=2` shows no caller local escaping through them, and no closure in the package escapes.
