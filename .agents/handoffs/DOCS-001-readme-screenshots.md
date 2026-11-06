# DOCS-001 README with screenshots and clips

## What changed
- `README.md` (new): pitch, hero clip, "See it move" clip grid, progress gallery
  of 11 stills grouped Building / Threats / Colony life, feature summary, run
  instructions, pointers to `AGENTS.md` and `.cursor/`.
- `docs/screenshots/*.png`: 1280×720 stills from the real renderer (`game -capture`).
- `docs/clips/*.webp`: 800×450, 30 fps, looping animated WebP (3–5 s, ≤1 MB each),
  from `game -capture-frames` dumps.
- `scripts/frames_to_webp.py`: Pillow encoder (glob → slice/step → Lanczos resize
  → animated WebP; fails over `--max-bytes`).
- `Makefile`: `screenshots` and `clips` targets (one capture line per item;
  clips use a `clip` define, frames go to gitignored `bin/clips/`).

## How to refresh
`make screenshots clips` (about 5 min; opens a game window per capture; needs
python3 + Pillow). Then bump the "Media updated" date in `README.md`.

## Notes
- Stills reproduce pixel-for-pixel except the soil hover panel, which follows the
  real mouse cursor. Clips run at 1× in real time, so frames (and sizes) vary slightly.
- `-capture-ui hits` never landed a damaging blow in any scenario tried, so the
  combat still uses `combat` and shows no damage number.
- Medic still: in `-scenario medic` Bo never lies in the bed; the shot shows the
  infection panel and healroot stages instead. Assigning the bed in the scenario
  would give a better shot.
- `defense` still: the battered wall is not readable; the caption covers only
  the sandbags and traps.
- No player-facing game behaviour changed; no checklist items affected.
- Stale `assets/generated/ingame_*.png` and `_to_delete/` are untouched.
