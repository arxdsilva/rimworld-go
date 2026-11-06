# OPT-1: the map-side Options rows (OPT-010, 014, 016, 026)

The four rows read `render.View.Prefs`, which the root fills in `syncRenderer`. Only `render/{creature,env}` and one call line in `render/world` changed.

## What changed
- **OPT-010 animal names.** `creature.AnimalLabel(mode, *AnimalView)`: `none` shows no plate, `named` (default) only named tame animals, `tame` every tame animal, by species label when unnamed. `DrawAnimalOverlays` uses it.
- **OPT-026 plant sway.** `env.swayTime(v)` replaces `envTime` in `Sky.PlantPose` and the crop batch. With `Prefs.PlantSway` off the wind phase is held at one instant, so plants keep a static lean. Wind for leaves and weather, and contact springs, still move. No plant gains a contour.
- **OPT-016 far pawns.** `creature.DrawFarPawns(set, v, cull, screen, w)` (free function: `Set` is at its method budget), called by `world/draw_art.go` after the animal overlays. Map tier only, modes `hostile` and `all`, styles `dots`, `silhouettes`, `both`. Hostile creatures are red, others pale yellow. Hostile animals count as hostile. One pass over the frame's pawn and animal lists with the view culled, `solidRect` draws of the shared white pixel, no allocation. Mode `none` returns on the first line.
- **OPT-014 hats only on the map: not doable in my packages.** Hats are drawn on the map (`creature/worn.go`, via the root's `FigureLayers`) and in the portraits by the same root function, `portrait.go:~165-195` (`hat` is appended last). The row needs `portrait.go` to skip the `hat` layer when `Prefs.HatsOnlyOnMap` is on (the colonist bar and the dossier/inspector portraits go through it, the map does not). That is a root file; left to the lead.

## Tests
`creature/far_test.go`: modes and styles, culling, `AnimalLabel` modes, name plates through `DrawAnimalOverlays` per mode. `env/sway_test.go`: pose frozen with sway off, moving with it on.

## Benchmark
`DrawArt500` A/B against a worktree of HEAD (`160cb607f`), default settings: see the report. Draw calls identical, allocs unchanged.

## Hash impact
None (presentation only).
