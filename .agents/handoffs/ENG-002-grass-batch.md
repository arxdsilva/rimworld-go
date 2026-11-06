# Task Handoff

Task ID: ENG-002 (PLAY-010)
Status: complete

## Summary
Grass is drawn as composed tufts instead of one centred sprite per tile: up to nine copies per tile (1/2/4/6/9 by growth) on a jittered 3×3 grid, edge copies spilling up to 0.15 tile into the neighbours, each with a hashed variant, mirror and scale (short 0.45–0.65, tall 0.7–0.9 on rich soil or smooth noise patches; dormant tufts in winter). All tufts in view go out in one lit triangle batch from a runtime atlas, after all ground and before objects, so a neighbour's ground never paints over them. Grass shows on every terrain the simulation seeds.

## LOD
48 px+ live with sway; 24–48 px at most 4 copies; 16–24 px tufts baked into the chunk images (bake key folds in the chunk's grass signature); under 16 px a soft grass tint in the map bake.

## Numbers
- `BenchmarkGrassBatch`: 0.19 ms for 2,310 quads, 1 draw call.
- `BenchmarkDrawArt500` 0.6–2.5 ms paused (4.4 ms running at z0.25); `DrawArt250` 1.0–3.3 ms paused, 5.2–5.9 ms running; 283–1,339 draw calls (unchanged).
- `-fpsprobe 60` on the 500×500 grown colony at 3 ticks/frame: z0.25 118.4 fps avg, 1% low 76.9; z1 119.9 avg, 1% low 77.2.

## Notes
The grown fixture has little grass in view, so the DrawArt numbers barely exercise tufts; `BenchmarkGrassBatch` covers a full screen. A per-chunk `GrassVer` in the simulation's chunk read model would spare the map tier its renderer-side hashing (optional follow-up).
