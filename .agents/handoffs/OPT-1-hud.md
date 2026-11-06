# OPT-1 hud: clock and colonist bar options

Scope: OPT-008, 011, 012, 013 (`hud/`, `hud_compat.go`).

- `hud/clock_text.go`: `ClockText` (24 h / 12 h), `WallText`, `hourLine`. The wall clock enters as `hud.Env.Wall`, filled by `hudHost.Env` from `App.now` (the pinnable presentation clock). `Global.HourLine` shows "08:12 (14:07)  Morning · work" with ClockRealtime, in 12 h with Clock12h, ellipsized to `GlobalInnerW`.
- `hud/colonistbar.go`: `showWeapon` / `weaponRect` / `drawWeapon`: the held-weapon sprite (`creature.HeldWeaponKey`) in the portrait's bottom-left corner for BarWeapons never/drafted/always; fists show nothing. The mood fill needs `Interface.BarMood`.
- OPT-013: the HUD menu toggle already reads `Gameplay.CategorizedResources`, the same setting as `resources.go:157`; no change.
- Tests: `hud/clock_options_test.go` (clock strings, realtime, weapon modes, icon fits slot at UI 75-200 %).
- Not run: root package tests (build broken by other agents' in-flight edits at the time). Hash impact: none.
