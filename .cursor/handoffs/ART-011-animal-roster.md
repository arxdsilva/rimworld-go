# ART-011 Animal roster: 61 species from one parametric rig

## What changed
- **Roster:** `families/animal_roster.py` mirrors the Go roster (keys, order, attack). Each row's `features` text is the visual brief.
  - Ten RimWorld-invented creatures are original designs (ART §11.1 originals rule): no blue wool cylinder, no yellow sacs, no single horn.
- **Rig** (`families/animals/`):
  - `rig.py`:
    - `Body` spec: length, torso, chest/waist/hips, legs, neck, head, ears, horns, antlers, tail, palette roles, marks, `muscle`, `outline`, plus the `legs=`/`model=`/`poses=` hooks.
    - `Skel` with 2-bone IK legs.
    - Poses: trot, each attack kind, graze (the neck is solved so the nose reaches the ground), flinch, three corpse poses.
    - `render_species` sizes one canvas per species.
  - `parts.py`: horns, branching antlers, ears, tails, manes, wool skirts, humps, trunk, tusks, emissive glands, claws.
  - `quadruped.py`: 48 species, including the monkey. `bird.py`, `reptile.py` and `insect.py` cover the other 13, with their own materials in `materials_animals_extra.yaml`.
- **Sprites per species and facing:** standing, `walk01..08`, the attack set (`bite`/`claw`/`kick`/`gore`/`headbutt`/`peck` 1..4), `graze1..2`, `flinch1..2`.
  - Corpses `item/corpse_<key>_v0..v2`, with dried-blood pools.
  - Icons `ui_icon/<key>`.
  - Female sets for deer, elk and chicken (hen).
- **Style details:** tapered, muscled legs; outlines of 2 px (XS) and 3 px (S).
- **Pipeline helpers:** `stage.render_asset(outline_width=)`, and the palette merges `materials_*.yaml`.
- **Removed:** the old dustbuck and ridgewolf art and icons.
- **Validator:** requires every roster species' full set.
- **Renderer:**
  - attack frames by `AnimalView.Attack`; the `_f` art for females
  - size-aware hunt and tame markers and health bars
  - 61 original one-line descriptions (`animal_blurbs.go`)
  - icon fallback (an initial on a disc)
  - corpse sprites by species; burst effects for explosive deaths

## Verification
- 7556 sprites, 0 validator errors. The animal family renders in about 8.5 min (A1 plus A2), private TMPDIR per worker.
- **Reviewed by eye:** colour sheets for all species (standing in 4 facings, trot frames, attack, graze, flinch, corpses), a 64 px silhouette sheet, a 1× lineup, and the in-game `-scenario zoo`.
  - The polish round fixed the elk and glimmerstag mass, the cinder rat's glow, the Yorkshire terrier's coat and the hare's ears.
- **Tests:** `TestEverySpeciesHasArt`, frame keys by attack, female fallback, blurbs, corpse sprites, overlay height, icon fallback.

## Known limitations
- The animal lean is 40° (`contracts.FIGURE_LEAN_DEG` says 55): kept as before.
- South and north silhouettes of slim animals are narrow.
- The XS birds (chicken, duck) are small blobs at far zoom.
- The emberbuck's throat gland is faint.
- The cobra's hood is edge-on from east and west.
- Repo size grew by about 60 MB of sprites and normal maps.
