# Task Handoff

Task ID: PLAY-008
Status: complete (phase 4)

## Summary

Rooms, impressiveness, mood and lamp light. Rooms are flood-filled
regions bounded by rock, walls and doors; regions touching the map edge
are outdoors. Each room gets contents (cells, floored cells, beds,
tables, chairs, lamps, workbenches, stone-wall share), a role
(general / bedroom / dining / workshop / barracks), an impressiveness
score and a label (awful / dull / decent / nice / impressive). Pawns
keep timed memory thoughts (sleeping in a bed or on the ground, outside,
in an awful or nice room; eating a meal or raw food, with or without a
table) and situational thoughts (hungry, tired); mood = 50 + thoughts,
clamped 0..100. Pawns below mood 30 lose one work tick in four. A lamp
lights its whole room: the night veil is lifted there with a warm tint.

## Public Contracts Changed

- new domain packages `world/room` (`Detect`, `Contents`, `Role`, `Impressiveness`, `Label`) and `pawn/mood` (`Kind`, `Def`, `Memories`, `Active`, `Level`, `LowMood`, `Label`).
- read model: `PawnView.Mood/MoodLabel/Thoughts`, `WorldView.RoomGrid/Rooms`, `RoomView`, `ThoughtView`, `Game.RoomInfo(x, y)`.
- ebiten: `drawDaylightRooms` replaces the flat veil on the art path; `skyOverlay` is now premultiplied (it used to add light) and its max alpha rose 48 -> 96. Capture adds `-capture-ui pawn`; the capture layout cuts a wider area so the room gets built, and the lamp sits clear of the 1x2 bed.

## Verification

go test -race ./... green; gofmt/vet clean. New tests: room detection/outdoors/gaps, scoring and roles (3x3 furnished hut = "nice"), mood memories/groups/expiry, rooms scored in game, mood from eating and sleeping, unhappy pawns working slower. In-game: assets/generated/ingame_night_room.png (day 2, 22:00, lit barracks, inspector with mood and thoughts).

## Known Limitations

Rooms recompute fully when dirty (no region-local invalidation). Mood
has no smoothing, trend arrow or breaks. Light is per room (no falloff,
no outdoor lamp radius, no effect on plants). Wall art overlapping the
row above can show a lit band along a room's south wall.
