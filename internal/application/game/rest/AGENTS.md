# rest package

## Package Purpose
Sleeping: where a pawn sleeps (its own bed, its partner's double bed, the nearest free bed, the floor), who owns which bed, the sleep job driver and when a sleeper gets up, and the thoughts it wakes with. A consumer sub-package of `internal/application/game` (`.cursor/SOLID_GO.md` §10), created by BED-0; the gap backlog is `.cursor/BEDS_GAP_ANALYSIS.md`.

## Owned Concepts
- `Sleep` (sleep.go): the root, built by `New(Host)`. Named `Sleep`, not `Rest`, so it never reads as `care.Rest`. `Assign` (bed, else floor), `InPlace` (the floor), `Do` (the sleep job driver, one tick), `done` (when a sleeper gets up), and the rested marks (`RestedFree`, `MarkRested`, `Unmark`, `Export`/`Import`, saved as `RestedFree`). Kept small: a feature's entry points go on its component.
- `Beds` (beds.go): bed choice (`SleepIn`: partner's double bed, own bed, nearest free bed it may claim; prison and medical beds skipped) and `AssignOwner` (the `command.AssignBedOwner` handler).
- `Waking` (waking.go): the thoughts on waking (in bed or on the ground, outside, awful or nice room).

## Not Here
- Bed rest for patients and the medical-bed holds: `care/rest.go`.
- Room detection and room score: `game/rooms.go` (read through the `Rooms` role).
- Lovin': `game/beds.go`, with the social layer (its cooldown is saved with the gatherings, its roll is on the social stream).
- Where a pawn slept (`sleptAt`): kept on the game, since orders, surgery, prisoners and caskets write it too; read and written through `Bedding`.

## Allowed Dependencies
- Domain value types (`sim/entity`, `ai/job`, `ai/pathing`, `building/construction`, `pawn/health`, `pawn/needs`, `pawn/mood`, `pawn/schedule`).
- `care.IsMedicalBed`, the one rule of what a medical bed is.
- Nothing from `internal/application/game`: the roles in `deps.go` are the only way in.

## Forbidden Dependencies
- `internal/application/game` (the guard's `imports` rule), adapters, platform, Ebitengine, global RNG, wall clock.

## Public Contracts
- `deps.go`: `Host` embeds seven value-only roles (Bodies, Bedding, Claims, Routine, Patients, Rooms, Events), each ≤ 8 methods. The game implements them with adapter types in `rest_host.go`; none adds a method to `game.Game`.
- Saved state keeps its gob field name: `RestedFree` in the game's save struct, filled from `Export` and restored with `Import` (nil keeps the live map, for older saves).

## Invariants
- No randomness, no wall clock; beds are walked in the host's building order with ties broken by the lower id.
- Sleep starts at `needs.CriticalRest` (or bedtime) and ends at `needs.WakeRest`, `needs.Full` in Sleep hours, or the minimum sleep in Work hours: never the same line (no-flap test `TestWakeDoesNotFlap` here, and the whole-game ones in `game/sleep_test.go`).
- Each bed rule keeps what a couple already holds, so the bed never flips between nights.

## Tests
- Unit tests run against `fakeHost` (host_test.go); coverage 100 % at BED-0.
- Whole-game tests (sleep schedule, no-flap, beds, couples, areas, room thoughts, save round trips) stay in `internal/application/game`.

## Hot Paths
- `Sleep.Do` runs every tick for each sleeper; `Beds.SleepIn` walks the building ids once per sleep assignment. Host calls take and return values only (no caller local escapes through an interface call).
