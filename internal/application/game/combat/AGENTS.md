# combat package

## Package Purpose
Fighting: line of sight and cover, attack resolution and the attack/hunt job, defending against attackers (fight or flee), spike traps, raiders breaching walls and doors, and the sentry turrets. A consumer sub-package of `internal/application/game` (`.cursor/SOLID_GO.md` §10), created by DEF-0; the gap backlog is `.cursor/DEFENCE_GAP_ANALYSIS.md`. Raids themselves (`raids.go`, `raid_plans.go`, `hive.go`) stay in the game until DEF-4.

## Owned Concepts
- `Combat` (combat.go): the root, built by `New(Host, Roles)`; components as exported fields and no methods of its own (a feature's entry points go on its component).
- `Sight` (combat.go): `Line` (Bresenham, endpoints excluded; rock, walls and doors block), `CanHit` (range, then sight for guns past touching distance), `Cover` (best object-layer cover beside the target, facing the shooter). `SwitchMargin`.
- `Strikes` (strike.go): `Target` (attackable: living animal, standing humanlike, sentry turret), `Strike` (main stream) / `StrikeRoll` (a brawl's social-stream roll), `Do` (Attack and Hunt jobs: close in, warmup, burst, cooldown; `job.Record.Count` holds the burst shots left), `AutoEngage` (an idle drafted pawn), `Report` (hit chance with its factors: `attack.Report`, the game's `HitChance` read accessor). `AimTicks(d)` is `d.Warmup`; `CycleTicks(d)` the wait after an attack. Hits pass worn armor (`armor.Hit`; a halved sharp hit bruises).
- `Shots` (shot.go, DEF-1): projectiles in flight. Decided at the muzzle (aim, wild miss within `attack.MissRadius`, cover catch), flown cell by cell on the line-of-sight line by `Step` (once a tick, after the turrets): rock, walls and doors stop them, creatures in the way may take them (`attack.InterceptChance`; allies within `attack.FriendlySafe` of the muzzle are safe), an aimed shot strikes a target within `Tracking` of its destination, anything else strikes something in the landing cell `attack.StrayHit` of the time. `Views` (WorldView.Shots), `Export`/`Import` (the save's `Shots`).
- `Guard` (defend.go): `Defend` (armed fight back, unarmed flee and calm down), `NearestThreat`, `ThreatTo`, `InCombat`, `Reach` and the reach/release constants.
- `Traps` and `Breach` (defense.go): `At`, `Has`, `Spring`, `Underfoot`, `Victim`; `Start` (plan through walls, doors and, for sappers, rock), `Do` (blows on cooldown; rifle butt; a sapper digs).
- `Turrets` (turret.go, PLAY-024): `Phase`, `Targetable`, `Blast` (from the game's building damage), `ForRaider`, `At`, `Firing`, `Aim`, `View`, `Export`/`Import` (saved as `TurretSave`). DEF-3: the autocannon and slug turret (`gunOf`, wiki guns with a minimum range, bursts in `Aim.Burst`), barrel wear (`Arm.Fired`, `BarrelOf`, `Spent`, `Barrel`, `Rearm`), hold fire (`SetHold`, `Held`). `Phase` also steps the `after` phasers (fuses, doors, mortars, rearm hauls) every tick.
- DEF-3 defences: `Sight.CoverAt`/`Cover` (permille, `domain/combat/cover`: buildings, rock, grown trees/cacti/bushes, stone chunks cached for `chunkEvery`); IEDs in `Traps` (ied.go: `IEDFuse` 15 ticks, then `Blasts.Explode` and a re-laid blueprint); `Doors` (door.go: open state from `domain/building/door`; the game hooks `Wait` (movement: a step into a shut door opens it and waits), `HeldOpen` (the shut-door set hostiles path with) and `Barred` (a forbidden door is unwalkable); `SetHold`; only open doors are stepped); `Mortars` (mortar.go: crew call, forced or nearest target in the wiki range band, warmup/cooldown, forced miss by the crew's Shooting, `Shots.Lob`); `Rearms` (rearm.go: idle colonist hauls steel or shells, job RearmTurret, advanced by `Do` from the game's job phase); `Orders` (orders.go: `Apply` for the four DEF-3 commands, `Do` for the two jobs, `View` = BuildingView facts); `Hunting` and `Tiles` (hunt.go, tiles.go: moved unchanged from the game for the package budget).

## Allowed Dependencies
- Domain value types (`combat/weapon`, `combat/projectile`, `combat/attack`, `combat/armor`, `combat/damage`, `building/construction`, `animal`, `pawn/body`, `pawn/health`, `economy/item`, `ai/job`, `ai/pathing`, `sim/entity`).
- Nothing from `internal/application/game`: the roles in `deps.go` are the only way in.

## Forbidden Dependencies
- `internal/application/game`, adapters, platform, Ebitengine, global RNG, wall clock.

## Public Contracts
- `deps.go`: `Host` embeds eight value-only roles, each ≤ 8 methods: Creatures (8, with `Armor`), Rosters (6), Ground (8; `AnyBuilt` takes a kind predicate), Skills (6, with `ShotAim`), Jobs (5), Harm (7), Events (7), Dice (2). `Who` has the game's creature-index kind bits. `Roles` carries Power (4), Flight (6), Hunts (7), Breaches (2), Field (6: plants, chunk tiles, building lists, map size, dry walkable, reserved tile) and Supply (3: stock, pick up, spend). The game implements them in `internal/application/game/combat_host.go`; none adds a method to `game.Game`.
- On the game, behind roles: wounds, death and the harm cascade (`hurt`, `hurtAs`), building damage, rebuild blueprints and repair (`damageBuilding`, `assignRepair`), the calm window (`g.calm`, also cleared by lying down and stashing), presentation stamps (`g.traces`, `g.lastStrike`, `g.lastHit`), the hostile-map and breach-map path queries, and hunting work (`assignHunt`).
- Saved state keeps gob field names: the save's `Turrets` field (`Version`, `Aims` of `ID, Target, Dir, ShotAt, ReadyAt, SeenAt, StunnedUntil, Burst`; DEF-3 adds `Arms`, `Mortars`, `Doors`, `Fuses`, `Rearms`); nil while nothing holds state. The save's `Shots` (`[]projectile.Shot`), nil while nothing flies.
- Traces: melee, turret and trap blows are `projectile.Whole`; a shot leaves `Fired` (at the muzzle; Hit = aimed) and `Landed` (Hit, Building, Damage after armor; Hit with Damage 0 = armor stopped it).
- Numbers: wiki numbers (ENG-003) for the knife, club, spear and rifle (the bolt-action rifle), accuracy bands, shooter accuracy per tile, armor, miss radius, stray and intercept rules; wiki cover values (`domain/combat/cover`, DEF-3); original tuning for fists and natural weapons, body sizes of the size classes, `Tracking`, and the turret (`.cursor/PLAY024_PLAN.md`; instant until DEF-3).

## Invariants
- Every roll comes from `Dice`: strikes and traps on the game's main stream, turrets on the defence stream; a brawl passes its own social-stream roll to `StrikeRoll`. No wall clock.
- DEF-1 changed behaviour on purpose: the raid/250 hash moved (grown and raid/500 have no fight in their 3000 ticks).
- DEF-3 hysteresis: hold fire drops the target and a released turret looks again only on its scan tick (`TestTurretHoldDoesNotFlap`); a door closes `door.StayOpen` after its last user (`TestDoorsOpenForColonists`, domain `TestAutoCloseDoesNotFlap`); a mortar crew stays `MortarManMin` and stands down only after `MortarIdleRelease` without a target (`TestMortarCrewDoesNotFlap`); a rearm is asked at a quarter barrel or half a rack and fills to full.
- Hysteresis: a turret locks within `TurretLock` and lets go past `TurretRelease` or after `TurretLost` ticks out of sight; an armed colonist (or a raider choosing a turret) switches targets only for one `SwitchMargin` closer; a fleeing colonist keeps running to `FleeRelease` and holds back `FleeCalm` ticks.

## Tests
- Unit tests here run against `fakeHost` (host_test.go); coverage ≈ 97 % at DEF-1 (shot_test.go: burst, bands, stray, friendly fire, armor, saves). Whole-game: `combat_model_test.go`. No-flap tests: `TestDefendSwitchDoesNotFlap`, `TestTurretLetsGo`, `TestReachHysteresis`.
- Whole-game tests: `internal/application/game/turret_test.go`, `allies_test.go`, `combat_test.go`, `combat_state_test.go`, `defense_test.go`, `flee_test.go`, `fight_test.go`.

## Hot Paths
- `Guard.Defend` runs per undrafted colonist per tick: the attacking-animal scan fills a reused `[]Seen` through `Rosters.Near` and checks candidates the way `nearestWhere` does (only one that would beat the best); the raider loop is one `RaiderOn` call per raider.
- `Sight.Line` runs per tile (turret scans, allies, light, bonds, social): two value host calls per tile (`Rock`, `Object`). `Turrets.Phase` runs per turret per tick, scanning every `TurretScanEvery` ticks.
- `Shots.Step` runs per shot per tick: one `Near` (radius 0) per cell entered, the shot slice and the Near and armor buffers reused (no allocation once grown).
- Host calls pass values only (§7); `go build -gcflags=-m=2` shows no caller local escaping through them, and no closure in the package escapes.
