# STK-AL: allow system gap analysis and fixes

User request: "do a gap analysis on the allow system and add it to this change". Read as RimWorld's forbid/allow system for things and buildings plus allowed areas. Programme brief: STK (lead, 2026-11-01).

## Delivered

- `.agents/gaps/ALLOW_GAP_ANALYSIS.md`: 31 rows (15 forbidding things, 5 forbidding buildings, 11 allowed areas) with the vanilla behaviour, our evidence, the gap and the state. 9 gaps closed here (F3, F4, F5, F6, F7, F10, F11, F12, A2), 1 left open (B4), 1 not a gap per the wiki (copy area), 1 without content (cargo pods).
- Sources: wiki Allowed area, Zone/Area, Home area, Orders, Door, Menus, Assign. The Forbidden and Forbid pages returned 404; the auto-forbid and order-allows-target rows rest on documented game behaviour and say so.
- New sub-package `internal/application/game/allow` (stateless): `allow.Forbids` over `allow.Host` (roles Stacks, Buildings, Ground; 3/3/2 methods). It owns setting a stack's or building's flag, the Forbid/Allow rectangle, the haul mark and Haul things rectangle, `Ordered`, and the auto-forbid rules `DropForbidden` (outside the home area) and `CorpseForbidden` (except a kill its hunter or a predator takes). `game/forbid.go` is now the adapter (`allowHost`) plus thin wrappers; no Game field, no new file in package game.
- Behaviour changes in package game (exact edits at the call sites):
  - `burial.go` `spawnCorpse`: a pawn's body starts forbidden outside Home, any faction. This replaces `forbidRaidBody` (raiders anywhere, nobody else).
  - `raids.go` `dropRaiderGear`: the weapon and a thief's haul follow the same rule; `craft_host.go` `dropStackOf` now carries `Forbidden`.
  - `animals.go` `killAnimal`, `animal_life.go` (old age): carcasses outside Home start forbidden unless `killTaken` (a predator, or a hunter on its hunt for that animal). `wildlife.go` `dropCorpseAs` gained a `forbid` argument and stacks only with like-forbidden carcasses.
  - `setup_mapgen.go`, `vault.go`, `casket.go`: map loot starts forbidden (and found, as before). Stone chunks stay allowed.
  - `haul_claim.go` `setHaulMark`/`markHaul`: Haul things and Mark for hauling allow a forbidden stack; found stacks also take the mark.
  - `orders.go` `prioritize`: an Eat, Haul or Equip order allows its target.
  - `food.go` `bestFood`: a colonist in a mental break ignores forbidding (binge).
- Allowed areas: `internal/domain/world/area` `MaxAreas` 10 → 8 (wiki: Allowed area; Home not counted). The palette keeps ten colours. A save with more areas keeps them and cannot add one. `architect/area_manager.go` (AG's directory, two-line exact edit): `AreasMax = area.MaxAreas`, cap reason text.
- UI, read-only (exact edits):
  - `inspector/building_stats.go`: the building Forbid toggle carries the F key and reads Allow (icon `order_allow`) when forbidden, for beds too. New `Building.ForbidSelection`.
  - `hotkeys.go` `forbidSelection`: with no stack selected, F flips the selected door or bed.
  - `inspector/cmds_build.go`: the building pane says "Forbidden: colonists leave it alone".
  - `render/world/objects.go` `DrawBuildingObj`: forbidden buildings get the item's forbidden marker.
  - `inspector/cmds_thing.go`: Mark for hauling is no longer disabled on a forbidden found stack; its description says marking allows it.
- No new command kind, no game.go view field, no save field. No Architect card change (the Haul things card text "Mark things found on the map for hauling..." still holds; the lead may add "allows forbidden items" through AG).

## Tests

- `game/allow/allow_test.go`: Set (stack, carried stack, door re-walk once, bed), Rect (fog, outside), HaulRect (colony stack allowed without mark, loot allowed and marked, fog left, only changed stacks put back, cancel unmarks only), Mark, Ordered, the drop and corpse rules, Rect over a copy of the ids.
- `game/forbid_test.go`: `TestDeathDropsForbiddenOutsideHome` (replaces `TestRaidDropsForbidden`: raider and colonist bodies and weapons by the home-area rule both ways), `TestCarcassForbiddenOutsideHome` (inside/outside, a hunter's kill, `killTaken`), `TestHaulThingsAllows` (loot allowed and marked, colony stack allowed unmarked, SetHaulMark allows), `TestOrderAllowsTarget` (Eat, Equip, Haul), `TestBingeIgnoresForbidden`.
- `adapters/inbound/ebiten/journey_allow_test.go`:
  - `TestJourneyAllowDoorKeyAndMarker`: click a door, F forbids it (SetForbidden), the pane says forbidden, the toggle reads Allow, a frame draws, F again allows it.
  - `TestJourneyAllowHaulThingsAllows`: F forbids a stack, the Architect's Haul things drag allows it.
  - `TestJourneyAllowOrderedEquipAllows`: a forbidden knife, right-click Equip, the knife is allowed.
  - `TestJourneyAllowDeathOutsideHome`: a deer killed outside Home leaves a forbidden found carcass, one inside does not; clicking Mark for hauling allows and marks it.
- Updated, not weakened (the rule changed): `cmds_thing_test.go` `TestItemHaulMarkCommand` (the forbidden mark is enabled now); `haul_claim_test.go` (map and vault loot and the casket weapon are found and forbidden, chunks stay allowed; the raider/visitor/colonist gear test paints Home round the visitor and the colonist); `burial_test.go` `burGame` and `herd_food3_test.go` `TestCowButchersByAge` paint Home over their map (a body or carcass outside Home is forbidden now); `area_test.go` (cap 8); `herd_food3_test.go`, `animal_claims_test.go` (new `dropCorpseAs` argument).
- Golden: `TestGrownGolden` moves with this change alone (HEAD plus STK-AL in a scratch worktree): `0x51487b30909d4220` → `0x582e6835f12d21a2`, stable over `-count=2`. Reason: carcasses and bodies outside Home now start forbidden. The lead re-pins the combined value.

## Budget

Package game: 53652 → 53635 lines (-17, measured in the scratch worktree; `make arch` passes there); Game methods -2 (`forbidRaidBody`, the unused `forbidden`); fields and files unchanged. `inspector.Building` 38 → 39 methods.

## Left open

- B4: visitors, traders and colonists in a mental break should pass a forbidden door. The path grid is shared, so this needs a per-faction door layer in pathing and regions (a defence/pathing task).
- Downed pawns dropping their weapon is a combat rule we do not have; when it lands, its drop should use `allow.Forbids.DropForbidden`.
- Selecting several doors at once: the selection model groups pawns, animals and items only.

## UI representation

Every player-visible change has one: the forbidden marker on items and now buildings, the pane line, the toggle label and key, and the area manager's cap.
