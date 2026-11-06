# ART-023: storyteller portraits redrawn as painted busts

## Why
The user found the three storyteller busts (Orla Vane, Bram Holloway,
Kestrel Quay) far below the main-menu colonist. They were Blender busts
built from primitives (`families/portrait.py`). This change replaces them
with painted busts in the menu colonist's style.

## What changed
- New sources `assets/source/raster/ui/portraits/{steady,hearth,gambler}.png`
  (1288×1600, flat magenta field).
  - Made with Canva's image generator from the prompts in `briefs.md`.
  - The only style reference was our own `main_menu_colonist.png`; no
    RimWorld art was used.
  - The character briefs are unchanged from ART §17.1.
- New tool `assets/source/tools/portrait_cut.py` with target `make portraits`.
  - Keys out the magenta and removes magenta spill within 6 px of the edge.
  - Inks the fringe, then fits each bust bottom-flush into 512×640 with a 4 px contour.
  - Cuts a 120×150 thumbnail framed on the head, with a 2 px contour.
  - Writes `metadata/ui_portrait/*.yaml` with the same keys as before, minus the unused `projection`.
  - Deterministic.
- Removed:
  - `families/portrait.py` and its `FAMILIES` entry.
  - 18 bust-only colours in `palettes/materials_ng1.yaml`; the shared colours were kept.
  - The dead `ui/portrait/` fallback in `screen_storyteller.go` `portraitImage`.
- Docs: ART §17.1, NG1 plan §7.2 and `assets/AGENTS.md`.

## Verify
- `python3 assets/source/tools/portrait_cut.py`, then `validate_assets.py` (0 errors).
- `go test -race -run 'Storyteller|Splash|Story' ./internal/adapters/inbound/ebiten/`.
- `game -capture … -capture-ui storyteller`: the cards and the large bust read clearly.

## Notes
- `python3` on this machine resolves to a Homebrew build without Pillow. `/usr/local/bin/python3` has Pillow and numpy.
- Follow-up for the user: the old menu brief at repo-root `ai-bg-rimworld` describes itself as a "reconstruction" of a 700×412 source crop. The menu art's clean-room status deserves a check. It is out of scope here.
- There is no player-observable behaviour change beyond the art itself.
