# Main-menu hero brief

Provenance for `hero.png` (ART-028). It was generated on 2026-10-26 with
Canva's image generator from the prompt below. The only reference image was
our own original storyteller bust `../portraits/gambler.png`, used for
rendering style only. No existing illustration was used as a reference or
input.

It replaces the earlier menu colonist, whose brief was an ASCII
reconstruction of an existing illustration. That brief has been deleted.

`make portraits` (`assets/source/tools/portrait_cut.py`, `cut_menu`) keys
the magenta field and writes `../main_menu_colonist.png` (904×1104), which
`splash.go` loads.

> ORIGINAL heroic title-screen character for a sci-fi frontier colony game.
> Match the rendering style of the reference image (heroic comic
> illustration, bold black cel outlines, flat two-tone cel shading, slight
> painterly texture, warm rim light) but NOT its character. Waist-up bust,
> three-quarter view, confident steady gaze toward the viewer. Subject: a
> rugged colonist woman in her 40s, dark hair in a thick braid over one
> shoulder, a pale scar across one cheekbone, brass-rimmed goggles pushed up
> on her forehead, a patched olive-green field coat with a shearling collar
> and leather shoulder straps, a long hunting rifle held diagonally across
> her chest. A large grey-and-white sled dog with pale blue eyes sits close
> at her side, its head at her waist, looking out alert. Muted earthy
> palette with olive, rust and brass accents. Background: solid flat pure
> magenta (#FF00FF), no gradient, no shadow on the background. No text, no
> logo, no frame. Original design.

## Extension (ART-029, 2026-10-27)

The first take was framed with the scout's right arm and the rifle's muzzle
running off the right edge, which the menu shows as a straight cut (the
splash sits bottom-left, so only its right and top edges are visible).
`hero.png` was extended with Canva's image generator using that take as
the only reference (our own art), at 3:4, exported at 1200×1600. The user
picked it from two takes. `cut_menu` now refuses a source whose figure
touches the right or top edge, and inks no contour along a side where the
painting is cut.

> The same picture as the reference, but the camera has stepped back about
> twenty percent: a wider shot of the same scout and her sled dog in the
> same pose, so her right arm and sleeve and the whole hunting rifle across
> her chest sit well inside the frame with magenta margin around them.
> Identical character, face, expression, goggles, braid, coat, colours and
> cel-shaded comic ink style; the same flat bright magenta background with
> nothing else added; no text.
