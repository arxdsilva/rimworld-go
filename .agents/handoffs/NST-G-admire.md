# NST-G: recreate.Admire

- `recreate.Admire` (fifth Mode) and `SpecOf` rows for DisplayCase, RoundPedestal, Small/Large/GrandSculpture: JoySolitary, 700 permille, 1 user, no quality, no learning, verb "admire". Runs as `job.Recreate`.
- Case and pedestal are usable only while a stack lies on their footprint (`usable`, also checked mid-session; a cut-short session sets the existing RetryTicks wait). Sculptures always. Needs sight. Stood on a bare ring cell (no seats).
- New consumer role `recreate.Displays{Holds(x,y)}` in `deps.go`, part of `Host`; game adapter is one line on `recHost` in `game/rest_host.go` (`itemAt(x,y) != 0`).
- `Seek` now gates on `SpecOf` ok instead of `Kind.Recreation()`, same set for old kinds. `Kind.Recreation()` (room tally, building_view) is untouched: the lead may add the five kinds there for the building card joy line.
- No save, golden or random-draw change. Tests: `TestAdmire`, `TestAdmireNoFlap`, `TestAdmireDeterministic`.
