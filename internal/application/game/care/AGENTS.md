# care package

## Package Purpose
Looking after pawns that cannot look after themselves: the carry relation, rescue to a safe bed, tending and emergency tending, bed rest, the medical care policy, and the pawn-targeted orders (Rescue, Capture, Arrest, Prioritize tending, Feed, Strip). A consumer sub-package of `internal/application/game` (`.cursor/SOLID_GO.md` §10), created by SIM-061; the gap backlog is `.cursor/PAWN_INTERACTION_GAP_ANALYSIS.md`.

## Owned Concepts
- `Care` (care.go): the root, built by `New(Host, Roles)`; components as exported fields; the care level and self-tend commands, `DropPatient`, `FollowCarriers`, `View(id, j) CareView`, `SafeBedFor` (beds.go), `PawnOrders` (orders.go), `Force` and `judge` (forced.go: one verdict for the menu row and the order), `Do` (jobs.go: the SIM-063…065 jobs), `kindOf` (targets.go: colonist, prisoner, guest, stranger, hostile). Kept at ≤ 40 methods: a feature's entry points go on its component, not here.
- `Carries` (carry.go): who carries whom, a two-way index, carriers in ascending order.
- `Policy` (policy.go): care levels per pawn and per group, self-tend; saved as `PolicySave`.
- `Doctor` (rescue.go, tend.go): rescue to a safe bed (every downed cause but collapse-asleep, colonists and guests), tend, emergency tend, medicine fetch, `tendedUntil`, `PatientDied`.
- `Rest` (rest.go): bed rest and the holds on beds for a stay; saved as `RestSave`.
- `Feeding` (feed.go, SIM-064): the Doctor's feed giver for patients who cannot feed themselves, the warden's prisoner food run, "Feed <name>"; retry state saved as `FeedRetry`.
- `PrisonBeds` (prisonbed.go, SIM-063): a warden carries a downed prisoner back to a prison bed; "Take <name> to bed".
- `Hospitality` (guest.go, SIM-063): rescued guests' lifecycle (tended, leave when healed, goodwill, a stranger's join offer).
- `Visits` (visit.go) and `Stripping` (strip.go, SIM-065): visiting the sick; "Strip <name>".
- Capture and arrest (capture.go, SIM-062) with the shared carry-to-bed driver (carrybed.go).
- `Option`: field-identical to `game.OrderOption`.

## Allowed Dependencies
- Domain value types (`sim/entity`, `sim/command`, `ai/job`, `pawn/health`, `pawn/medcare`, `construction`, `economy/item`, `world/pathing`, …).
- Nothing from `internal/application/game`: the roles in `deps.go` are the only way in.

## Forbidden Dependencies
- `internal/application/game` (the guard's `imports` rule), adapters, platform, Ebitengine, global RNG, wall clock.

## Public Contracts
- `deps.go`: `Host` embeds eight value-only roles (Bodies, Roster, Beds, Reach, Jobs, Health, Treatment, Events), each ≤ 8 methods; `Roles` carries the rest as their own interfaces: `Prison` (capture/arrest), `Guests` (guest lifecycle), `Social` (visits), `Gear` (strip), `Meals` (feeding, declared in feed.go). The game implements them with adapter types (`care_host.go`, `care_host_prison.go`, and beside the game code they wrap: `prisoners.go`, `visitors.go`, `social.go`, because the game package's file budget is full); none adds a method to `game.Game`.
- Saved state keeps gob field names (`Carrying`, `TendedUntil`, `Care`, `BedRest`, `FeedRetry`); a nil field loads an older save.
- Pawn-targeted orders are `command.Prioritize` with `Order.TargetsPawn()` and the target in `Command.Target`; reason strings in reasons.go and next to each verb.

## Invariants
- No randomness, no wall clock; maps iterated only in sorted order.
- Every need- or threshold-driven behaviour has hysteresis or a cooldown and a no-flap test.

## Tests
- Unit tests here run against `fakeHost` (host_test.go) plus per-role fakes (fakeprison, fakeguests, fakesocial, fakehost_* files); coverage ≈ 97 % at SIM-066. No-flap tests: `TestRestDoesNotFlap`, `TestFeedDoesNotFlap`, `TestGuestDoesNotFlap`, the visit pair test.
- Whole-game tests (medical, bed rest, care policy, bleeding, rescue, kidnap, save round trips) stay in `internal/application/game`.

## Hot Paths
- `Doctor.DoRescue`/`DoTend`, `Rest`'s per-tick checks and `Care.FollowCarriers` run on the tick path. Host calls pass values only; `go build -gcflags=-m=2` showed no caller local escaping through them (SIM-061). `Carries.Each` walks a sorted slice and never sorts on the tick path.
