# internal/application/game/rage

## Purpose
The animals' fight-or-flight brain: manhunter rage, tame guards, hunts, charges, fighting back, fleeing and the chase warnings. A consumer sub-package of `internal/application/game` (`.agents/specs/SOLID_GO.md` §10), created as a pure, hash-neutral extraction of the game's `handling.go` (rage, guards, manhunter packs), `animals.go` (flee, fight and charge checks, prey), `animal_flee.go` and `chase.go`, to free room in the game for the enemy-AI gaps (EAI-001..007).

## Owns
- `Rage` (rage.go): built by `New(Host, *Memory)`; its only state is the saved `Memory` and per-call scratch (`probes`, a reused `pathing.Searcher`). `Enrage`, `Provoked`, `Chase` (close in, bite on cooldown, break through: bash.go), `ManhunterPack(points)` (one biome species, `animal.PackSize`, EAI-005), `Pack(n)`, the pure `ManhunterSpecies`, the legendaries' rally, `BadlyHurt` (T10), `Forget`.
- target.go (EAI-002): `ThinkManhunter` and the foe pick: standing people (colonists, guests, raiders) and tame animals through the index in widening rings, reachable first (`reachProbes` reach checks), locked in `Memory.Lock`.
- bash.go (EAI-001): `Bashing`; a shut-out breaker (a manhunter, a hunting predator, a hive insect) finds the first door (an insect: also wall) on its way through `Siege.BlockAt`, holds it in `Memory.Bash`, stands by it and strikes it (`Siege.Bash`); gives up after `BashGiveUp`.
- herd.go (EAI-003): `HerdCall` (from the game's `animalHurt`), `Defending`.
- guard.go: `GuardCheck`, `GuardBreaksHunt` (PLAY-017), `CombatAlive`.
- fight.go: `FightCheck` (hunt, revenge, territorial chase; T5 give-up, T10 break-off for `BreakOffTicks`), `ChargeCheck` (PLAY-010 2.3 C), `FindPrey`, `Huntable`, `HuntRadius`, pack calls, `EndFight`.
- flee.go: `FleeCheck`, `NearestPredator`, `HurtBlind`, `CanFightBack`, `FightBack`, `ValidFoe`, `FleeHit`, `FleeSpot`, `ReachesFoe`, `DistSq` (PLAY-010 2.3 A, B).
- chase.go: `ChaseTarget`, `NoteChase`, `WarnChase`, `KeepChaseWarned`, `ChaseText`, `WithArticle` (PLAY-017).

## Depends on
- `game/combat` (`FleeCell`, pure), `game/evlog`, `ai/pathing` (bash.go's breach search), `animal`, `combat/weapon`, `sim/clock`, `sim/entity`, `pkg/msgfmt`.

## Must not depend on
- `internal/application/game`: the roles in `deps.go` are the only way in.

## Contracts
- `deps.go`: `Host` embeds seven value-only roles, each ≤ 8 methods: World (8: clock, dice, names, log, alarm, threat), Beasts (8), People (7: with `DownedPawn`), Ground (8: the creature index's `Nearest`/`Within`, the walk, layer reach, fire), Story (4), Fight (2), Siege (2: `BlockAt`, `Bash`, EAI-001). The game implements them with `rageLook` and `rageAct` at the end of `game/handling.go`; `rageOf(g)` is a free function, not a `Game` method. `Kind` bits equal the game's index bits (`TestRageKindsAreIndexKinds`).
- Allocation-free on the hot path: the animal under thought crosses the host by value (`GoTo` and `FireThreat` take and return `animal.State`), never as a pointer (a pointer handed to an interface escapes, and every animal would move to the heap every tick: `TestWorldViewQuietTickAllocs`); sight filters are `Sight` values whose `Keep` is a plain function, never a closure.
- On the game, behind roles: the animal store and positions, `animalGoTo`, `animalPathQuery`, `layerReach` and `penQuery` (fences.go), `tameHuntOn`, `sleeping`, difficulty (`revengePct`, `predatorsHuntColonists`), species picks and the map edge, strikes (`combat.Strikes`).
- Per-animal state stays in `animal.State` (at its 60-field ceiling): add no fields for this package.
- `Memory` (memory.go) is the brain's saved memory, written by bash.go, target.go and herd.go: per-animal `Bash` and `Lock` maps and the herds' `Call` map, exported for gob and the state hash, kept as `wildlifeState.Rage`. Read it by key; never range over it on a path that rolls or orders work (sort the keys first). Drop an animal's entries with `Forget`.

## Invariants
- Hash-neutral: every roll comes from `World.Roll` (the main stream) in the order the game rolled before the extraction; the benchmark state hashes did not move.
- No map iteration; sight queries follow the creature index (ties to the lower id), `Within` ascending id.
- Hysteresis (domain constants, never retuned here): a fleeing animal runs on until `def.FleeRadius + animal.FleeRelease` clear, keeps a goal at least as far from the threat and re-picks only every 40 ticks (`TestTameAnimalFleesWithoutFlapping`, `TestFleeingDustbuckRunsToItsGoal` in `game/animals_flee_test.go`); a foe it cannot reach is no reason to run or fight (`TestFighterFleesFoeItCannotReach`, `TestTerritorialReactionDoesNotFlap` in `game/animal_react_test.go`); a hunt gives up after `animal.HuntGiveUp` and retries after `animal.HuntRetry` (`TestPredatorGivesUpChase`, `game/animal_needs_test.go`); a charge rolls once per `animal.ChargeCooldown` and ends past `animal.ChargeLeash` (`TestChargeCooldownHolds`, `game/animal_charge_test.go`); a chase is warned again only `chaseRewarn` (an hour) after it ended (`TestChargeWarnsOnceAcrossRepeatedCharges`, `game/chase_test.go`); rage lasts `animal.ManhunterTicks` (a grieving tame pet's too: `TestGrievingPetCalmsNoFlap`); a bash is held on one block until it falls, the way opens or `BashGiveUp`, then waits for the path retry (`TestAnimalBashNoFlapOnHeldDoor`); a manhunter's foe is locked `lockTicks` and switched only for one `retargetMargin` tiles closer (`TestManhunterRetargetNoFlap`); a herd calls at most once per `callEvery` (`TestHerdDefenceNoFlap`); the game's guards keep a foe at livestock inside their keep band (`TestLivestockDefenceNoFlap`).

## Hot paths
- `FleeCheck`, `ChargeCheck`, `GuardCheck` and `FightCheck` run per animal per tick from the game's `thinkAnimal`: value host calls only; sight queries every `animal.ThreatEvery` ticks (charges every `animal.DecideEvery`), staggered by id, and every tick only while fleeing.

## Tests
- Unit tests here: `rage_test.go` (chase text, `KeepChaseWarned`, `FightBack`, `HurtBlind`, `HuntRadius`, `DistSq`).
- EAI whole-game tests in the game: `eai_bash_test.go` (EAI-001/002), `eai_herd_test.go` (EAI-003/004), `eai_pack_test.go` (EAI-005..007); `animal/pack_test.go` (`TestPackSizeScalesWithPoints`).
- Whole-game tests stay in the game: `chase_test.go`, `animal_charge_test.go`, `animal_react_test.go`, `animals_flee_test.go`, `flee_test.go`, `animal_needs_test.go`, `animal_downed_test.go`, `handling_test.go`, `train_test.go`, `wildlife_test.go`.
