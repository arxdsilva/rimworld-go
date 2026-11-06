# ART-028: main-menu hero replaced with original art (clean-room fix)

## Why
The old main-menu colonist (a red-bearded man with a raven and a rifle) was
generated from `ai-bg-rimworld`. That brief was an ASCII-art
"reconstruction" of a 700×412 crop of an existing illustration, which breaks
the clean-room rule. The user chose to regenerate original art.

## What changed
- New hero: a scout woman with goggles, a shearling field coat and a rifle, with a sled dog.
  - Generated with Canva from an original prompt (`assets/source/raster/ui/menu/briefs.md`).
  - The only reference was our own storyteller bust (`portraits/gambler.png`), used for style.
  - The user picked it from three takes.
- `assets/source/raster/ui/menu/hero.png`: a 1288×1600 magenta source.
- `portrait_cut.py` `cut_menu` (run by `make portraits`): keys, inks and outlines it, then fits it bottom-flush into the 904×1104 `main_menu_colonist.png`. The splash contract and release packaging are unchanged.
- `ai-bg-rimworld` is deleted. `assets/AGENTS.md` and ART §17.1 are updated.

## Note
The ART-023 storyteller busts used the old menu image as a *style* reference: comic cel-shading only, no character, composition or props carried over. If you want a fully clean chain, regenerate them with the new hero as the style reference. This is a cheap follow-up with the same pipeline.

## Verify
- `make portraits` re-cuts the three busts byte-identical and writes the new menu image.
- `game -capture … -capture-ui menu` shows the new hero beside the menu.
- The splash tests pass.
