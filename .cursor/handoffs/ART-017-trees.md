# Task Handoff

Task ID: ART-017 (PLAY-010)
Status: complete

## Summary
Trees (oak, birch, pine, jungle; autumn, bare and snow looks) have a visible trunk with a slim root flare, a loose canopy of clusters with gaps that overhangs into the tile above, muted §9.1 colours and no outline. Each tree also renders `_trunk` and `_canopy` parts on the same canvas and origin. The renderer draws trunk then canopy; the canopy fades to 40% while a creature stands on the tree's tile, the tile above or the two diagonals above, or the hovered tile, selection or Architect ghost is there (eases ~0.2 s; instant with Reduced Motion). Trunks and shadows never fade.

## Files
`families/trees.py` (its own build family: `make art FAMILIES=trees`), `ebiten/tree_fade.go`, `ebiten/tree_focus.go` (hover/selection/ghost focus), `drawPlantObj` parts.

## Notes
Heights are 1.5–1.7 tiles (bare variants taller). The menu diorama still needs `make art-menu` to pick up the new trees.
