# Task Handoff

Task ID: UI-023-social-finish
Status: complete (race-tested)

## Summary
Animal inspector layout fix, social dev hooks, a richer social scenario and capture states for the social tabs and windows.

## Screen / UI Impact
- Animal inspector: the log rect now subtracts the bond block (`animalExtraH + animalBondH`), so Master/Bond rows no longer overlap the log line. Training buttons are 134 px wide, two per row (`Obedience 2/3` reads in full); the Log button is 48 px wide. `TestAnimalInspectorNoOverlap` asserts no overlapping or clipped rects.
- Social tab: relation label column widened (92 px) so "acquaintance" is not clipped. Prisoner tab: "Reduce resistance" button reads "Reduce" (tooltip keeps the full text).

## Dev hooks (`internal/application/game/dev_social.go`, bump viewGen, tested)
`DevSetPair(a, b, kind)` (lover, fiance, spouse, exlover, exspouse), `DevAddOpinionMemory(observer, subject, kind)` (insulted, deep_talk, slighted, kind_words, ...), `DevBond(animal, pawn)`, `DevScar(pawn, part)` (permanent wound; "jaw" disfigures). Unused by golden scenarios.

## Scenario
`-scenario social`: Maren and Tobias married (double bed), Eli and Fen engaged (wedding at the marriage spot), Hale and Iris rivals, Biscuit bonded to Maren, Cyrus with a jaw scar (`SocialCast.Scarred`). `TestSocialScenario` asserts all of it.

## Captures
`-scenario social -width 80 -height 60 -capture-zoom 2 -capture-ticks 3600 -capture-ui <state>` with state social (Maren, first row tooltip), health (Cyrus), prisoner (Garr), history, world, assign, inspector (the dog). Code: `capture_social.go`. Screenshots: scratchpad `social_tab.png`, `health_tab.png`, `prisoner_tab.png`, `history_tab.png`, `world_tab.png`, `assign_tab.png`, `social_inspector_fixed.png`.

## Known Limitations
The social scenario has no world map, so the World window shows "no world map". Assign's Area column is disabled until areas exist.
