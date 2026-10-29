# rest package

## Package Purpose
Sleeping: where a pawn sleeps (its own bed, its partner's double bed, the nearest free bed, the floor), who owns which bed, the sleep job driver and when a sleeper gets up, and the thoughts it wakes with. A consumer sub-package of `internal/application/game` (`.cursor/SOLID_GO.md` §10), created by BED-0; the gap backlog is `.cursor/BEDS_GAP_ANALYSIS.md`.

## Owned Concepts
- `Sleep` (sleep.go): the root, built by `New(Host)`. Named `Sleep`, not `Rest`, so it never reads as `care.Rest`. `Assign` (bed, else floor), `InPlace` (the floor), `Do` (the sleep job driver, one tick), `done` (when a sleeper gets up), and the rested marks (`RestedFree`, `MarkRested`, `Unmark`, `Export`/`Import`, saved as `RestedFree`). Kept small: a feature's entry points go on its component.
- `Beds` (beds.go): bed choice (`SleepIn`: partner's double bed, own bed, nearest free bed it may claim; prison and medical beds skipped) and `AssignOwner` (the `command.AssignBedOwner` handler).
- `Waking` (waking.go): the thoughts on waking (in bed or on the ground, outside, awful or nice room).
- `Stats` (stats.go, BED-1): comfort, rest effectiveness and beauty of beds and bed furniture by kind, stuff and quality (tables in `construction/furniture.go`, wiki numbers per ENG-003; quality is Normal until PRD-2 puts it on buildings), plus the end table (cardinally beside a head cell) and dresser (within 6 tiles, same room) links, +0.05 comfort each, one of each. `RestEffect` scales the sleep tick (`needs.SleepTickAt`; the ground is 0.8). `Of` fills the read model's `BuildingView` (Comfort, RestEffect, Beauty, Facilities bits = `Links`, Forbidden) and allocates nothing.
- `Animals` (animals.go, BED-1): a pet's animal bed or spot (`Building.Claimant` is the animal; its own first, else the nearest free one in its area; a holder no longer tame on the map frees it), chosen only on the bucket decision's bed-down (T6, `game/animal_needs.go`); `SleepRate` scales a pet's sleep in its own bed by the bed's rest effectiveness over the ground's (wild animals and pets on the ground unchanged). Penned livestock never use beds.
- `Lovin` (lovin.go, moved from `game/beds.go` by BED-1): couples' lovin' in a shared double bed (double bed or double bedroll). The cooldown stays saved with the gatherings and the roll on the social stream (the `Couples` role).
- Forbidden beds (BED-1): `Building.Forbidden` (set by `command.SetForbidden` on a building, `game/forbid.go`) keeps `Beds` and `Animals` off the bed.

## Not Here
- Bed rest for patients and the medical-bed holds: `care/rest.go`.
- Room detection and room score: `game/rooms.go` (read through the `Rooms` role).
- Lovin's cooldown (saved with the gatherings) and roll (social stream): on the game, read through `Couples`.
- Medical animal beds (tending animals in a bed): not yet (BED-1 follow-up).
- Where a pawn slept (`sleptAt`): kept on the game, since orders, surgery, prisoners and caskets write it too; read and written through `Bedding`.

## Allowed Dependencies
- Domain value types (`sim/entity`, `ai/job`, `ai/pathing`, `building/construction`, `pawn/health`, `pawn/needs`, `pawn/mood`, `pawn/schedule`, `pawn/social` (the lovin' curve), `economy/quality`).
- `care.IsMedicalBed`, the one rule of what a medical bed is.
- Nothing from `internal/application/game`: the roles in `deps.go` are the only way in.

## Forbidden Dependencies
- `internal/application/game` (the guard's `imports` rule), adapters, platform, Ebitengine, global RNG, wall clock.

## Public Contracts
- `deps.go`: `Host` embeds eight value-only roles (Bodies, Bedding, Claims, Routine, Patients, Rooms, Events, Couples), each ≤ 8 methods; the guard allows no more than 8 embedded, so a new need goes on the role it fits. The game implements them with adapter types in `rest_host.go`; none adds a method to `game.Game`.
- Saved state keeps its gob field name: `RestedFree` in the game's save struct, filled from `Export` and restored with `Import` (nil keeps the live map, for older saves).

## Invariants
- No randomness, no wall clock; beds are walked in the host's building order with ties broken by the lower id.
- Sleep starts at `needs.CriticalRest` (or bedtime) and ends at `needs.WakeRest`, `needs.Full` in Sleep hours, or the minimum sleep in Work hours: never the same line (no-flap test `TestWakeDoesNotFlap` here, and the whole-game ones in `game/sleep_test.go`).
- Each bed rule keeps what a couple already holds, so the bed never flips between nights.

## Tests
- Unit tests run against `fakeHost` (host_test.go); coverage 100 % at BED-0 and BED-1.
- Whole-game tests (sleep schedule, no-flap, beds, couples, areas, room thoughts, save round trips) stay in `internal/application/game`; BED-1's are in `bed_stats_test.go`.

## Hot Paths
- `Sleep.Do` runs every tick for each sleeper (one building lookup for the rest effectiveness); `Beds.SleepIn` and `Animals.Bed` walk the building ids once per sleep assignment; `Animals.SleepRate` runs every tick for each sleeping pet (one cell lookup); `Stats.Of` runs per building per snapshot (cell lookups only for beds, no allocation). Host calls take and return values only (no caller local escapes through an interface call).
