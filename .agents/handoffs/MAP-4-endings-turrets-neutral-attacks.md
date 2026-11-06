# MAP-4: the everyone-gone ending, hostile turrets, neutral attacks

Settles the three questions MAP-2 and MAP-3 left open for the user (decided 2026-11-02, all as recommended). Gap rows MAP-13 to MAP-16 in `.agents/gaps/MULTIMAP_GAP_ANALYSIS.md`.

## What changed
- Ending (MAP-14): `AX.EveryoneGone` from `everyoneGone` (`world_stash.go`): no living colonist on a map, on the road, in a pod or casket, due to land, or away on the ship. Prisoners, animals and the kidnapped don't count. `hud/ending.go` opens modal `ending.gone` with Keep watching, Load game and Main menu; a latch stops it reopening until colonists have come back. It also fires when everyone dies, which had no flow at all.
- Abandon: the last colony can be abandoned from the world view (the UI used to refuse what the simulation allowed); its confirm says the game ends when nobody would be left.
- Turrets (MAP-13): `Building.Hostile`, set by `Settle`. Off every grid (`powerCache.hostile`; `Power.Armed` runs `Turrets.Phase` on a map with no defence of ours). They aim at colonists and tame animals; settlement defenders are safe near the muzzle (`raidersSide`). The player can't claim, order, rearm, move, deconstruct or uninstall one; the inspector shows only a status line. Drafted colonists can order an attack on one or shoot it with fire at will. Defeat ignores them, and they keep firing after the fall.
- Neutral attacks (MAP-15): the caravan inspector's refusal is gone, so caravans match pods and the simulation; a non-hostile base asks first (modal `world.attack`), from both. The tip no longer says the relation is permanent.
- Goodwill (MAP-16): `Faction.Besieged`, set from the atlas by `Lifecycle.Flush`. `Adjust` and `DailyDrift` ignore rises; the radio refuses a gift with a reason, so silver isn't spent; a pod gift lands for no goodwill.
- ENG-003 on the root `AGENTS.md` now names STK, AX, WG and MAP.

## Tests
`journey_gone_test.go`, `hud/ending_test.go`, `TestEveryoneGone`, `combat/turret_hostile_test.go` (with `TestHostileTurretDoesNotFlap`), `settlement_turret_test.go`, `TestGoodwillCannotRiseDuringAttack`, `TestBesiegedGoodwillCannotRise`, `TestAttackConfirmsAndAbandonWarns`. `TestJourneyWorldEnterAndRefusedGizmos` and `TestCaravanGizmosCarryTheirReasons` dropped their "attack is refused" assertions, because the rule changed; no golden moved.

## Gaps
- New wanderers, visitors reacting to a hostile faction and shut compound doors were closed by MAP-5 (`MAP-5-doors-visitors-wanderers.md`).
- `worldview.attackGoodwill` copies `maps.attackGoodwill`, because no read model carries it. `hud.goneLoadReason` copies `options.CommitLoadReason`, because `hud` may not import `options`.
- A spent settlement turret stays quiet for good; nobody rearms it (original simplification).
