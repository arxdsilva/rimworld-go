# DOCS-003: README that shows the game first

User request (2026-11-05): the README read as a wall of text; make it easy to scan, with the game and the download visible at once.

## What changed

- `README.md` is 84 lines, longest line 167 characters. Order: title and one-line promise, download link (`releases/latest`), hero clip, "One colony, one bad season", six feature cards (2-column table, clip plus two lines), Start playing (three steps, one-line requirements, build from source), First five minutes, Go deeper, Your data (three plain lines, the update check named), Credits and contributing. Links to the new docs and Releases are full URLs because `scripts/package_release.sh` ships the README alone; images stay relative as before.
- New `docs/FEATURES.md` (all 48 "What works today" bullets under 13 headings, long ones split into sub-bullets), `docs/GALLERY.md` (all 23 clips and 13 stills with captions, "Media updated" line), `docs/PLAYING.md` (install, requirements, sound and settings, full controls table), `docs/DEVELOPING.md` (build and test, developer flags as a table, media and sound refreshing, the Blender MCP pointer, the release note), `docs/PRIVACY.md` (the old section word for word).
- `Makefile`: three comments and one help text that said "README" now say "gallery". No behaviour change.
- YouTube is gone (it was only on the first line). No "open source" or "free" wording: there is no licence file yet.

## Stale facts, checked in code

- Research projects: "seven" is wrong. `internal/domain/research/research.go` has 75 `Project` values before `NumProjects` (and 75 `Project:` rows in `defs`). `RESEARCH_GAP_ANALYSIS.md` still says 45; that doc is not mine to edit.
- Traits: "19" is wrong. `internal/domain/pawn/trait/trait.go` has 41 traits before `NumTraits` (slot 0 is None). FEATURES says 41.
- Animal species: 61 stands. `TestRosterHas65` counts 65 including 4 mechanoids (`ArchMech`, hostile machines, not tameable), so 61 animals. README and FEATURES keep "61".
- Biomes: twelve stands (`NumBiomes` is 12 in `internal/domain/world/biome/biome.go`).
- Also confirmed: 59 backstories (`backstory/catalog.go`), 19 difficulty settings (`difficulty.Settings`), eleven skills.
- Go version: the old README said 1.26.5; `go.mod` says 1.26.10. Corrected in README and DEVELOPING.
- The 117 fps figure keeps its source and conditions (500×500 grown colony, ten minutes, 3× speed, Apple M3 Pro, 1% low 68, 0.7 ms per tick). Not re-measured.

## Moved as is, to be corrected later

"Generated with ElevenLabs" (animal voices) is in FEATURES unchanged; a later commit corrects it.

## Front-page clips

Frames from the clips were looked at; nothing was re-recorded. `lategame`, `building`, `turret`, `zoo` and `bigmap` look current. The "A world that moves" card uses `shadows` instead of `daycycle`, whose frames are mostly an empty field and then black.

Worth re-recording: `daycycle`, `shadows`, `rain` and `snow` all show a raid letter and death or bleeding messages in the corner ("Cora bled to death", "Bo froze to death"), because a raid fires during those timelapses. `medical` starts with the patient already in bed, so the carry its caption describes is at most in the first tenth; the rest follows other colonists.

## Checklists and behaviour

No checklist item changes and no game behaviour changes: docs and Makefile comments only. No HUD representation needed.
