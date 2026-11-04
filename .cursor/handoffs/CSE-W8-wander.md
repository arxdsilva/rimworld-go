# CSE-W8: wander, snow sky, cold store (CSE-028, 041, 042, 043)

- CSE-028: `snow.Sky.SkipClear`, checked first in `AssignClear`; `snowHost.SkipClear` (weather.go) = `Kind.Falling()` && `SnowWaitsForSky`. A started job is not dropped; only new assignments wait. Tests: `TestClearWaitsForClearSky`, `TestClearSkyNoFlap` (game/snow_sky_test.go). Sand: none exists.
- CSE-041/042: `sense.Wander` now carries `gameplay.Settings` plus `Shelter` and `Manners` roles (sense/deps.go); `senseCourtesy` in sense_host.go. Both off: identical draws and picks (`TestWanderOffDrawsSameNumbers`). Sheltered: roofed Home goal with roofed quarter points of the straight line; under open sky `seekRoof` rings out to radius 12 (<=6 reachability tries, no random draw), else stays put. Jog: no urgency field, walks normally.
- CSE-043: `room.ColdStore` (appended), `FullContents.ColdSides`, `Tally.RoleCold`, `thermo.Model.ColdSides(r)` (cooler thermostat target < 0, from the site cache).
- Not done: `rest_host.go:RoomQuality` and `rooms.go` still call `Tally.Role` (not my files), so RoomView and room_mood do not see ColdStore yet; `Rude` computes the cold role itself. Label shows via `Role.String()` once wired. ShouldHide ignores toxic resistance (no pawn stat exists) and lord duty/home-map (idle pawns only).
- Cost: `Rude` calls `countFacts` (a pass over stacks and buildings) per candidate with PoliteWander on; off by default. A per-room cached fact would fix it.
- Bench `Wander.Pick`: off 44 ns, on-fair 48 ns, on-rain 60 ns, 0 allocs.
