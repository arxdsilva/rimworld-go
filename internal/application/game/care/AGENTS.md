# care package

## Package Purpose
Looking after pawns that cannot look after themselves: the carry relation, rescue to a safe bed, tending and emergency tending, bed rest, the medical care policy, and the pawn-targeted orders (Rescue, Capture, Arrest, Prioritize tending, Feed, Strip). A consumer sub-package of `internal/application/game` (`.cursor/SOLID_GO.md` §10), created by SIM-061; the gap backlog is `.cursor/PAWN_INTERACTION_GAP_ANALYSIS.md`.

## Owned Concepts
- `Care` (care.go, 9 methods): the root, built by `New(Host)`; components as exported fields; `Level`, `SelfTendAllowed`, the `Set*` care commands, `DropPatient`, `FollowCarriers`, `View(id, j) CareView`, `SafeBedFor` (beds.go), `PawnOrders` (orders.go).
- `Carries` (carry.go, 9): who carries whom, a two-way index with carriers kept in ascending order (`Pick`, `Drop`, `Patient`, `Carrier`, `Forget`, `Len`, `Each`, `Export`, `Import`).
- `Policy` (policy.go, 9): care levels per pawn and per group, self-tend; saved as `PolicySave`.
- `Doctor` (rescue.go, tend.go, 23): rescue to a bed, tend, emergency tend, medicine fetch, `tendedUntil`.
- `Rest` (rest.go, 27): bed rest, its hold on a medical bed, lying down for care; saved as `RestSave`.
- `SafeBedFor(patient, x, y, BedNeed)`: one bed choice reproducing the old rescue and bed-rest rankings (SIM-063 changes the rules); `IsMedicalBed`.
- `Option`: field-identical to `game.OrderOption`, so the game converts with `OrderOption(o)`.

## Allowed Dependencies
- Domain value types (`sim/entity`, `sim/command`, `ai/job`, `pawn/health`, `pawn/medcare`, `construction`, `economy/item`, `world/pathing`, …).
- Nothing from `internal/application/game`: the roles in `deps.go` are the only way in.

## Forbidden Dependencies
- `internal/application/game` (the guard's `imports` rule), adapters, platform, Ebitengine, global RNG, wall clock.

## Public Contracts
- `deps.go`: `Host` embeds eight roles, each ≤ 8 value-only methods: Bodies (8), Roster (8), Beds (7), Reach (4), Jobs (7), Health (8), Treatment (7, one slot kept for SIM-064's `Feed`), Events (2). The game implements them with adapter types in `internal/application/game/care_host.go`; none of it adds a method to `game.Game`.
- Saved state keeps the game's gob field names (`Carrying`, `TendedUntil`, `Care`, `BedRest`), so old saves load unchanged (SIM-061 proved it on cached fixtures: identical state hashes).
- The reason strings `ReasonIncapable`, `ReasonBusyTarget`, `ReasonUnreachable` equal the game's; `TestCareReasonsMatchGame` keeps them equal.

## Invariants
- No randomness, no wall clock; maps iterated only in sorted order.
- Every need- or threshold-driven behaviour has hysteresis or a cooldown and a no-flap test.

## Tests
- Unit tests here run against `fakeHost` (host_test.go); coverage 96.1 % at SIM-061. The no-flap test for bed rest is `TestRestDoesNotFlap`.
- Whole-game tests (medical, bed rest, care policy, bleeding, rescue, kidnap, save round trips) stay in `internal/application/game`.

## Hot Paths
- `Doctor.DoRescue`/`DoTend`, `Rest`'s per-tick checks and `Care.FollowCarriers` run on the tick path. Host calls pass values only; `go build -gcflags=-m=2` showed no caller local escaping through them (SIM-061). `Carries.Each` walks a sorted slice and never sorts on the tick path.
