# ART-029: the main-menu hero's right side was cut; extended, and the cut tool hardened

## Why
The ART-028 hero (`assets/source/raster/ui/menu/hero.png`, 1288×1600) was generated with the scout's right arm and the rifle's muzzle already past the frame's right edge, so the menu showed a straight vertical cut through the sleeve. Nothing downstream could restore it: `cut_menu` keys and fits the whole source, and `drawMenuSplash` draws it unclipped, bottom-left, so only the right and top edges of the figure are ever visible. The cut tool made it worse by inking its 4 px contour along the cut, a dark band at x 900–903.

## What changed
- **Art:** `hero.png` is now the extended take (1200×1600, 3:4): Canva's image generator with the ART-028 hero as the only reference (our own art), "a wider shot of the same scout … with magenta margin around them"; the user picked it from two takes (a third was refused by the credit quota). The prompt is in `menu/briefs.md`. `main_menu_colonist.png` (904×1104) is re-cut from it: content spans x 36–867, the last columns are transparent, the bottom row is flush.
- **`portrait_cut.py`:**
  - `clipped_sides(img)`: the sides a keyed source's content runs off.
  - `contour(img, width, clipped)`: no contour past the shape's edge on a clipped side, so a painting cut on a side keeps a clean edge instead of an inked band. This also re-cut two storyteller busts (`gambler`, `steady`), which were cut on a side: they lost the 4 px dark line there (reviewed: an improvement).
  - `cut_menu` refuses a source whose figure touches the right or top edge (`SystemExit` with the reason), so `make portraits` catches a bad source before the test does.
- **Test:** `TestMenuSplashKeepsItsRightEdgeFree` (splash_test.go): the splash PNG's last two columns are fully transparent.
- Docs: `assets/AGENTS.md` hero paragraph, `menu/briefs.md`, ART §17.1's hero row.

## Verify
- `PATH=/usr/local/bin:$PATH make portraits` reproduces the outputs; against the old source it exits 1 with the right-edge message.
- `go test -race -run 'Splash|Menu' ./internal/adapters/inbound/ebiten/`.
- `bin/game -capture <scratch>/menu.png -capture-ui menu` (and `-capture-ui-scale 2`): arm and rifle whole, no band, the figure clears the action rail at 1280×720.

## Left for the user
- Delete the Canva scratch design "Blank magenta portrait scratch canvas" (`DAHZHun9Dn8`, now 7 pages); there is no delete tool here.
- The Canva image-generation quota is used up; the storyteller-bust restyle noted in ART-028 waits for it.
