# BED-2: room statistics, roles and room thoughts

Date: 2026-10-29. Wave C. Backlog `.cursor/BEDS_GAP_ANALYSIS.md` (sections 3.4, 3.5).

## What changed
- **Domain** (B2, opus; `domain/world/room/tally.go`, `stats.go`, `room.go`): a `Tally` built at room rebuild holds cells, building beauty (furniture x stuff factor + offset x quality), floor beauty and cleanliness (bare -1/-1, wood 0, stone tile and smooth +1, carpet +2), building and floor value. The old ad-hoc score is removed; impressiveness stages live in `stats.go`.
- **Game** (`rooms.go`, `rooms_index.go`, `rest_host.go`, one-line hook in `idle.go`): filth (-15 beauty, -5 cleanliness per full unit), item value and medical/prisoner bed flags are read live. Reads happen only at waking, a meal, a finished pastime and the world view; zero allocations.
- **Roles** added: hospital, prison cell, prison barracks, kitchen, recreation room.
- **Thoughts** (`rest/room_mood.go`, `waking.go`, `deps.go`; six new mood kinds): bedroom and prison cell -2, 0, +1..+8; barracks and prison barracks -7..+4; hospital -4, -2, none, none, +1..+6; dining and recreation +2..+8 from decent up. Read once per occasion, memory lasts a day. Shared bed -5 + opinion/20 clamped -10..0. The old nice/awful pair is gone; trait Fussy is rekeyed.
- **Read model:** `RoomView` Space, Beauty, Wealth, CleanStat, Impressiveness, SpaceLabel, BeautyLabel, WealthLabel; `Score` is whole impressiveness points.

## Wiki numbers versus original tuning
Wiki: impressiveness formula, space and stage bands, floor beauty and cleanliness, filth values, all thought values above. The shared-bed value is flagged by the wiki as possibly other-version. Nothing here is original tuning except the choice of reading points.

## UI
`hud/rooms.go` `RoomCardLines`: role, impressiveness, space, beauty, wealth, cleanliness with labels; `RoomTint` bands 20/30/40/50/65/85/120/170/240. `hud/room_facts*.go` deleted. Journey `TestJourneyRoomCardStats`.

## Tests and numbers
`room/tally_test.go`, `stats_test.go`, `rest` tests, `hud/room_card_test.go`. Coverage: rest 100 %, room 99 %. Game package 11 lines and one `Game` method smaller.

## Gaps and follow-ups
- Greedy and Jealous (and the Ascetic ladder) need the trait catalogue; no separate shared-bedroom thought beyond barracks.
- Colony `Wealth()` could reuse `room.BuildingValue`.
- No shelves, so stored items do not yet count toward room wealth beyond loose items (PRD-4).
- Comfort, beauty and outdoors needs remain BED-3.
