# prison package

## Package Purpose
The colony's prisoners: each one's state (interaction mode, resistance, next visit), a prisoner's life on the raid roster (needs, healing, sleeping in its bed, pottering about its room, a released pawn or visitor walking out), and the warden's work (meals, visits, resistance, recruiting, release, execution). A consumer sub-package of `internal/application/game` (`.cursor/SOLID_GO.md` §10); capture and arrest stay in `care` (SIM-062), the prisoner read models and `makePrisoner` in the game's `prisoners.go`.

## Owned Concepts
- `Prison` (prison.go): built by `New(Host)`; `Is`, `State` (the tab's read model and tests), `Take` (a new prisoner), `Release`, `Drop`, `SetMode`, `Export`/`Import` (the save's `Prisoners` field, copied by the game in id order).
- `Live(id, now)` (life.go): one tick of a prisoner, released pawn or visitor; `FoodBand` and its `FoodLow`/`FoodFed` hysteresis.
- `Assign(id)`, `Do(id, j)`, `Chat(w, q, now)` (warden.go): the warden work type's assignment, the visit job and one chat.
- `Held`: the value view of the game's raid-roster record a prisoner lives on.
- Prison breaks (CBT-4, escape.go): each prisoner rolls hourly (`EscapeMTB` 60 days, original; none for `EscapeGrace` after its capture); the starter's room always joins, other rooms within 20 tiles half the time (wiki); escapees stop being prisoners and turn hostile (`Escapes.Escape`: the raid roster's `Escaping`, raid/tactics.go fights its way out and flees); the prisoners left calm for `EscapeCalm` (`prisoner.State.Calm`, saved). A released pawn's walk out (`walkOut`) moved here from the game (`Escapes.Exit`, `Depart`).

## Allowed Dependencies
- Domain value types (`pawn/prisoner`, `pawn/health`, `pawn/needs`, `ai/job`, `sim/entity`).
- Nothing from `internal/application/game` or its other sub-packages: the roles in `deps.go` are the only way in.

## Forbidden Dependencies
- `internal/application/game`, adapters, platform, Ebitengine, global RNG, wall clock.

## Public Contracts
- `deps.go`: `Host` embeds seven roles, each ≤ 8 value-only methods: Bodies (6), Roster (7), Life (7), Cells (7), Wardens (5), Talk (6), Fate (4), Escapes (5, CBT-4). The game implements them in `internal/application/game/prison_host.go`, reusing its care adapters where the methods match; none adds a method to `game.Game`.
- Numbers: resistance, recruit chance and visit interval come from `pawn/prisoner` (wiki, ENG-003). The work ticks, food band and stroll tuning are original.

## Invariants
- Every roll comes from `Host.Roll` (the game's prison stream); no wall clock; the state map is only indexed, never ranged (the save sorts it).
- Hysteresis: the food band asks for a meal at or below `FoodLow` and stops at or above `FoodFed`; a failed food run waits `foodRetry`; visits are `prisoner.VisitInterval` apart.

## Tests
- Unit tests here run against `fakeHost` (host_test.go); no-flap test `TestFoodBandDoesNotFlap`; `TestPrisonBreakCalm` (escape_test.go).
- Whole-game tests: `internal/application/game/prisoner_test.go`, `feed_patient_test.go`, `turret_test.go`.

## Hot Paths
- `Live` runs per prisoner, released pawn and visitor per tick: a handful of value host calls. `Assign` runs per idle warden check and scans the raid roster once.
