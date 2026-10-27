# DOCS-002: README "See it move" media re-captured

## Why
The README's clips and gallery stills dated from before the RimWorld-style
art pass (ART-023..027) and the medical overhaul (MED-00..09), so they showed
the old look. The user asked to regain the section.

## What changed
- Re-captured all 13 `docs/media/*.webp` stills and all 18 existing `docs/clips/*.webp` clips (including the hero `lategame`) with the current build.
- Three new clips (Makefile `clips` target):
  - `medical`: `-scenario bleeding`. A colonist bleeding from cuts is carried to a hospital bed and tended.
  - `surgery`: `-scenario surgery`. A peg-leg install in a hospital bed beside a powered vitals monitor.
  - `outbreak`: `-scenario outbreak`. Flu: the letter, the disease alert, the sick on bed rest.
- New dev scenarios in `cmd/game` (`bleeding`, `surgery`, `outbreak`, listed in the flag help). There is also a new dev hook, `Game.InjureCut` (a bleeding cut, unlike `Injure`'s bruise), with `TestInjureCutBleeds`.
- Lower quality so clips fit the 1.4 MB cap: chores `--quality 55`, surgery 240 frames at `--quality 45`.
- README:
  - the three clips in the "See it move" table;
  - captions updated for the new look (chores: no tools, a progress bar; melee: weapons across the body, bleeding; shadows);
  - "Media updated 2026-10-26";
  - the Health feature bullet describes the medical system.

## Verify
- Every WebP is under its size cap.
- A few frames of each were extracted and reviewed. The pawns, weapons and medical scenes read as intended.

## Note
`make media clips` needs a `python3` with Pillow. On this machine that is `/usr/local/bin/python3`, so run `PATH=/usr/local/bin:$PATH make media clips`.

## Follow-up (PLAY-024)
- Two new clips: `turret` (`-scenario turret -capture-ui turret`) and `trade` (`-scenario trade -capture-ui trade`). The trade capture mode orders the first colonist to "Trade with" the caravan trader, so the dialog opens about 55 frames in.
