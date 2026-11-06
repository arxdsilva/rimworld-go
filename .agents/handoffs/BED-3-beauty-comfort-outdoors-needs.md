# BED-3: comfort, beauty and outdoors needs

Date: 2026-10-29. Wave D. Backlog `.cursor/BEDS_GAP_ANALYSIS.md` (section 3.6). Contract commit `8f3932359` (arch(contract): wave D kinds, commands, jobs and read-model fields) added the `SitDown` job and `PawnView` Comfort/Beauty/Outdoors (percent) before this work started.

## What changed
- **Needs** (B3; `game/rest/needs.go`, `beauty.go`, `seats.go`, `room_mood.go`, `waking.go`, `beds.go`): `NeedRecord` holds comfort, beauty and outdoors. Updates every 150 ticks per pawn, staggered by id. Need thoughts are memories refreshed while their stage holds.
- **Comfort:** rises sitting on a chair, stool or armchair, or asleep in a bed (end table and dresser count); falls otherwise.
- **Outdoors:** six places (in a room or not; open, roofed, thick rock); losses x0.2 in bed.
- **Beauty:** average tile beauty over 241 cells of the pawn's room (or open ground) every 250 ticks. A tile is floor, filth, building (once) and plant; sculpture beauty is picked up. No per-cell cache (local sampling).
- **Sitting:** `SitDown` starts below 30 % comfort outside Work hours; ends at min(70 %, seat comfort) or 1500 ticks; a failed seat search waits 2500 ticks. It is idle (`job.IsIdle`), so work replaces it. Idle Relax and meals prefer seats weighted by comfort.
- **Slept cold and slept hot** (`waking.go`): -4 for a day against the pawn's comfort band.
- **HasBed** (`beds.go`): true if the pawn holds any bed slot (prison and medical included); otherwise each free claimable slot goes to one bedless colonist in pawn order; one pass per view build.
- Saves: RestNeeds round trips. Mood: five new kinds. Chair row: dining chair 0.7 comfort, 8 beauty.

## Wiki numbers versus original tuning
- Wiki: comfort gain and ladder, furniture comfort and beauty, outdoors rates and ladder, beauty ladder, slept cold/hot -4 for a day, outdoors-in-bed x0.2.
- Original: outdoor filth at 30 % of wiki, beauty target and drift, bare ground 0, plant +1, SitDown thresholds and timings, the 2 % climb margin, update and sampling intervals.

## UI
- U4: inspector Needs tab shows Comfort, Beauty and Outdoors bars; `hud/alerts.go` key `nobed` ("Ada needs a bed" / "N colonists need beds") with an explanation.
- Beauty overlay (UI 16.6) is not done.

## Hysteresis
Comfort stage thoughts need a 2 % margin to climb; sitting starts at 30 % and ends at min(70 %, seat comfort) or 1500 ticks; a failed seat search waits 2500 ticks. No-flap tests in `rest/seats_test.go` and `needs_test.go`.

## Tests and numbers
`rest/{needs,seats,beds,waking}_test.go`, `game/rest_needs_test.go`, `hud/alerts_test.go`; journeys `TestJourneyColonistsNeedBedsAlert` and `TestJourneySittingRaisesComfort` (the second uses a stool; `BuildNow` cannot place fabric). Rest coverage 100 %. Game package 9 lines smaller (`tableSeat` moved into rest).
Integration (lead): `job.SitDown` is idle (`Kind.IsIdle`), so any real work replaces a sit. The beds alert legitimately covers right-hand map tiles: `journey_zones_test.go` gives its colonist a sleeping spot so the area-tool drags stay on the map.

## Gaps and follow-ups
- No couch kind. No Indoorsy or Undergrounder trait modifiers (trait catalogue).
- No per-cell beauty cache; beauty overlay and "selected pawn perceived beauty" (UI 16.6) open.
- No threshold markers on the new bars; no noisy-place thought.
- Colony `Wealth()` still a separate sum (reuse `room.BuildingValue`).
- Report vs code: the report said "RestNeeds with round trips" and "5 mood kinds"; only the slept cold/hot kinds were confirmed by grep (`mood.SleptCold`, `SleptHot`).
